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

func TestDownloadRetryDecisionRequiresIdleTimeout(t *testing.T) {
	if shouldRetryDownloadAttempt(0, false, nil) {
		t.Fatal("successful attempt requested retry")
	}
	if shouldRetryDownloadAttempt(0, false, context.Canceled) {
		t.Fatal("successful attempt with cleaned-up context requested retry")
	}
	if !shouldRetryDownloadAttempt(0, true, nil) {
		t.Fatal("idle-cancelled first attempt did not request retry")
	}
	if shouldRetryDownloadAttempt(1, true, nil) {
		t.Fatal("second idle-cancelled attempt requested another retry")
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

func TestValidateSettingsDefaultsDownloadPresetFields(t *testing.T) {
	settings := validateSettings(Settings{})
	if settings.DownloadPreset != defaultDownloadPreset {
		t.Fatalf("DownloadPreset = %q, want %q", settings.DownloadPreset, defaultDownloadPreset)
	}
	if settings.CustomFormatID != defaultCustomFormatID {
		t.Fatalf("CustomFormatID = %q, want %q", settings.CustomFormatID, defaultCustomFormatID)
	}
	if settings.CustomContainer != defaultCustomContainer {
		t.Fatalf("CustomContainer = %q, want %q", settings.CustomContainer, defaultCustomContainer)
	}
	if settings.CustomSubtitle != defaultCustomSubtitle {
		t.Fatalf("CustomSubtitle = %q, want %q", settings.CustomSubtitle, defaultCustomSubtitle)
	}
}

func TestResolveSettingsPathUsesPortableWhenWritable(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	want := filepath.Join(appDir, "settings.json")
	if got := resolveSettingsPathFor(filepath.Join(root, "config"), want, "settings.json"); got != want {
		t.Fatalf("resolveSettingsPathFor() = %q, want %q", got, want)
	}
}

func TestResolveSettingsPathFallsBackWhenPortableUnavailable(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	want := filepath.Join(configDir, "settings.json")
	if got := resolveSettingsPathFor(configDir, "", "settings.json"); got != want {
		t.Fatalf("resolveSettingsPathFor() = %q, want %q", got, want)
	}
}

func TestStartupResolvesPortableBinDir(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.startup(context.Background())
	if app.binDir == "" {
		t.Fatal("binDir not initialized")
	}
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		if isWritableDir(exeDir) {
			want := filepath.Join(exeDir, "bin")
			if app.binDir != want {
				t.Fatalf("binDir = %q, want %q (portable path)", app.binDir, want)
			}
		}
	}
}

func TestDownloadPostProcessingArgsByPreset(t *testing.T) {
	if got := downloadPostProcessingArgs("compatible", "mkv", "embed"); len(got) != 2 || got[0] != "--recode-video" || got[1] != "mp4" {
		t.Fatalf("compatible preset args = %#v, want recode-video mp4", got)
	}
	if got := downloadPostProcessingArgs("audio", "mkv", "embed"); len(got) != 3 || got[0] != "-x" || got[1] != "--audio-format" || got[2] != "mp3" {
		t.Fatalf("audio preset args = %#v, want extract-audio mp3", got)
	}
	if got := downloadPostProcessingArgs("custom", "mkv", "embed"); len(got) < 2 || got[0] != "--merge-output-format" || got[1] != "mkv" {
		t.Fatalf("custom preset args = %#v, want merge-output-format mkv", got)
	}
}


func TestHistoryHelpersPreserveFileOrder(t *testing.T) {
	entries := []HistoryEntry{
		{DownloadID: "old", Title: "first"},
		{DownloadID: "new", Title: "second"},
	}
	path := filepath.Join(t.TempDir(), "history.json")

	if err := writeHistoryEntriesToFile(path, entries); err != nil {
		t.Fatalf("writeHistoryEntriesToFile: %v", err)
	}

	got := readHistoryEntriesFromFile(path)
	if len(got) != len(entries) {
		t.Fatalf("history length = %d, want %d", len(got), len(entries))
	}
	for i := range entries {
		if got[i].DownloadID != entries[i].DownloadID || got[i].Title != entries[i].Title {
			t.Fatalf("entry %d = %#v, want %#v", i, got[i], entries[i])
		}
	}

	reversed := reverseHistoryEntries(got)
	if len(reversed) != len(entries) {
		t.Fatalf("reversed length = %d, want %d", len(reversed), len(entries))
	}
	if reversed[0].DownloadID != "new" || reversed[1].DownloadID != "old" {
		t.Fatalf("reverseHistoryEntries returned %#v", reversed)
	}
}

func TestParseProgressLine(t *testing.T) {
	tests := []struct {
		line  string
		ok    bool
		pct   float64
		size  string
		speed string
		eta   string
	}{
		{
			line:  "[download]  42.5% of ~12.34MiB at 1.23MiB/s ETA 00:10",
			ok:    true,
			pct:   42.5,
			size:  "12.34MiB",
			speed: "1.23MiB/s",
			eta:   "00:10",
		},
		{
			line:  "[download]   0.2% of  614.43KiB at  987.13KiB/s ETA 00:00",
			ok:    true,
			pct:   0.2,
			size:  "614.43KiB",
			speed: "987.13KiB/s",
			eta:   "00:00",
		},
		{
			line:  "[download]   0.5% of  614.43KiB at    1.63MiB/s ETA 00:00",
			ok:    true,
			pct:   0.5,
			size:  "614.43KiB",
			speed: "1.63MiB/s",
			eta:   "00:00",
		},
		{
			line:  "[download] 100.0% of  614.43KiB at    8.50MiB/s ETA 00:00",
			ok:    true,
			pct:   100.0,
			size:  "614.43KiB",
			speed: "8.50MiB/s",
			eta:   "00:00",
		},
	}

	for _, tt := range tests {
		pct, size, speed, eta, ok := parseProgressLine(tt.line)
		if ok != tt.ok {
			t.Errorf("parseProgressLine(%q) ok = %v, want %v", tt.line, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if pct != tt.pct {
			t.Errorf("parseProgressLine(%q) pct = %v, want %v", tt.line, pct, tt.pct)
		}
		if size != tt.size {
			t.Errorf("parseProgressLine(%q) size = %q, want %q", tt.line, size, tt.size)
		}
		if speed != tt.speed {
			t.Errorf("parseProgressLine(%q) speed = %q, want %q", tt.line, speed, tt.speed)
		}
		if eta != tt.eta {
			t.Errorf("parseProgressLine(%q) eta = %q, want %q", tt.line, eta, tt.eta)
		}
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
