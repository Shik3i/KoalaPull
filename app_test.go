package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestFFmpegZipDestNamePreservesWindowsExecutableSuffix(t *testing.T) {
	if got := ffmpegZipDestName("ffmpeg.exe", "windows"); got != "ffmpeg.exe" {
		t.Fatalf("ffmpeg.exe destination = %q, want ffmpeg.exe", got)
	}
	if got := ffmpegZipDestName("ffprobe.exe", "windows"); got != "ffprobe.exe" {
		t.Fatalf("ffprobe.exe destination = %q, want ffprobe.exe", got)
	}
	if got := ffmpegZipDestName("ffmpeg.exe", "darwin"); got != "ffmpeg" {
		t.Fatalf("darwin ffmpeg.exe destination = %q, want ffmpeg", got)
	}
}

func TestFetchMetadataRespectsAppContextCancellation(t *testing.T) {
	binDir := installFakeYtDlp(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	app := NewApp()
	app.ctx = ctx
	app.binDir = binDir
	t.Setenv("KOALAPULL_HELPER_MODE", "sleep-json")
	t.Setenv("KOALAPULL_HELPER_DELAY_MS", "250")

	start := time.Now()
	_, err := app.FetchMetadata("https://example.invalid/video")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("FetchMetadata succeeded, want cancellation error")
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("FetchMetadata ignored context cancellation; elapsed = %s", elapsed)
	}
}

