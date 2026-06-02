package main

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var AppVersion = "dev"

type App struct {
	ctx               context.Context
	configDir         string
	binDir            string
	dlMu              sync.Mutex
	dlCounter         int
	activeDownloads   map[string]context.CancelFunc
	adMu              sync.Mutex
	lastFileSize      map[string]string
	lastSpeed         map[string]string
	downloadSemaphore chan struct{}
	semMu             sync.Mutex
	historyMu         sync.Mutex
}

type DependencyStatus struct {
	YtDlpInstalled  bool `json:"ytDlpInstalled"`
	FfmpegInstalled bool `json:"ffmpegInstalled"`
}

type DepProgress struct {
	Dependency string `json:"dependency"`
	Progress   int    `json:"progress"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type FormatInfo struct {
	FormatID   string `json:"formatId"`
	Ext        string `json:"ext"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	VCodec     string `json:"vcodec"`
	ACodec     string `json:"acodec"`
	Filesize   int64  `json:"filesize"`
	FormatNote string `json:"formatNote"`
}

type VideoMetadata struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	Thumbnail  string       `json:"thumbnail"`
	Uploader   string       `json:"uploader"`
	Duration   float64      `json:"duration"`
	Formats    []FormatInfo `json:"formats"`
	IsPlaylist bool         `json:"isPlaylist"`
	EntryCount int          `json:"entryCount"`
}

type DownloadProgress struct {
	DownloadID     string  `json:"downloadId"`
	Percent        float64 `json:"percent"`
	Speed          string  `json:"speed"`
	ETA            string  `json:"eta"`
	FileSize       string  `json:"fileSize"`
	Status         string  `json:"status"`
	Error          string  `json:"error,omitempty"`
	PlaylistStatus string  `json:"playlistStatus,omitempty"`
}

type Settings struct {
	DefaultOutputDir string `json:"defaultOutputDir"`
	Theme            string `json:"theme"`
	MaxConcurrency   int    `json:"maxConcurrency"`
	AutoPasteURL     bool   `json:"autoPasteURL"`
}

type HistoryEntry struct {
	DownloadID string    `json:"downloadId"`
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	FormatID   string    `json:"formatId"`
	FileSize   string    `json:"fileSize"`
	AvgSpeed   string    `json:"avgSpeed"`
	Status     string    `json:"status"`
	ErrorMsg   string    `json:"errorMsg,omitempty"`
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
}

type VersionInfo struct {
	Ytdlp  string `json:"ytdlp"`
	Ffmpeg string `json:"ffmpeg"`
	App    string `json:"app"`
}

type UpdateInfo struct {
	YtdlpUpdateAvailable  bool   `json:"ytdlpUpdateAvailable"`
	LatestYtdlpVersion    string `json:"latestYtdlpVersion"`
}

