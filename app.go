package main

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
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

type App struct {
	ctx            context.Context
	configDir      string
	binDir         string
	dlMu           sync.Mutex
	dlCounter      int
	activeDownloads map[string]context.CancelFunc
	adMu           sync.Mutex
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
	Status         string  `json:"status"`
	Error          string  `json:"error,omitempty"`
	PlaylistStatus string  `json:"playlistStatus,omitempty"`
}

type Settings struct {
	DefaultOutputDir string `json:"defaultOutputDir"`
}

var downloadSemaphore = make(chan struct{}, 3)

var progressRegex = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?[\d.]+\S+\s+at\s+([\d.]+\S+)\s+ETA\s+(\S+)`)

var playlistItemRegex = regexp.MustCompile(`\[download\]\s+Downloading\s+(video|item)\s+(\d+)\s+of\s+(\d+)`)

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	a.configDir = filepath.Join(configDir, "KoalaPull")
	a.binDir = filepath.Join(a.configDir, "bin")
	if err := os.MkdirAll(a.binDir, 0755); err != nil {
		println("Failed to create bin directory:", err.Error())
	}
	a.activeDownloads = make(map[string]context.CancelFunc)
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
		if json.Unmarshal(data, &s) == nil && s.DefaultOutputDir != "" {
			return s
		}
	}
	s := Settings{DefaultOutputDir: defaultOutputDir()}
	a.writeSettings(s)
	return s
}

func (a *App) writeSettings(s Settings) {
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(a.settingsPath(), data, 0644)
}

func (a *App) UpdateSettings(s Settings) {
	a.writeSettings(s)
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

// ---------- Dependency Management ----------

func (a *App) CheckDependencies() DependencyStatus {
	return DependencyStatus{
		YtDlpInstalled:  fileExists(a.ytdlpPath()),
		FfmpegInstalled: fileExists(a.ffmpegPath()),
	}
}

func (a *App) GetYtdlpVersion() string {
	out, err := exec.Command(a.ytdlpPath(), "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

func (a *App) emitDownloadProgress(downloadID string, percent float64, speed, eta, status, errMsg, playlistStatus string) {
	ev := DownloadProgress{
		DownloadID:     downloadID,
		Percent:        percent,
		Speed:          speed,
		ETA:            eta,
		Status:         status,
		PlaylistStatus: playlistStatus,
	}
	if errMsg != "" {
		ev.Error = errMsg
	}
	wailsRuntime.EventsEmit(a.ctx, "download-progress", ev)
}

func (a *App) DownloadDependencies() error {
	if err := a.downloadYtdlp(); err != nil {
		a.emitProgress("yt-dlp", 0, "error", err.Error())
		return fmt.Errorf("yt-dlp download failed: %w", err)
	}
	if err := a.downloadFfmpeg(); err != nil {
		a.emitProgress("ffmpeg", 0, "error", err.Error())
		return fmt.Errorf("ffmpeg download failed: %w", err)
	}
	return nil
}

func (a *App) downloadYtdlp() error {
	a.emitProgress("yt-dlp", 0, "downloading", "")
	destPath := a.ytdlpPath()
	if fileExists(destPath) {
		a.emitProgress("yt-dlp", 100, "completed", "")
		return nil
	}
	url := ytdlpDownloadURL()
	tmpPath := destPath + ".tmp"
	if err := a.downloadFile(url, tmpPath, "yt-dlp"); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	}
	if runtime.GOOS == "darwin" {
		exec.Command("xattr", "-d", "com.apple.quarantine", destPath).Run()
	}
	a.emitProgress("yt-dlp", 100, "completed", "")
	return nil
}

func (a *App) downloadFfmpeg() error {
	a.emitProgress("ffmpeg", 0, "downloading", "")
	destPath := a.ffmpegPath()
	if fileExists(destPath) {
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
	if err := a.downloadFile(url, archivePath, "ffmpeg"); err != nil {
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
		os.Chmod(destPath, 0755)
	}
	a.emitProgress("ffmpeg", 100, "completed", "")
	return nil
}

func (a *App) downloadFile(url, destPath, depName string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer out.Close()
	resp, err := http.Get(url)
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
		dst.Close()
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
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if writeErr := os.WriteFile(filepath.Join(destDir, "ffmpeg"), data, 0755); writeErr != nil {
				return writeErr
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
	args := []string{"--no-check-formats", "--no-warnings", "--dump-json", "--skip-download", "--flat-playlist", url}
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

func (a *App) StartDownload(url, formatID, outputDir, container, subtitle string) (string, error) {
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
	args = append(args, url)

	ctx, cancel := context.WithCancel(context.Background())
	a.adMu.Lock()
	a.activeDownloads[downloadID] = cancel
	a.adMu.Unlock()

	go a.runDownload(ctx, downloadID, args)
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

func (a *App) runDownload(ctx context.Context, downloadID string, args []string) {
	defer func() {
		a.adMu.Lock()
		delete(a.activeDownloads, downloadID)
		a.adMu.Unlock()
	}()

	select {
	case downloadSemaphore <- struct{}{}:
	case <-ctx.Done():
		a.emitDownloadProgress(downloadID, 0, "", "", "cancelled", "", "")
		return
	}
	defer func() { <-downloadSemaphore }()

	a.emitDownloadProgress(downloadID, 0, "", "", "starting", "", "")

	cmd := exec.CommandContext(ctx, a.ytdlpPath(), args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.emitDownloadProgress(downloadID, 0, "", "", "error", fmt.Sprintf("pipe failed: %v", err), "")
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.emitDownloadProgress(downloadID, 0, "", "", "error", fmt.Sprintf("stdout pipe failed: %v", err), "")
		return
	}

	if err := cmd.Start(); err != nil {
		if ctx.Err() == context.Canceled {
			a.emitDownloadProgress(downloadID, 0, "", "", "cancelled", "", "")
			return
		}
		a.emitDownloadProgress(downloadID, 0, "", "", "error", fmt.Sprintf("start failed: %v", err), "")
		return
	}

	done := make(chan struct{})
	go func() {
		currentPS := ""
		lastPct := 0.0
		lastSpeed := ""
		lastETA := ""
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if pct, speed, eta, ok := parseProgressLine(line); ok {
				lastPct, lastSpeed, lastETA = pct, speed, eta
				a.emitDownloadProgress(downloadID, pct, speed, eta, "downloading", "", currentPS)
			} else if ps := parsePlaylistStatus(line); ps != "" {
				currentPS = ps
				a.emitDownloadProgress(downloadID, lastPct, lastSpeed, lastETA, "downloading", "", currentPS)
			}
		}
		close(done)
	}()

	errLines := make([]string, 0, maxErrLines)
	errCh := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if len(errLines) >= maxErrLines {
				errLines = append(errLines[1:], line)
			} else {
				errLines = append(errLines, line)
			}
		}
		close(errCh)
	}()

	<-done
	<-errCh
	err = cmd.Wait()

	if err != nil {
		if ctx.Err() == context.Canceled {
			a.emitDownloadProgress(downloadID, 0, "", "", "cancelled", "", "")
			return
		}
		errMsg := strings.Join(errLines, "\n")
		if errMsg == "" {
			errMsg = err.Error()
		}
		a.emitDownloadProgress(downloadID, 0, "", "", "error", errMsg, "")
		return
	}

	a.emitDownloadProgress(downloadID, 100, "", "", "completed", "", "")
}

func parseProgressLine(line string) (percent float64, speed string, eta string, ok bool) {
	matches := progressRegex.FindStringSubmatch(line)
	if matches == nil {
		return 0, "", "", false
	}
	pct, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, "", "", false
	}
	return pct, strings.TrimSpace(matches[2]), strings.TrimSpace(matches[3]), true
}

func parsePlaylistStatus(line string) string {
	matches := playlistItemRegex.FindStringSubmatch(line)
	if matches == nil {
		return ""
	}
	return fmt.Sprintf("%s %s of %s", matches[1], matches[2], matches[3])
}
