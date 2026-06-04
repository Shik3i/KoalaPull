package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func init() {
	isTesting = true
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		runHelperProcess()
	}
}

func runHelperProcess() {
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
	case "spawn-child":
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), "KOALAPULL_HELPER_MODE=delayed-file")
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.WriteFile(os.Getenv("KOALAPULL_HELPER_READY"), []byte("ready"), 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "delayed-file":
		time.Sleep(500 * time.Millisecond)
		if err := os.WriteFile(os.Getenv("KOALAPULL_HELPER_SENTINEL"), []byte("child survived"), 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	case "large-output":
		fmt.Print(strings.Repeat("x", 64*1024))
		os.Exit(0)
	default:
		fmt.Fprintln(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}
}


func TestDependencyArtifactsRequireIntegrityVerification(t *testing.T) {
	for _, goos := range []string{"windows", "darwin", "linux"} {
		artifact := ffmpegArtifactFor(goos)
		if artifact.URL == "" || artifact.MaxBytes <= 0 {
			t.Fatalf("%s artifact missing URL or size limit: %#v", goos, artifact)
		}
		if artifact.ChecksumURL == "" && artifact.SignatureURL == "" {
			t.Fatalf("%s artifact has no integrity verification", goos)
		}
	}
}

func TestDownloadFileRejectsOversizedContentLengthAndRemovesPartialFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "download")
	app := NewApp()
	err := app.downloadFile(context.Background(), server.URL, dest, "test", 10)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("downloadFile error = %v, want size-limit error", err)
	}
	if fileExists(dest) {
		t.Fatal("oversized partial download was not removed")
	}
}

func TestDownloadFileRejectsOversizedChunkedResponseAndRemovesPartialFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		_, _ = io.WriteString(w, strings.Repeat("x", 32))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "download")
	app := NewApp()
	err := app.downloadFile(context.Background(), server.URL, dest, "test", 10)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("downloadFile error = %v, want size-limit error", err)
	}
	if fileExists(dest) {
		t.Fatal("oversized partial download was not removed")
	}
}

func TestVerifyFileChecksumRejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset")
	if err := os.WriteFile(path, []byte("trusted"), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("trusted"))
	if err := verifyFileChecksum(path, fmt.Sprintf("%x", sum), "asset"); err != nil {
		t.Fatalf("verifyFileChecksum valid digest: %v", err)
	}
	if err := verifyFileChecksum(path, strings.Repeat("0", sha256.Size*2), "asset"); err == nil {
		t.Fatal("verifyFileChecksum accepted mismatched digest")
	}
}