var progressRegex = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?([\d.]+\S+)\s+at\s+([\d.]+\S+)\s+ETA\s+(\S+)`)

var sizeLineRegex = regexp.MustCompile(`\[download\]\s+100%\s+of\s+~?([\d.]+\S+)`)

var playlistItemRegex = regexp.MustCompile(`\[download\]\s+Downloading\s+(video|item)\s+(\d+)\s+of\s+(\d+)`)

func NewApp() *App {
	a := &App{
		lastFileSize: make(map[string]string),
		lastSpeed:    make(map[string]string),
	}
	return a
}

func (a *App) initSemaphore() {
	a.downloadSemaphore = make(chan struct{}, a.GetSettings().MaxConcurrency)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			configDir = filepath.Join(home, ".config")
		} else {
			configDir = os.TempDir()
		}
	}
	a.configDir = filepath.Join(configDir, "KoalaPull")
	a.binDir = filepath.Join(a.configDir, "bin")
	if err := os.MkdirAll(a.binDir, 0755); err != nil {
		println("Failed to create bin directory:", err.Error())
	}
	a.activeDownloads = make(map[string]context.CancelFunc)
	a.initSemaphore()
	a.cleanupStaleTempFiles()
}

func (a *App) cleanupStaleTempFiles() {
	entries, err := os.ReadDir(a.binDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			if err := os.Remove(filepath.Join(a.binDir, e.Name())); err != nil {
				println("cleanup stale tmp:", err.Error())
			}
		}
	}
}

func (a *App) ytdlpPath() string {
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(a.binDir, name)
}

func (a *App) ffmpegPath() string {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(a.binDir, name)
}

func (a *App) ffmpegDir() string {
	return filepath.Dir(a.ffmpegPath())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (a *App) nextDownloadID() string {
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	a.dlCounter++
	return fmt.Sprintf("dl_%d_%d", time.Now().UnixMilli(), a.dlCounter)
}

// ---------- Settings ----------

func (a *App) settingsPath() string {
	return filepath.Join(a.configDir, "settings.json")
}

func defaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "KoalaPull")
	}
	return filepath.Join(home, "Downloads", "KoalaPull")
}

func (a *App) GetSettings() Settings {
	path := a.settingsPath()
	data, err := os.ReadFile(path)
	if err == nil {
		var s Settings
		if json.Unmarshal(data, &s) == nil {
			if s.Theme == "" {
				s.Theme = "dark"
			}
			if s.MaxConcurrency < 1 {
				s.MaxConcurrency = 3
			}
			return s
		}
	}
	s := Settings{DefaultOutputDir: defaultOutputDir(), Theme: "dark", MaxConcurrency: 3, AutoPasteURL: false}
	a.writeSettings(s)
	return s
}

func (a *App) writeSettings(s Settings) {
	if data, err := json.MarshalIndent(s, "", "  "); err == nil {
		if err := os.WriteFile(a.settingsPath(), data, 0644); err != nil {
			println("writeSettings error:", err.Error())
		}
	}
}

func (a *App) UpdateSettings(s Settings) {
	old := a.GetSettings()
	a.writeSettings(s)
	if s.MaxConcurrency != old.MaxConcurrency && s.MaxConcurrency > 0 {
		a.semMu.Lock()
		a.downloadSemaphore = make(chan struct{}, s.MaxConcurrency)
		a.semMu.Unlock()
	}
}

func (a *App) SelectDirectory() (string, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Choose Download Directory",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// ---------- History ----------

func (a *App) historyPath() string {
	return filepath.Join(a.configDir, "history.json")
}

func (a *App) getHistoryLocked() []HistoryEntry {
	path := a.historyPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return []HistoryEntry{}
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return []HistoryEntry{}
	}
	return entries
}

func (a *App) GetHistory() []HistoryEntry {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	return a.getHistoryLocked()
}

func (a *App) saveHistoryEntry(entry HistoryEntry) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	entries := a.getHistoryLocked()
	entries = append([]HistoryEntry{entry}, entries...)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		println("saveHistoryEntry marshal error:", err.Error())
		return
	}
	if err := os.WriteFile(a.historyPath(), data, 0644); err != nil {
		println("saveHistoryEntry write error:", err.Error())
	}
}

func (a *App) ClearHistory() {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := os.WriteFile(a.historyPath(), []byte("[]"), 0644); err != nil {
		println("ClearHistory write error:", err.Error())
	}
}

func (a *App) DeleteHistoryEntry(downloadID string) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	entries := a.getHistoryLocked()
	filtered := make([]HistoryEntry, 0, len(entries))
	for _, e := range entries {
		if e.DownloadID != downloadID {
			filtered = append(filtered, e)
		}
	}
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		println("DeleteHistoryEntry marshal error:", err.Error())
		return
	}
	if err := os.WriteFile(a.historyPath(), data, 0644); err != nil {
		println("DeleteHistoryEntry write error:", err.Error())
	}
}

// ---------- Dependency Management ----------

func (a *App) CheckDependencies() DependencyStatus {
	return DependencyStatus{
		YtDlpInstalled:  fileExists(a.ytdlpPath()),
		FfmpegInstalled: fileExists(a.ffmpegPath()),
	}
}

func (a *App) GetYtdlpVersion() string {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, a.ytdlpPath(), "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (a *App) GetFfmpegVersion() string {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, a.ffmpegPath(), "-version").Output()
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(out), " ", 4)
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[2])
	}
	return ""
}

func (a *App) UpdateDependencies() error {
	if err := a.downloadYtdlp(true); err != nil {
		return err
	}
	return a.downloadFfmpeg(true)
}

func (a *App) OpenOutputDir() error {
	settings := a.GetSettings()
	dir := settings.DefaultOutputDir
	if dir == "" {
		dir = defaultOutputDir()
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func (a *App) GetVersionInfo() VersionInfo {
	return VersionInfo{
		Ytdlp:  a.GetYtdlpVersion(),
		Ffmpeg: a.GetFfmpegVersion(),
		App:    AppVersion,
	}
}

func (a *App) CheckForUpdates() UpdateInfo {
	info := UpdateInfo{}
	if latest, err := fetchLatestYtdlpVersion(a.ctx); err == nil && latest != "" {
		info.LatestYtdlpVersion = latest
		current := strings.TrimPrefix(a.GetYtdlpVersion(), "v")
		latestStr := strings.TrimPrefix(latest, "v")
		if current != "" && latestStr != current {
			info.YtdlpUpdateAvailable = true
		}
	}
	return info
}

func fetchLatestYtdlpVersion(ctx context.Context) (string, error) {
	dlCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.TagName, nil
}

func (a *App) emitProgress(dep string, pct int, status, errMsg string) {
	ev := DepProgress{
		Dependency: dep,
		Progress:   pct,
		Status:     status,
	}
	if errMsg != "" {
		ev.Error = errMsg
	}
	wailsRuntime.EventsEmit(a.ctx, "dependency-progress", ev)
}

func (a *App) emitDownloadProgress(downloadID string, percent float64, speed, eta, fileSize, status, errMsg, playlistStatus string) {
	ev := DownloadProgress{
		DownloadID:     downloadID,
		Percent:        percent,
		Speed:          speed,
		ETA:            eta,
		FileSize:       fileSize,
		Status:         status,
		PlaylistStatus: playlistStatus,
	}
	if errMsg != "" {
		ev.Error = errMsg
	}
	wailsRuntime.EventsEmit(a.ctx, "download-progress", ev)
}

func (a *App) DownloadDependencies() error {
	if err := a.downloadYtdlp(false); err != nil {
		a.emitProgress("yt-dlp", 0, "error", err.Error())
		return fmt.Errorf("yt-dlp download failed: %w", err)
	}
	if err := a.downloadFfmpeg(false); err != nil {
		a.emitProgress("ffmpeg", 0, "error", err.Error())
		return fmt.Errorf("ffmpeg download failed: %w", err)
	}
	return nil
}

func (a *App) downloadYtdlp(force bool) error {
	a.emitProgress("yt-dlp", 0, "downloading", "")
	destPath := a.ytdlpPath()
	if !force && fileExists(destPath) {
		a.emitProgress("yt-dlp", 100, "completed", "")
		return nil
	}
	url := ytdlpDownloadURL()
	tmpPath := destPath + ".tmp"
	dlCtx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	if err := a.downloadFile(dlCtx, url, tmpPath, "yt-dlp"); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename failed: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	}
	if runtime.GOOS == "darwin" {
		if err := exec.Command("xattr", "-d", "com.apple.quarantine", destPath).Run(); err != nil {
			println("xattr warning:", err.Error())
		}
	}
	a.emitProgress("yt-dlp", 100, "completed", "")
	return nil
}

func (a *App) downloadFfmpeg(force bool) error {
	a.emitProgress("ffmpeg", 0, "downloading", "")
	destPath := a.ffmpegPath()
	if !force && fileExists(destPath) {
		a.emitProgress("ffmpeg", 100, "completed", "")
		return nil
	}
	url := ffmpegDownloadURL()
	tmpDir, err := os.MkdirTemp("", "koalapull-ffmpeg")
	if err != nil {
		return fmt.Errorf("temp dir failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	archivePath := filepath.Join(tmpDir, "ffmpeg-archive")
	dlCtx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	if err := a.downloadFile(dlCtx, url, archivePath, "ffmpeg"); err != nil {
		return err
	}
	destDir := filepath.Dir(destPath)
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if err := extractFFmpegFromZip(archivePath, destDir); err != nil {
			return fmt.Errorf("zip extraction failed: %w", err)
		}
	} else {
		if err := extractFFmpegFromTarXz(archivePath, destDir); err != nil {
			return fmt.Errorf("tar extraction failed: %w", err)
		}
	}
	if !fileExists(destPath) {
		return fmt.Errorf("ffmpeg binary not found after extraction")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	}
	a.emitProgress("ffmpeg", 100, "completed", "")
	return nil
}

func (a *App) downloadFile(ctx context.Context, url, destPath, depName string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer out.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	total := resp.ContentLength
	lastPct := -1
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write failed: %w", writeErr)
			}
			written += int64(n)
			if total > 0 {
				pct := int(float64(written) / float64(total) * 100)
				if pct != lastPct {
					lastPct = pct
					a.emitProgress(depName, pct, "downloading", "")
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read failed: %w", readErr)
		}
	}
	return nil
}

func extractFFmpegFromZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if base != "ffmpeg" && base != "ffmpeg.exe" && base != "ffprobe" && base != "ffprobe.exe" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		binName := strings.TrimSuffix(base, ".exe")
		dst, err := os.Create(filepath.Join(destDir, binName))
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(dst, rc)
		if cerr := dst.Close(); cerr != nil {
			rc.Close()
			return errors.Join(err, cerr)
		}
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractFFmpegFromTarXz(archivePath, destDir string) error {
	tmpDir, err := os.MkdirTemp("", "koalapull-ffmpeg-extract")
	if err != nil {
		return fmt.Errorf("temp dir failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("tar", "-xJf", archivePath, "-C", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar failed: %s: %w", string(output), err)
	}

	var found bool
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return err
		}
		if !info.IsDir() && info.Mode().IsRegular() && filepath.Base(path) == "ffmpeg" {
			src, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}
			dst, createErr := os.Create(filepath.Join(destDir, "ffmpeg"))
			if createErr != nil {
				src.Close()
				return createErr
			}
			_, copyErr := io.Copy(dst, src)
			src.Close()
			if cerr := dst.Close(); cerr != nil && copyErr == nil {
				return cerr
			}
			if copyErr != nil {
				return copyErr
			}
			found = true
		}
		return nil
	})

	if !found {
		return fmt.Errorf("ffmpeg binary not found in archive")
	}
	return nil
}

func ytdlpDownloadURL() string {
	base := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
	switch runtime.GOOS {
	case "windows":
		return base + ".exe"
	case "darwin":
		return base + "_macos"
	default:
		return base + "_linux"
	}
}

func ffmpegDownloadURL() string {
	switch runtime.GOOS {
	case "windows":
		return "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	case "darwin":
		return "https://evermeet.cx/ffmpeg/get/zip"
	default:
		return "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
	}
}

// ---------- Metadata Fetching ----------

type rawFormat struct {
	FormatID   string `json:"format_id"`
	Ext        string `json:"ext"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	VCodec     string `json:"vcodec"`
	ACodec     string `json:"acodec"`
	Filesize   int64  `json:"filesize"`
	FormatNote string `json:"format_note"`
}

type rawEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type rawMetadata struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Thumbnail string      `json:"thumbnail"`
	Uploader  string      `json:"uploader"`
	Duration  float64     `json:"duration"`
	Type      string      `json:"_type"`
	Formats   []rawFormat `json:"formats"`
	Entries   []rawEntry  `json:"entries"`
}

func (a *App) FetchMetadata(url string) (*VideoMetadata, error) {
	args := []string{"--no-check-formats", "--no-warnings", "--dump-json", "--skip-download", "--flat-playlist", "--", url}
	cmd := exec.Command(a.ytdlpPath(), args...)
	stdout, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("yt-dlp exec failed: %w", err)
	}
	var raw rawMetadata
	if err := json.Unmarshal(stdout, &raw); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	if raw.Type == "playlist" {
		meta := &VideoMetadata{
			ID:         raw.ID,
			Title:      raw.Title,
			Thumbnail:  raw.Thumbnail,
			Uploader:   raw.Uploader,
			IsPlaylist: true,
			EntryCount: len(raw.Entries),
			Formats:    make([]FormatInfo, 0),
		}
		return meta, nil
	}

	meta := &VideoMetadata{
		ID:        raw.ID,
		Title:     raw.Title,
		Thumbnail: raw.Thumbnail,
		Uploader:  raw.Uploader,
		Duration:  raw.Duration,
		Formats:   make([]FormatInfo, 0, len(raw.Formats)),
	}
	for _, f := range raw.Formats {
		meta.Formats = append(meta.Formats, FormatInfo{
			FormatID:   f.FormatID,
			Ext:        f.Ext,
			Width:      f.Width,
			Height:     f.Height,
			VCodec:     f.VCodec,
			ACodec:     f.ACodec,
			Filesize:   f.Filesize,
			FormatNote: f.FormatNote,
		})
	}
	return meta, nil
}

