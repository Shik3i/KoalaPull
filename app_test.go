package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	ytdlp := filepath.Join(binDir, "yt-dlp")
	script := "#!/bin/sh\nsleep 0.25\nprintf '%s\\n' '{\"id\":\"id\",\"title\":\"title\",\"formats\":[]}'\n"
	if err := os.WriteFile(ytdlp, []byte(script), 0755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	app := NewApp()
	app.ctx = ctx
	app.binDir = binDir

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
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	ytdlp := filepath.Join(binDir, "yt-dlp")
	script := "#!/bin/sh\nsleep 1\nprintf '%s\\n' '{\"id\":\"id\",\"title\":\"title\",\"formats\":[]}'\n"
	if err := os.WriteFile(ytdlp, []byte(script), 0755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.binDir = binDir

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
	settings := validateSettings(Settings{Theme: "neon", MaxConcurrency: 1 << 30})
	if settings.Theme != "dark" {
		t.Fatalf("Theme = %q, want dark", settings.Theme)
	}
	if settings.MaxConcurrency != 10 {
		t.Fatalf("MaxConcurrency = %d, want 10", settings.MaxConcurrency)
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