func TestExtractFFmpegZipBoundedExtractsOnlyFFmpeg(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "ffmpeg.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(archive)
	other, err := zw.Create("../../not-ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = other.Write([]byte("ignore"))
	ffmpeg, err := zw.Create("bundle/bin/ffmpeg.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = ffmpeg.Write([]byte("binary"))
	ffprobe, err := zw.Create("bundle/bin/ffprobe.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = ffprobe.Write([]byte("probe"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(root, "extracted")
	if err := extractFFmpegFromZipBounded(context.Background(), archivePath, dest); err != nil {
		t.Fatalf("extractFFmpegFromZipBounded: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "binary" {
		t.Fatalf("extracted data = %q, want binary", data)
	}
	probeDest := filepath.Join(root, "ffprobe-extracted.exe")
	if err := extractZipBinaryBounded(context.Background(), archivePath, probeDest, "ffprobe.exe"); err != nil {
		t.Fatalf("extractZipBinaryBounded ffprobe: %v", err)
	}
	probeData, err := os.ReadFile(probeDest)
	if err != nil {
		t.Fatal(err)
	}
	if string(probeData) != "probe" {
		t.Fatalf("extracted ffprobe data = %q, want probe", probeData)
	}
	if fileExists(filepath.Join(root, "not-ffmpeg")) {
		t.Fatal("archive traversal member was extracted")
	}
}

func TestExtractFFmpegZipBoundedHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "ffmpeg.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(archive)
	ffmpeg, err := zw.Create("bundle/bin/ffmpeg.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = ffmpeg.Write([]byte("binary"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dest := filepath.Join(root, "extracted")
	if err := extractFFmpegFromZipBounded(ctx, archivePath, dest); !errors.Is(err, context.Canceled) {
		t.Fatalf("extractFFmpegFromZipBounded error = %v, want context.Canceled", err)
	}
	if fileExists(dest) {
		t.Fatal("cancelled extraction created destination")
	}
}

func TestReplaceFilePreservingOldReplacesContents(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "new")
	dest := filepath.Join(root, "installed")
	if err := os.WriteFile(src, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFilePreservingOld(src, dest); err != nil {
		t.Fatalf("replaceFilePreservingOld: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("installed data = %q, want new", data)
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

func TestFetchMetadataConcurrentRequestsDoNotInterfere(t *testing.T) {
	binDir := installFakeYtDlp(t)
	app := NewApp()
	app.ctx = context.Background()
	app.binDir = binDir
	t.Setenv("KOALAPULL_HELPER_MODE", "sleep-json")
	t.Setenv("KOALAPULL_HELPER_DELAY_MS", "200")

	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)

	go func() {
		defer wg.Done()
		_, err := app.FetchMetadata("https://example.invalid/one")
		if err != nil {
			errs <- fmt.Errorf("first request failed: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		_, err := app.FetchMetadata("https://example.invalid/two")
		if err != nil {
			errs <- fmt.Errorf("second request failed: %w", err)
		}
	}()

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}


func TestCommandOutputCancellationKillsProcessTree(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	ready := filepath.Join(root, "ready")
	sentinel := filepath.Join(root, "sentinel")
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("KOALAPULL_HELPER_MODE", "spawn-child")
	t.Setenv("KOALAPULL_HELPER_READY", ready)
	t.Setenv("KOALAPULL_HELPER_SENTINEL", sentinel)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := commandOutput(ctx, exe, "-test.run=TestHelperProcess")
		done <- err
	}()
	deadline := time.Now().Add(3 * time.Second)
	for !fileExists(ready) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !fileExists(ready) {
		cancel()
		t.Fatal("helper parent did not start child")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("commandOutput error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("commandOutput did not return after cancellation")
	}
	time.Sleep(700 * time.Millisecond)
	if fileExists(sentinel) {
		t.Fatal("child process survived parent cancellation")
	}
}

func TestCommandOutputLimitedRejectsOversizedOutput(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("KOALAPULL_HELPER_MODE", "large-output")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := commandOutputLimited(ctx, 1024, exe, "-test.run=TestHelperProcess")
	if !errors.Is(err, errCommandOutputTooLarge) {
		t.Fatalf("commandOutputLimited returned %d bytes and error %v, want output limit error", len(out), err)
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

func TestNormalizeDownloadAttemptErrorSuppressesOnlyFirstIdleTimeout(t *testing.T) {
	base := errors.New("killed")
	suppress, got := normalizeDownloadAttemptError(0, true, base)
	if !suppress || !errors.Is(got, base) {
		t.Fatalf("first idle timeout = (%v, %v), want suppressed base error", suppress, got)
	}
	suppress, got = normalizeDownloadAttemptError(1, true, base)
	if suppress || !strings.Contains(got.Error(), "stalled") || !errors.Is(got, base) {
		t.Fatalf("second idle timeout = (%v, %v), want visible stalled error", suppress, got)
	}
	suppress, got = normalizeDownloadAttemptError(0, false, base)
	if suppress || got != base {
		t.Fatalf("ordinary error = (%v, %v), want unchanged", suppress, got)
	}
}

func TestVersionComparisonOnlyReportsNewerVersions(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{latest: "v1.2.4", current: "v1.2.3", want: true},
		{latest: "1.2", current: "1.2.0", want: false},
		{latest: "2026.05.01", current: "2026.04.30", want: true},
		{latest: "2026.04.30", current: "2026.05.01", want: false},
		{latest: "v1.2.3", current: "v1.2.3", want: false},
		{latest: "malformed", current: "v1.2.3", want: false},
		{latest: "v1.2.4", current: "", want: false},
	}
	for _, tt := range tests {
		if got := isVersionNewer(tt.latest, tt.current); got != tt.want {
			t.Errorf("isVersionNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestBrowserOperationsRejectUnknownBrowser(t *testing.T) {
	app := NewApp()
	if _, err := app.IsBrowserRunning("unknown"); err == nil {
		t.Fatal("IsBrowserRunning accepted unknown browser")
	}
	if err := app.KillBrowser("unknown"); err == nil {
		t.Fatal("KillBrowser accepted unknown browser")
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
	if err := NewApp().OpenExternalLink("file:///etc/passwd"); err == nil {
		t.Fatal("OpenExternalLink accepted file URL")
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

	got, err := readHistoryEntriesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
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

func TestUpdateSettingsFailurePreservesCacheAndSemaphore(t *testing.T) {
	app := NewApp()
	old := validateSettings(Settings{Theme: "dark", MaxConcurrency: 2})
	app.cachedSettings = old
	app.semLimit.Store(int32(old.MaxConcurrency))
	app.settingsFilePath = filepath.Join(t.TempDir(), "missing", "settings.json")

	err := app.UpdateSettings(Settings{Theme: "light", MaxConcurrency: 8})
	if err == nil {
		t.Fatal("UpdateSettings succeeded with unavailable settings directory")
	}
	got := app.GetSettings()
	if got.Theme != old.Theme || got.MaxConcurrency != old.MaxConcurrency {
		t.Fatalf("cached settings changed after failed write: %#v", got)
	}
	if got := app.semLimit.Load(); got != int32(old.MaxConcurrency) {
		t.Fatalf("semaphore limit = %d, want %d", got, old.MaxConcurrency)
	}
}

func TestConcurrentSettingsUpdatesKeepSemaphoreInSync(t *testing.T) {
	app := NewApp()
	app.settingsFilePath = filepath.Join(t.TempDir(), "settings.json")
	app.cachedSettings = validateSettings(Settings{Theme: "dark", MaxConcurrency: 3})
	app.semLimit.Store(3)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(limit int) {
			defer wg.Done()
			if err := app.UpdateSettings(Settings{Theme: "dark", MaxConcurrency: limit}); err != nil {
				t.Errorf("UpdateSettings: %v", err)
			}
		}(i%maxMaxConcurrency + 1)
	}
	wg.Wait()

	settings := app.GetSettings()
	if got := int(app.semLimit.Load()); got != settings.MaxConcurrency {
		t.Fatalf("semaphore limit = %d, cached settings = %d", got, settings.MaxConcurrency)
	}
}

func TestHistoryMutationsPreserveCacheWhenPersistenceFails(t *testing.T) {
	entry := HistoryEntry{DownloadID: "keep", Title: "keep"}
	app := NewApp()
	app.historyFilePath = filepath.Join(t.TempDir(), "missing", "history.json")
	app.historyCache = []HistoryEntry{entry}
	app.historyLoaded = true

	if err := app.ClearHistory(); err == nil {
		t.Fatal("ClearHistory succeeded with unavailable history directory")
	}
	if len(app.historyCache) != 1 || app.historyCache[0].DownloadID != entry.DownloadID {
		t.Fatalf("ClearHistory changed cache after failed write: %#v", app.historyCache)
	}

	if err := app.DeleteHistoryEntry(entry.DownloadID); err == nil {
		t.Fatal("DeleteHistoryEntry succeeded with unavailable history directory")
	}
	if len(app.historyCache) != 1 || app.historyCache[0].DownloadID != entry.DownloadID {
		t.Fatalf("DeleteHistoryEntry changed cache after failed write: %#v", app.historyCache)
	}
}

func TestHistoryFileRetainsNewestEntries(t *testing.T) {
	entries := make([]HistoryEntry, maxHistoryEntries+5)
	for i := range entries {
		entries[i].DownloadID = fmt.Sprintf("%d", i)
	}
	path := filepath.Join(t.TempDir(), "history.json")
	if err := writeHistoryEntriesToFile(path, entries); err != nil {
		t.Fatalf("writeHistoryEntriesToFile: %v", err)
	}

	got, err := readHistoryEntriesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxHistoryEntries {
		t.Fatalf("history length = %d, want %d", len(got), maxHistoryEntries)
	}
	if got[0].DownloadID != "5" || got[len(got)-1].DownloadID != fmt.Sprintf("%d", len(entries)-1) {
		t.Fatalf("retained history range = %q..%q", got[0].DownloadID, got[len(got)-1].DownloadID)
	}
}

func TestReadFileBoundedRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large")
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileBounded(path, 4); err == nil {
		t.Fatal("readFileBounded accepted oversized file")
	}
}

func TestReadHistoryRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxHistoryFileBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readHistoryEntriesFromFile(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("readHistoryEntriesFromFile error = %v, want size-limit error", err)
	}
}

func TestSaveHistoryEntryCompactsBeforeReplacingCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	entries := make([]HistoryEntry, maxHistoryEntries)
	for i := range entries {
		entries[i].DownloadID = fmt.Sprintf("%d", i)
	}
	if err := writeHistoryEntriesToFile(path, entries); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.historyFilePath = path
	app.historyCache = append([]HistoryEntry(nil), entries...)
	app.historyLoaded = true

	app.saveHistoryEntry(HistoryEntry{DownloadID: "new"})

	got, err := readHistoryEntriesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxHistoryEntries || got[0].DownloadID != "1" || got[len(got)-1].DownloadID != "new" {
		t.Fatalf("compacted history range = %d entries, %q..%q", len(got), got[0].DownloadID, got[len(got)-1].DownloadID)
	}
	if len(app.historyCache) != maxHistoryEntries || app.historyCache[len(app.historyCache)-1].DownloadID != "new" {
		t.Fatalf("history cache not compacted: %d entries", len(app.historyCache))
	}
}

func TestCorruptHistoryIsReportedAndPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	original := "{\"downloadId\":\"old\"}\n{\"downloadId\":"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.historyFilePath = path
	if _, err := app.GetHistory(); err == nil {
		t.Fatal("expected corrupt history error")
	}

	app.saveHistoryEntry(HistoryEntry{DownloadID: "new"})
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("corrupt history was overwritten: %q", got)
	}
}

func TestSaveHistoryEntryPreservesCacheWhenPersistenceFails(t *testing.T) {
	app := NewApp()
	app.historyFilePath = filepath.Join(t.TempDir(), "missing", "history.json")
	app.historyCache = []HistoryEntry{{DownloadID: "old"}}
	app.historyLoaded = true

	app.saveHistoryEntry(HistoryEntry{DownloadID: "new"})

	if len(app.historyCache) != 1 || app.historyCache[0].DownloadID != "old" {
		t.Fatalf("history cache changed after failed write: %#v", app.historyCache)
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

func TestParseRateLimitToBytes(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"1", 1024 * 1024},
		{"1.5", int64(1.5 * 1024 * 1024)},
		{"1,5", int64(1.5 * 1024 * 1024)},
		{"0.5", int64(0.5 * 1024 * 1024)},
		{"0,25", int64(0.25 * 1024 * 1024)},
		{"0", 0},
		{"-1.5", 0},
		{"invalid", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseRateLimitToBytes(tt.input); got != tt.want {
			t.Errorf("parseRateLimitToBytes(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseCustomArgs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"--proxy socks5://127.0.0.1:1080 --geo-bypass", []string{"--proxy", "socks5://127.0.0.1:1080", "--geo-bypass"}},
		{"--proxy \"socks5://127.0.0.1:1080\" --geo-bypass", []string{"--proxy", "socks5://127.0.0.1:1080", "--geo-bypass"}},
		{"--proxy 'socks5://127.0.0.1:1080'", []string{"--proxy", "socks5://127.0.0.1:1080"}},
		{"  --arg1   --arg2  ", []string{"--arg1", "--arg2"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := parseCustomArgs(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseCustomArgs(%q) len = %d, want %d (got: %#v)", tt.input, len(got), len(tt.want), got)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseCustomArgs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestValidateSettingsNewFields(t *testing.T) {
	s := validateSettings(Settings{
		RateLimitEnabled: true,
		RateLimitValue:   "2,5",
		CustomArgs:       "--proxy localhost:8080",
	})
	if s.RateLimitValue != "2,5" {
		t.Errorf("RateLimitValue = %q, want 2,5", s.RateLimitValue)
	}
	if s.CustomArgs != "--proxy localhost:8080" {
		t.Errorf("CustomArgs = %q, want --proxy localhost:8080", s.CustomArgs)
	}

	s2 := validateSettings(Settings{
		RateLimitValue: "invalid",
	})
	if s2.RateLimitValue != "1" {
		t.Errorf("invalid RateLimitValue should fallback to 1, got %q", s2.RateLimitValue)
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

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestDependencySkipChecks(t *testing.T) {
	oldTransport := http.DefaultClient.Transport
	defer func() {
		http.DefaultClient.Transport = oldTransport
	}()

	var ytdlpDlCount, ffmpegDlCount int
	var mu sync.Mutex

	http.DefaultClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			urlStr := req.URL.String()
			if strings.Contains(urlStr, "yt-dlp") {
				ytdlpDlCount++
			} else if strings.Contains(urlStr, "ffmpeg") {
				ffmpegDlCount++
			}
			return nil, errors.New("mock network error")
		},
	}

	t.Run("ytdlp_installed_ffmpeg_missing", func(t *testing.T) {
		mu.Lock()
		ytdlpDlCount = 0
		ffmpegDlCount = 0
		mu.Unlock()

		app := NewApp()
		app.ctx = context.Background()
		app.binDir = t.TempDir()

		if err := os.WriteFile(app.ytdlpPath(), []byte("dummy-ytdlp"), 0755); err != nil {
			t.Fatal(err)
		}

		err := app.DownloadDependencies()
		if err == nil {
			t.Fatal("expected download error for ffmpeg")
		}

		mu.Lock()
		defer mu.Unlock()
		if ytdlpDlCount != 0 {
			t.Errorf("yt-dlp download was not skipped: count = %d", ytdlpDlCount)
		}
		if ffmpegDlCount == 0 {
			t.Error("ffmpeg download was not attempted")
		}
	})

	t.Run("ffmpeg_installed_ytdlp_missing", func(t *testing.T) {
		mu.Lock()
		ytdlpDlCount = 0
		ffmpegDlCount = 0
		mu.Unlock()

		app := NewApp()
		app.ctx = context.Background()
		app.binDir = t.TempDir()

		if err := os.WriteFile(app.ffmpegPath(), []byte("dummy-ffmpeg"), 0755); err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS == "windows" {
			if err := os.WriteFile(app.ffprobePath(), []byte("dummy-ffprobe"), 0755); err != nil {
				t.Fatal(err)
			}
		}

		err := app.DownloadDependencies()
		if err == nil {
			t.Fatal("expected download error for yt-dlp")
		}

		mu.Lock()
		defer mu.Unlock()
		if ytdlpDlCount == 0 {
			t.Error("yt-dlp download was not attempted")
		}
	})

	if runtime.GOOS == "windows" {
		t.Run("ffmpeg_exists_ffprobe_missing", func(t *testing.T) {
			mu.Lock()
			ytdlpDlCount = 0
			ffmpegDlCount = 0
			mu.Unlock()

			app := NewApp()
			app.ctx = context.Background()
			app.binDir = t.TempDir()

			if err := os.WriteFile(app.ytdlpPath(), []byte("dummy-ytdlp"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(app.ffmpegPath(), []byte("dummy-ffmpeg"), 0755); err != nil {
				t.Fatal(err)
			}

			err := app.DownloadDependencies()
			if err == nil {
				t.Fatal("expected download error for ffmpeg")
			}

			mu.Lock()
			defer mu.Unlock()
			if ytdlpDlCount != 0 {
				t.Errorf("yt-dlp download was not skipped: count = %d", ytdlpDlCount)
			}
			if ffmpegDlCount == 0 {
				t.Error("ffmpeg download was not attempted despite missing ffprobe")
			}
		})
	}
}

func TestSaveHistoryEntryReturnsErrorOnWriteFailure(t *testing.T) {
	app := NewApp()
	app.historyFilePath = filepath.Join(t.TempDir(), "missing-dir", "history.json")
	app.historyLoaded = true

	err := app.saveHistoryEntry(HistoryEntry{DownloadID: "new"})
	if err == nil {
		t.Fatal("expected saveHistoryEntry to return error on write failure, got nil")
	}
}

func TestStartDownloadQueueLimit(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.configDir = t.TempDir()

	for i := 0; i < maxQueueLimit; i++ {
		app.activeDownloads[fmt.Sprintf("dl-%d", i)] = func() {}
	}

	_, err := app.StartDownloadWithPreset("https://example.com/video", "best", t.TempDir(), "mp4", "none", "title", "best")
	if err == nil || !strings.Contains(err.Error(), "queue limit reached") {
		t.Fatalf("expected queue limit error, got: %v", err)
	}
}

func TestUpdateDependenciesFailsWhenActive(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.configDir = t.TempDir()

	app.activeDownloads["dl-active"] = func() {}

	err := app.UpdateDependencies()
	if err == nil || !strings.Contains(err.Error(), "downloads are in progress") {
		t.Fatalf("expected active downloads error, got: %v", err)
	}
}

func TestVersionCheckSleepContextAware(t *testing.T) {
	app := NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	cancel()

	start := time.Now()
	_ = app.GetYtdlpVersion()
	elapsed := time.Since(start)

	if elapsed >= 1*time.Second {
		t.Fatalf("GetYtdlpVersion did not abort immediately on context cancellation, elapsed: %v", elapsed)
	}
}