// ---------- Download Execution ----------

const maxErrLines = 20

func (a *App) StartDownload(url, formatID, outputDir, container, subtitle, title string) (string, error) {
	if url == "" || formatID == "" {
		return "", fmt.Errorf("url and formatID are required")
	}
	if len(url) > 2048 || len(outputDir) > 4096 {
		return "", fmt.Errorf("input too long")
	}
	if outputDir == "" {
		settings := a.GetSettings()
		outputDir = settings.DefaultOutputDir
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("output dir: %w", err)
	}

	downloadID := a.nextDownloadID()
	template := filepath.Join(outputDir, "%(title)s [%(id)s].%(ext)s")

	args := []string{
		"--no-warnings",
		"-f", formatID,
		"--ffmpeg-location", a.ffmpegDir(),
		"--newline",
		"-o", template,
	}
	if container == "mp3" {
		args = append(args, "-x", "--audio-format", "mp3")
	} else if container != "" && container != "none" {
		args = append(args, "--merge-output-format", container)
	}
	switch subtitle {
	case "auto":
		args = append(args, "--write-auto-subs", "--sub-langs", "en", "--embed-subs")
	case "embed":
		args = append(args, "--sub-langs", "all", "--embed-subs")
	}
	args = append(args, "--", url)

	ctx, cancel := context.WithTimeout(a.ctx, 1*time.Hour)
	a.adMu.Lock()
	a.activeDownloads[downloadID] = cancel
	a.adMu.Unlock()

	go a.runDownload(ctx, downloadID, args, title, url, formatID)
	return downloadID, nil
}

