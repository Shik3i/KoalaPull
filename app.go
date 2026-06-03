package main

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var AppVersion = "dev"

type App struct {
	ctx              context.Context
	configDir        string
	binDir           string
	settingsFilePath string
	startupErr       error
	settingsMu       sync.Mutex
	dependencyMu     sync.Mutex
	dlMu             sync.Mutex
	dlCounter        int
	activeDownloads  map[string]context.CancelFunc
	adMu             sync.Mutex
	lastFileSize     map[string]string
	lastSpeed        map[string]string
	semCount         atomic.Int32
	semLimit         atomic.Int32
	semWake          chan struct{}
	historyMu        sync.Mutex
	cachedSettings   Settings
	metadataMu       sync.Mutex
	metadataCancel   context.CancelFunc
	metadataSeq      uint64
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
	Language         string `json:"language"`
	DownloadPreset   string `json:"downloadPreset"`
	CustomFormatID   string `json:"customFormatId"`
	CustomContainer  string `json:"customContainer"`
	CustomSubtitle   string `json:"customSubtitle"`
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
	YtdlpUpdateAvailable     bool   `json:"ytdlpUpdateAvailable"`
	LatestYtdlpVersion       string `json:"latestYtdlpVersion"`
	KoalaPullUpdateAvailable bool   `json:"koalaPullUpdateAvailable"`
	LatestKoalaPullVersion   string `json:"latestKoalaPullVersion"`
}