func TestFetchMetadataCancelsPreviousRequest(t *testing.T) {
	binDir := installFakeYtDlp(t)
	app := NewApp()
	app.ctx = context.Background()
	app.binDir = binDir
	t.Setenv("KOALAPULL_HELPER_MODE", "sleep-json")
	t.Setenv("KOALAPULL_HELPER_DELAY_MS", "1000")

	firstDone := make(chan error, 1)
	go func() {
		_, err := app.FetchMetadata("https://example.invalid/one")
		firstDone <- err
	}()
	time.Sleep(50 * time.Millisecond)

	_, _ = app.FetchMetadata("https://example.invalid/two")

	select {
	case err := <-firstDone:
		if err == nil {
			t.Fatal("first FetchMetadata succeeded, want cancellation")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("first FetchMetadata was not cancelled by second request")
	}
}

func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	switch os.Getenv("KOALAPULL_HELPER_MODE") {
	case "sleep-json":
		delay, err := time.ParseDuration(os.Getenv("KOALAPULL_HELPER_DELAY_MS") + "ms")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		time.Sleep(delay)
		_ = json.NewEncoder(os.Stdout).Encode(rawMetadata{
			ID:      "id",
			Title:   "title",
			Formats: []rawFormat{},
		})
		os.Exit(0)
	default:
		fmt.Fprintln(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}
}

func TestIncreasingConcurrencyWakesWaitingDownload(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.configDir = t.TempDir()
	app.semLimit.Store(1)
	app.semCount.Store(1)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	acquired := make(chan bool, 1)
	go func() {
		acquired <- app.semAcquire(ctx)
	}()

	select {
	case got := <-acquired:
		t.Fatalf("semAcquire returned early: %v", got)
	case <-time.After(20 * time.Millisecond):
	}

	if err := app.UpdateSettings(Settings{
		DefaultOutputDir: t.TempDir(),
		Theme:            "dark",
		MaxConcurrency:   2,
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	select {
	case got := <-acquired:
		if !got {
			t.Fatal("semAcquire returned false after concurrency increase")
		}
	case <-ctx.Done():
		t.Fatal("semAcquire was not woken after concurrency increase")
	}
}

func TestAllowedDownloadURLRejectsNonHTTPProtocols(t *testing.T) {
	if !isAllowedDownloadURL("https://example.com/video") {
		t.Fatal("https URL was rejected")
	}
	if !isAllowedDownloadURL("http://example.com/video") {
		t.Fatal("http URL was rejected")
	}
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/file", "/tmp/video.mp4", "not a url"} {
		if isAllowedDownloadURL(raw) {
			t.Fatalf("%q was allowed, want rejected", raw)
		}
	}
}

func TestFetchMetadataRejectsInvalidURLsBeforeExec(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.binDir = t.TempDir()

	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/file", strings.Repeat("a", 2050)} {
		if _, err := app.FetchMetadata(raw); err == nil {
			t.Fatalf("FetchMetadata(%q) succeeded, want validation error", raw[:min(len(raw), 64)])
		}
	}
}

func TestValidateSettingsClampsUnsafeValues(t *testing.T) {
	settings := validateSettings(Settings{Theme: "neon", MaxConcurrency: 1 << 30, Language: "pirate"})
	if settings.Theme != "dark" {
		t.Fatalf("Theme = %q, want dark", settings.Theme)
	}
	if settings.MaxConcurrency != 10 {
		t.Fatalf("MaxConcurrency = %d, want 10", settings.MaxConcurrency)
	}
	if settings.Language != "en" {
		t.Fatalf("Language = %q, want en", settings.Language)
	}
}

func TestValidateSettingsPreservesUTF8PathBoundaries(t *testing.T) {
	raw := strings.Repeat("你", maxPathLength/3+10)
	settings := validateSettings(Settings{DefaultOutputDir: raw, Theme: "dark", MaxConcurrency: 3, Language: "de"})
	if !utf8.ValidString(settings.DefaultOutputDir) {
		t.Fatal("DefaultOutputDir is not valid UTF-8 after validation")
	}
	if settings.Language != "de" {
		t.Fatalf("Language = %q, want de", settings.Language)
	}
}

func TestValidateSettingsDefaultsLanguageToEnglish(t *testing.T) {
	settings := validateSettings(Settings{})
	if settings.Language != "en" {
		t.Fatalf("Language = %q, want en", settings.Language)
	}
}

func TestCollectRecentLinesReportsLongLineScannerError(t *testing.T) {
	line := strings.Repeat("x", 70*1024)
	lines, err := collectRecentLines(strings.NewReader(line), 20)
	if err != nil {
		t.Fatalf("collectRecentLines returned error for long line: %v", err)
	}
	if len(lines) != 1 || lines[0] != line {
		t.Fatalf("collectRecentLines did not preserve long line; len=%d", len(lines))
	}
}

func TestParseProgressLine(t *testing.T) {
	pct, size, speed, eta, ok := parseProgressLine("[download]  42.5% of ~12.34MiB at 1.23MiB/s ETA 00:10")
	if !ok {
		t.Fatal("parseProgressLine rejected a valid yt-dlp line")
	}
	if pct != 42.5 {
		t.Fatalf("percent = %v, want 42.5", pct)
	}
	if size != "12.34MiB" {
		t.Fatalf("file size = %q, want 12.34MiB", size)
	}
	if speed != "1.23MiB/s" {
		t.Fatalf("speed = %q, want 1.23MiB/s", speed)
	}
	if eta != "00:10" {
		t.Fatalf("eta = %q, want 00:10", eta)
	}
}

func TestParsePlaylistStatus(t *testing.T) {
	if got := parsePlaylistStatus("[download] Downloading video 3 of 14"); got != "video 3 of 14" {
		t.Fatalf("playlist status = %q, want video 3 of 14", got)
	}
}

func installFakeYtDlp(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}

	srcPath, err := os.Executable()
	if err != nil {
		t.Fatalf("current executable: %v", err)
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dstPath := filepath.Join(binDir, name)

	src, err := os.Open(srcPath)
	if err != nil {
		t.Fatalf("open test binary: %v", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		t.Fatalf("create fake yt-dlp: %v", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		t.Fatalf("copy fake yt-dlp: %v", err)
	}
	if err := dst.Close(); err != nil {
		t.Fatalf("close fake yt-dlp: %v", err)
	}

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	return binDir
}