func (a *App) CancelDownload(downloadID string) {
	a.adMu.Lock()
	cancel, ok := a.activeDownloads[downloadID]
	a.adMu.Unlock()
	if ok {
		cancel()
	}
}

func (a *App) runDownload(ctx context.Context, downloadID string, args []string, title, url, formatID string) {
	defer func() {
		if r := recover(); r != nil {
			println("runDownload panic:", fmt.Sprint(r))
		}
		a.adMu.Lock()
		delete(a.activeDownloads, downloadID)
		a.adMu.Unlock()
	}()

	a.semMu.Lock()
	sem := a.downloadSemaphore
	a.semMu.Unlock()
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "")
		return
	}
	defer func() { <-sem }()

	startTime := time.Now()
	a.emitDownloadProgress(downloadID, 0, "", "", "", "starting", "", "")

	cmd := exec.CommandContext(ctx, a.ytdlpPath(), args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: fmt.Sprintf("pipe failed: %v", err), StartTime: startTime, EndTime: time.Now()})
		a.emitDownloadProgress(downloadID, 0, "", "", "", "error", fmt.Sprintf("pipe failed: %v", err), "")
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: fmt.Sprintf("stdout pipe failed: %v", err), StartTime: startTime, EndTime: time.Now()})
		a.emitDownloadProgress(downloadID, 0, "", "", "", "error", fmt.Sprintf("stdout pipe failed: %v", err), "")
		return
	}

	if err := cmd.Start(); err != nil {
		if ctx.Err() == context.Canceled {
			a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "cancelled", StartTime: startTime, EndTime: time.Now()})
			a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "")
			return
		}
		a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: fmt.Sprintf("start failed: %v", err), StartTime: startTime, EndTime: time.Now()})
		a.emitDownloadProgress(downloadID, 0, "", "", "", "error", fmt.Sprintf("start failed: %v", err), "")
		return
	}

	done := make(chan struct{})
	go func() {
		defer func() { recover(); close(done) }()
		currentPS := ""
		lastPct := 0.0
		lastSpeed := ""
		lastETA := ""
		lastFileSz := ""
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if matches := sizeLineRegex.FindStringSubmatch(line); matches != nil {
				lastFileSz = matches[1]
				a.adMu.Lock()
				a.lastFileSize[downloadID] = matches[1]
				a.adMu.Unlock()
			}
			if pct, fileSz, speed, eta, ok := parseProgressLine(line); ok {
				lastPct, lastSpeed, lastETA, lastFileSz = pct, speed, eta, fileSz
				a.adMu.Lock()
				a.lastSpeed[downloadID] = speed
				a.lastFileSize[downloadID] = fileSz
				a.adMu.Unlock()
				a.emitDownloadProgress(downloadID, pct, speed, eta, fileSz, "downloading", "", currentPS)
			} else if ps := parsePlaylistStatus(line); ps != "" {
				currentPS = ps
				a.emitDownloadProgress(downloadID, lastPct, lastSpeed, lastETA, lastFileSz, "downloading", "", currentPS)
			}
		}
	}()

	errLines := make([]string, 0, maxErrLines)
	errCh := make(chan struct{})
	go func() {
		defer func() { recover(); close(errCh) }()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if len(errLines) >= maxErrLines {
				errLines = append(errLines[1:], line)
			} else {
				errLines = append(errLines, line)
			}
		}
	}()

	<-done
	<-errCh
	err = cmd.Wait()

	endTime := time.Now()

	a.adMu.Lock()
	fileSize := a.lastFileSize[downloadID]
	avgSpeed := a.lastSpeed[downloadID]
	delete(a.lastFileSize, downloadID)
	delete(a.lastSpeed, downloadID)
	a.adMu.Unlock()

	if err != nil {
		if ctx.Err() == context.Canceled {
			a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "cancelled", StartTime: startTime, EndTime: endTime})
			a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "")
			return
		}
		errMsg := strings.Join(errLines, "\n")
		if errMsg == "" {
			errMsg = err.Error()
		}
		a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: errMsg, StartTime: startTime, EndTime: endTime})
		a.emitDownloadProgress(downloadID, 0, "", "", "", "error", errMsg, "")
		return
	}

	a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, FileSize: fileSize, AvgSpeed: avgSpeed, Status: "completed", StartTime: startTime, EndTime: endTime})
	a.emitDownloadProgress(downloadID, 100, "", "", fileSize, "completed", "", "")
}

func parseProgressLine(line string) (percent float64, fileSize, speed, eta string, ok bool) {
	matches := progressRegex.FindStringSubmatch(line)
	if matches == nil {
		return 0, "", "", "", false
	}
	pct, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, "", "", "", false
	}
	return pct, strings.TrimSpace(matches[2]), strings.TrimSpace(matches[3]), strings.TrimSpace(matches[4]), true
}

func parsePlaylistStatus(line string) string {
	matches := playlistItemRegex.FindStringSubmatch(line)
	if matches == nil {
		return ""
	}
	return fmt.Sprintf("%s %s of %s", matches[1], matches[2], matches[3])
}