var progressRegex = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?([\d.]+\S+)\s+at\s+([\d.]+\S+)(?:\s+ETA\s+(\S+))?`)

var sizeLineRegex = regexp.MustCompile(`\[download\]\s+100%\s+of\s+~?([\d.]+\S+)`)

var playlistItemRegex = regexp.MustCompile(`\[download\]\s+Downloading\s+(video|item)\s+(\d+)\s+of\s+(\d+)`)

const (
	defaultMaxConcurrency  = 3
	maxMaxConcurrency      = 10
	maxInputLength         = 2048
	maxPathLength          = 4096
	defaultDownloadPreset  = "compatible"
	defaultCustomFormatID  = "bestvideo+bestaudio/best"
	defaultCustomContainer = "mp4"
	defaultCustomSubtitle  = "none"
)

func NewApp() *App {
	a := &App{
		activeDownloads: make(map[string]context.CancelFunc),
		lastFileSize:    make(map[string]string),
		lastSpeed:       make(map[string]string),
		semWake:         make(chan struct{}, maxMaxConcurrency),
	}
	a.semLimit.Store(defaultMaxConcurrency)
	return a
}

func (a *App) appContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) initSemaphore() {
	s := a.GetSettings()
	a.semLimit.Store(int32(s.MaxConcurrency))
	if a.semWake == nil {
		a.semWake = make(chan struct{}, maxMaxConcurrency)
	}
}

func (a *App) semAcquire(ctx context.Context) bool {
	for {
		cur := a.semCount.Load()
		if cur >= a.semLimit.Load() {
			select {
			case <-a.semWake:
				continue
			case <-ctx.Done():
				return false
			}
		}
		if a.semCount.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
}

func (a *App) semRelease() {
	a.semCount.Add(-1)
	select {
	case a.semWake <- struct{}{}:
	default:
	}
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
	a.settingsFilePath = a.resolveSettingsPath()
	if err := os.MkdirAll(a.binDir, 0755); err != nil {
		a.startupErr = fmt.Errorf("create bin directory: %w", err)
		println("Failed to create bin directory:", err.Error())
	}
	a.activeDownloads = make(map[string]context.CancelFunc)
	a.initSemaphore()
	a.loadSettings()
	a.migrateHistoryIfNeeded()
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
	if a.settingsFilePath != "" {
		return a.settingsFilePath
	}
	return a.resolveSettingsPath()
}

func (a *App) portableSettingsPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "settings.json")
}

func isWritableDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".koalapull-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func (a *App) resolveSettingsPath() string {
	portable := a.portableSettingsPath()
	return resolveSettingsPathFor(a.configDir, portable)
}

func resolveSettingsPathFor(configDir, portablePath string) string {
	fallback := filepath.Join(configDir, "settings.json")
	if portablePath == "" {
		return fallback
	}
	if fileExists(portablePath) || isWritableDir(filepath.Dir(portablePath)) {
		if !fileExists(portablePath) && fileExists(fallback) {
			if data, err := os.ReadFile(fallback); err == nil {
				_ = os.WriteFile(portablePath, data, 0644)
			}
		}
		return portablePath
	}
	return fallback
}

func defaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "KoalaPull")
	}
	return filepath.Join(home, "Downloads", "KoalaPull")
}

func (a *App) loadSettings() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	path := a.settingsPath()
	data, err := os.ReadFile(path)
	if err == nil {
		var s Settings
		if json.Unmarshal(data, &s) == nil {
			a.cachedSettings = validateSettings(s)
			return
		}
	}
	s := validateSettings(Settings{
		DefaultOutputDir: defaultOutputDir(),
		Theme:            "dark",
		MaxConcurrency:   defaultMaxConcurrency,
		AutoPasteURL:     false,
		Language:         "en",
		DownloadPreset:   defaultDownloadPreset,
		CustomFormatID:   defaultCustomFormatID,
		CustomContainer:  defaultCustomContainer,
		CustomSubtitle:   defaultCustomSubtitle,
	})
	a.cachedSettings = s
	if err := a.writeSettingsLocked(s); err != nil {
		println("writeSettings error:", err.Error())
	}
}

func (a *App) GetSettings() Settings {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.getSettingsLocked()
}

func (a *App) getSettingsLocked() Settings {
	return a.cachedSettings
}

func (a *App) writeSettings(s Settings) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.writeSettingsLocked(s)
}

func (a *App) writeSettingsLocked(s Settings) error {
	s = validateSettings(s)
	a.cachedSettings = s
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(a.settingsPath(), data, 0644); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}
	return nil
}

func (a *App) UpdateSettings(s Settings) error {
	s = validateSettings(s)
	a.settingsMu.Lock()
	old := a.getSettingsLocked()
	if err := a.writeSettingsLocked(s); err != nil {
		a.settingsMu.Unlock()
		return err
	}
	a.settingsMu.Unlock()
	if s.MaxConcurrency != old.MaxConcurrency && s.MaxConcurrency > 0 {
		oldLimit := int(a.semLimit.Load())
		a.semLimit.Store(int32(s.MaxConcurrency))
		if s.MaxConcurrency > oldLimit {
			for i := oldLimit; i < s.MaxConcurrency; i++ {
				select {
				case a.semWake <- struct{}{}:
				default:
				}
			}
		}
	}
	return nil
}

func validateSettings(s Settings) Settings {
	if s.DefaultOutputDir == "" {
		s.DefaultOutputDir = defaultOutputDir()
	}
	if len(s.DefaultOutputDir) > maxPathLength {
		s.DefaultOutputDir = truncateToValidUTF8Prefix(s.DefaultOutputDir, maxPathLength)
	}
	if s.Theme != "dark" && s.Theme != "light" {
		s.Theme = "dark"
	}
	if s.MaxConcurrency < 1 {
		s.MaxConcurrency = defaultMaxConcurrency
	}
	if s.MaxConcurrency > maxMaxConcurrency {
		s.MaxConcurrency = maxMaxConcurrency
	}
	if s.Language != "en" && s.Language != "de" && s.Language != "fr" {
		s.Language = "en"
	}
	switch s.DownloadPreset {
	case "best", "compatible", "audio", "custom":
	default:
		s.DownloadPreset = defaultDownloadPreset
	}
	if s.CustomFormatID == "" {
		s.CustomFormatID = defaultCustomFormatID
	}
	if s.CustomContainer != "mp4" && s.CustomContainer != "mkv" && s.CustomContainer != "mp3" {
		s.CustomContainer = defaultCustomContainer
	}
	if s.CustomSubtitle != "none" && s.CustomSubtitle != "auto" && s.CustomSubtitle != "embed" {
		s.CustomSubtitle = defaultCustomSubtitle
	}
	return s
}

func truncateToValidUTF8Prefix(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if cut == 0 {
		return ""
	}
	return s[:cut]
}

func (a *App) SelectDirectory() (string, error) {
	title := "Choose Download Directory"
	switch a.GetSettings().Language {
	case "de":
		title = "Download-Ordner auswählen"
	case "fr":
		title = "Choisir le dossier de téléchargement"
	}
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: title,
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// ---------- History ----------

func (a *App) portableHistoryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "history.json")
}

func (a *App) historyPath() string {
	portable := a.portableHistoryPath()
	return resolveSettingsPathFor(a.configDir, portable)
}

func (a *App) migrateHistoryIfNeeded() {
	path := a.historyPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if len(data) == 0 {
		return
	}
	if data[0] != '[' {
		return
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		_ = enc.Encode(e)
	}
}

func readHistoryFromFile(path string) []HistoryEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	entries := make([]HistoryEntry, 0, 64)
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for dec.More() {
		var e HistoryEntry
		if err := dec.Decode(&e); err != nil {
			break
		}
		entries = append(entries, e)
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

func (a *App) GetHistory() []HistoryEntry {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	return readHistoryFromFile(a.historyPath())
}

func (a *App) saveHistoryEntry(entry HistoryEntry) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	f, err := os.OpenFile(a.historyPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		println("saveHistoryEntry open error:", err.Error())
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		println("saveHistoryEntry encode error:", err.Error())
	}
}

func (a *App) ClearHistory() {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := os.WriteFile(a.historyPath(), nil, 0644); err != nil {
		println("ClearHistory write error:", err.Error())
	}
}

func (a *App) DeleteHistoryEntry(downloadID string) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	path := a.historyPath()
	entries := readHistoryFromFile(path)
	filtered := make([]HistoryEntry, 0, len(entries))
	for _, e := range entries {
		if e.DownloadID != downloadID {
			filtered = append(filtered, e)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		println("DeleteHistoryEntry create error:", err.Error())
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range filtered {
		_ = enc.Encode(e)
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
	// retry once — macOS Gatekeeper can delay first launch
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(a.appContext(), 5*time.Second)
		out, err := commandOutput(ctx, a.ytdlpPath(), "--version")
		cancel()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
		if i == 0 {
			if ee, ok := err.(*exec.ExitError); ok {
				fmt.Fprintf(os.Stderr, "yt-dlp version attempt %d stderr: %s\n", i, string(ee.Stderr))
			} else {
				fmt.Fprintf(os.Stderr, "yt-dlp version attempt %d: %v\n", i, err)
			}
			time.Sleep(2 * time.Second)
		}
	}
	return ""
}

func (a *App) GetFfmpegVersion() string {
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(a.appContext(), 5*time.Second)
		out, err := commandOutput(ctx, a.ffmpegPath(), "-version")
		cancel()
		if err == nil {
			parts := strings.SplitN(string(out), " ", 4)
			if len(parts) >= 3 {
				return strings.TrimSpace(parts[2])
			}
		}
		if i == 0 {
			if ee, ok := err.(*exec.ExitError); ok {
				fmt.Fprintf(os.Stderr, "ffmpeg version attempt %d stderr: %s\n", i, string(ee.Stderr))
			} else {
				fmt.Fprintf(os.Stderr, "ffmpeg version attempt %d: %v\n", i, err)
			}
			time.Sleep(2 * time.Second)
		}
	}
	return ""
}

func (a *App) UpdateDependencies() error {
	a.dependencyMu.Lock()
	defer a.dependencyMu.Unlock()
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
		cmd = command("open", dir)
	case "windows":
		cmd = command("explorer", dir)
	default:
		cmd = command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			println("OpenOutputDir wait error:", err.Error())
		}
	}()
	return nil
}

func (a *App) GetVersionInfo() VersionInfo {
	vi := VersionInfo{App: AppVersion}
	done := make(chan struct{}, 2)
	go func() {
		vi.Ytdlp = a.GetYtdlpVersion()
		done <- struct{}{}
	}()
	go func() {
		vi.Ffmpeg = a.GetFfmpegVersion()
		done <- struct{}{}
	}()
	<-done
	<-done
	return vi
}

func (a *App) GetAppVersion() string {
	return AppVersion
}

func (a *App) CheckForUpdates() UpdateInfo {
	info := UpdateInfo{}
	if latest, err := fetchLatestYtdlpVersion(a.appContext()); err == nil && latest != "" {
		info.LatestYtdlpVersion = latest
		current := strings.TrimPrefix(a.GetYtdlpVersion(), "v")
		latestStr := strings.TrimPrefix(latest, "v")
		if current != "" && latestStr != current {
			info.YtdlpUpdateAvailable = true
		}
	}
	if AppVersion != "" && AppVersion != "dev" && !strings.Contains(AppVersion, "-") {
		if latest, err := fetchLatestKoalaPullVersion(a.appContext()); err == nil && latest != "" {
			info.LatestKoalaPullVersion = latest
			if latest != AppVersion {
				info.KoalaPullUpdateAvailable = true
			}
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
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.TagName, nil
}

func fetchLatestKoalaPullVersion(ctx context.Context) (string, error) {
	dlCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, "https://api.github.com/repos/Shik3i/KoalaPull/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.TagName, nil
}

func (a *App) OpenExternalLink(url string) {
	wailsRuntime.BrowserOpenURL(a.appContext(), url)
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
	wailsRuntime.EventsEmit(a.appContext(), "dependency-progress", ev)
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
	wailsRuntime.EventsEmit(a.appContext(), "download-progress", ev)
}

func (a *App) DownloadDependencies() error {
	if a.startupErr != nil {
		return fmt.Errorf("app setup failed: %w", a.startupErr)
	}
	a.dependencyMu.Lock()
	defer a.dependencyMu.Unlock()
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
	dlCtx, cancel := context.WithTimeout(a.appContext(), 10*time.Minute)
	defer cancel()
	if err := a.downloadFile(dlCtx, url, tmpPath, "yt-dlp"); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := verifyYtdlpChecksum(dlCtx, tmpPath, filepath.Base(url)); err != nil {
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
		if err := command("xattr", "-d", "com.apple.quarantine", destPath).Run(); err != nil {
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
	dlCtx, cancel := context.WithTimeout(a.appContext(), 10*time.Minute)
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
	defer func() {
		if out != nil {
			_ = out.Close()
		}
	}()
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
	if err := out.Close(); err != nil {
		return fmt.Errorf("close file failed: %w", err)
	}
	out = nil
	return nil
}

func verifyYtdlpChecksum(ctx context.Context, filePath, assetName string) error {
	expected, err := fetchChecksumForAsset(ctx, ytdlpChecksumsURL(), assetName)
	if err != nil {
		return fmt.Errorf("fetch yt-dlp checksum: %w", err)
	}
	actual, err := sha256File(filePath)
	if err != nil {
		return fmt.Errorf("hash yt-dlp: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("yt-dlp checksum mismatch")
	}
	return nil
}

func fetchChecksumForAsset(ctx context.Context, checksumURL, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad checksum status: %s", resp.Status)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "./")
		if filepath.Base(name) == assetName {
			sum := strings.ToLower(fields[0])
			if len(sum) != sha256.Size*2 {
				return "", fmt.Errorf("invalid checksum length for %s", assetName)
			}
			if _, err := hex.DecodeString(sum); err != nil {
				return "", fmt.Errorf("invalid checksum for %s: %w", assetName, err)
			}
			return sum, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found", assetName)
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
		binName := ffmpegZipDestName(base, runtime.GOOS)
		dstPath := filepath.Join(destDir, binName)
		dst, err := os.Create(dstPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(dst, rc)
		closeErr := dst.Close()
		rcErr := rc.Close()
		if copyErr != nil || closeErr != nil || rcErr != nil {
			os.Remove(dstPath)
			return errors.Join(copyErr, closeErr, rcErr)
		}
	}
	return nil
}

func validateTarXzArchive(archivePath string) error {
	cmd := command("tar", "-tJf", archivePath)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("tar list failed: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if err := validateArchiveMemberName(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func validateArchiveMemberName(name string) error {
	cleaned := filepath.Clean(name)
	if name == "" || filepath.IsAbs(name) || strings.HasPrefix(cleaned, "..") || filepath.VolumeName(name) != "" {
		return fmt.Errorf("unsafe archive path: %q", name)
	}
	return nil
}

func ffmpegZipDestName(base, goos string) string {
	if goos == "windows" {
		return base
	}
	return strings.TrimSuffix(base, ".exe")
}

func extractFFmpegFromTarXz(archivePath, destDir string) error {
	tmpDir, err := os.MkdirTemp("", "koalapull-ffmpeg-extract")
	if err != nil {
		return fmt.Errorf("temp dir failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := validateTarXzArchive(archivePath); err != nil {
		return err
	}

	cmd := command("tar", "-xJf", archivePath, "-C", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar failed: %s: %w", string(output), err)
	}

	var found bool
	var walkErr error
	walkErr = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		if !info.IsDir() && info.Mode().IsRegular() && filepath.Base(path) == "ffmpeg" {
			src, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}
			defer src.Close()
			dst, createErr := os.Create(filepath.Join(destDir, "ffmpeg"))
			if createErr != nil {
				return createErr
			}
			_, copyErr := io.Copy(dst, src)
			closeErr := dst.Close()
			if copyErr != nil {
				os.Remove(filepath.Join(destDir, "ffmpeg"))
				return copyErr
			}
			if closeErr != nil {
				os.Remove(filepath.Join(destDir, "ffmpeg"))
				return closeErr
			}
			found = true
		}
		return nil
	})

	if walkErr != nil {
		return fmt.Errorf("walk temp dir: %w", walkErr)
	}
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

func ytdlpChecksumsURL() string {
	return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS"
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
	if url == "" || len(url) > maxInputLength {
		return nil, fmt.Errorf("url is required and must be at most %d characters", maxInputLength)
	}
	if !isAllowedDownloadURL(url) {
		return nil, fmt.Errorf("url must use http or https")
	}
	args := []string{"--no-check-formats", "--no-warnings", "--dump-json", "--skip-download", "--flat-playlist", "--", url}
	ctx, cancel, seq := a.newMetadataContext()
	defer a.clearMetadataContext(cancel, seq)
	stdout, err := commandOutput(ctx, a.ytdlpPath(), args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("yt-dlp metadata timed out: %w", ctx.Err())
		}
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

func (a *App) newMetadataContext() (context.Context, context.CancelFunc, uint64) {
	a.metadataMu.Lock()
	defer a.metadataMu.Unlock()
	if a.metadataCancel != nil {
		a.metadataCancel()
	}
	ctx, cancel := context.WithTimeout(a.appContext(), 30*time.Second)
	a.metadataCancel = cancel
	a.metadataSeq++
	return ctx, cancel, a.metadataSeq
}

func (a *App) clearMetadataContext(cancel context.CancelFunc, seq uint64) {
	a.metadataMu.Lock()
	defer a.metadataMu.Unlock()
	if a.metadataSeq == seq {
		a.metadataCancel = nil
	}
	cancel()
}

// ---------- Download Execution ----------

const maxErrLines = 20

func (a *App) StartDownload(url, formatID, outputDir, container, subtitle, title string) (string, error) {
	return a.StartDownloadWithPreset(url, formatID, outputDir, container, subtitle, title, defaultDownloadPreset)
}

func (a *App) StartDownloadWithPreset(url, formatID, outputDir, container, subtitle, title, preset string) (string, error) {
	if url == "" || formatID == "" {
		return "", fmt.Errorf("url and formatID are required")
	}
	if !isAllowedDownloadURL(url) {
		return "", fmt.Errorf("url must use http or https")
	}
	if len(url) > maxInputLength || len(outputDir) > maxPathLength {
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
	args = append(args, downloadPostProcessingArgs(preset, container, subtitle)...)
	args = append(args, "--", url)

	ctx, cancel := context.WithCancel(a.appContext())
	a.adMu.Lock()
	a.activeDownloads[downloadID] = cancel
	a.adMu.Unlock()

	go a.runDownload(ctx, cancel, downloadID, args, title, url, formatID)
	return downloadID, nil
}

func downloadPostProcessingArgs(preset, container, subtitle string) []string {
	switch preset {
	case "audio":
		return []string{"-x", "--audio-format", "mp3"}
	case "compatible":
		return []string{"--recode-video", "mp4"}
	default:
		args := make([]string, 0, 4)
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
		return args
	}
}

func isAllowedDownloadURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func (a *App) CancelDownload(downloadID string) {
	a.adMu.Lock()
	cancel, ok := a.activeDownloads[downloadID]
	a.adMu.Unlock()
	if ok {
		cancel()
	}
}

func (a *App) runDownload(ctx context.Context, cancel context.CancelFunc, downloadID string, args []string, title, url, formatID string) {
	defer func() {
		cancel()
		if r := recover(); r != nil {
			println("runDownload panic:", fmt.Sprint(r))
			debug.PrintStack()
		}
		a.adMu.Lock()
		delete(a.activeDownloads, downloadID)
		delete(a.lastFileSize, downloadID)
		delete(a.lastSpeed, downloadID)
		a.adMu.Unlock()
	}()

	if !a.semAcquire(ctx) {
		a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "")
		return
	}
	defer a.semRelease()

	startTime := time.Now()
	a.emitDownloadProgress(downloadID, 0, "", "", "", "starting", "", "")

	var lastProgress atomic.Value
	lastProgress.Store(time.Now())

	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				return
			}
			a.emitDownloadProgress(downloadID, 0, "", "", "", "retrying", "", "")
			time.Sleep(2 * time.Second)
			lastProgress.Store(time.Now())
		}

		attemptCtx, attemptCancel := context.WithCancel(ctx)

		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if time.Since(lastProgress.Load().(time.Time)) > 5*time.Minute {
						attemptCancel()
						return
					}
				case <-attemptCtx.Done():
					return
				}
			}
		}()

		func() {
			cmd := commandContext(attemptCtx, a.ytdlpPath(), args...)

			stderr, err := cmd.StderrPipe()
			if err != nil {
				return
			}

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return
			}

			if err := startCommand(attemptCtx, cmd); err != nil {
				if ctx.Err() == context.Canceled {
					a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "cancelled", StartTime: startTime, EndTime: time.Now()})
					a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "")
					return
				}
				return
			}

			type downloadLine struct {
				source string
				line   string
				err    error
			}

			lineCh := make(chan downloadLine, 128)
			var readWG sync.WaitGroup
			scanPipe := func(source string, r io.Reader) {
				defer readWG.Done()
				defer func() {
					if r := recover(); r != nil {
						println("scanPipe panic:", fmt.Sprint(r))
					}
				}()
				scanner := bufio.NewScanner(r)
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				for scanner.Scan() {
					lineCh <- downloadLine{source: source, line: scanner.Text()}
				}
				if scanErr := scanner.Err(); scanErr != nil {
					lineCh <- downloadLine{source: source, err: scanErr}
				}
			}
			readWG.Add(2)
			go scanPipe("stdout", stdout)
			go scanPipe("stderr", stderr)
			go func() {
				readWG.Wait()
				close(lineCh)
			}()

			currentPS := ""
			lastPct := 0.0
			lastSpeed := ""
			lastETA := ""
			lastFileSz := ""
			errLines := make([]string, 0, maxErrLines)
			var stdoutScanErr error
			var stderrScanErr error
			for item := range lineCh {
				if item.err != nil {
					if item.source == "stderr" {
						stderrScanErr = item.err
					} else {
						stdoutScanErr = item.err
					}
					continue
				}
				line := item.line
				if item.source == "stderr" {
					if len(errLines) >= maxErrLines {
						errLines = append(errLines[1:], line)
					} else {
						errLines = append(errLines, line)
					}
				}
				if matches := sizeLineRegex.FindStringSubmatch(line); matches != nil {
					lastFileSz = matches[1]
					lastProgress.Store(time.Now())
					a.adMu.Lock()
					a.lastFileSize[downloadID] = matches[1]
					a.adMu.Unlock()
				}
				if pct, fileSz, speed, eta, ok := parseProgressLine(line); ok {
					lastPct, lastSpeed, lastETA, lastFileSz = pct, speed, eta, fileSz
					lastProgress.Store(time.Now())
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

			err = cmd.Wait()
			if err == nil {
				err = errors.Join(stdoutScanErr, stderrScanErr)
			}

			endTime := time.Now()

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

			a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, FileSize: lastFileSz, AvgSpeed: lastSpeed, Status: "completed", StartTime: startTime, EndTime: endTime})
			a.emitDownloadProgress(downloadID, 100, "", "", lastFileSz, "completed", "", "")
		}()

		attemptCancel()

		// Only retry if attempt was cancelled by idle timeout (not user cancel)
		if attempt == 0 && errors.Is(attemptCtx.Err(), context.Canceled) && ctx.Err() == nil {
			continue
		}
		return
	}
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

func collectRecentLines(r io.Reader, limit int) ([]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	lines := make([]string, 0, limit)
	for scanner.Scan() {
		line := scanner.Text()
		if limit <= 0 {
			continue
		}
		if len(lines) >= limit {
			lines = append(lines[1:], line)
		} else {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
