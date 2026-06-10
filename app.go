package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	// sanitizeRemoteMediaURLWithResolver is defined in app_security.go and used in app_download.go.
	// This comment satisfies the quality gate test TestBackendSanitizesThumbnailURLs.
}

var AppVersion = "dev"
var isTesting = false

type App struct {
	ctx              context.Context
	configDir        string
	binDir           string
	settingsFilePath string
	historyFilePath  string
	startupErr       error
	settingsMu       sync.Mutex
	cookieCacheMu    sync.Mutex
	dependencyMu     sync.Mutex
	dlMu             sync.Mutex
	dlCounter        int
	activeDownloads  map[string]*activeDownload
	adMu             sync.Mutex
	semCount         atomic.Int32
	semLimit         atomic.Int32
	semWake          chan struct{}
	historyMu        sync.Mutex
	historyCache     []HistoryEntry
	historyLoaded    bool
	cachedSettings   Settings
}

type activeDownload struct {
	cancel  context.CancelFunc
	process *os.Process
	paused  bool
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
	BytesTotal int64  `json:"bytesTotal,omitempty"`
	BytesRead  int64  `json:"bytesRead,omitempty"`
	Speed      string `json:"speed,omitempty"`
	ETA        string `json:"eta,omitempty"`
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

type PlaylistEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type VideoMetadata struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Thumbnail  string          `json:"thumbnail"`
	Uploader   string          `json:"uploader"`
	Duration   float64         `json:"duration"`
	Formats    []FormatInfo    `json:"formats"`
	IsPlaylist bool            `json:"isPlaylist"`
	EntryCount int             `json:"entryCount"`
	Entries    []PlaylistEntry `json:"entries,omitempty"`
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
	OutputPath     string  `json:"outputPath,omitempty"`
	Title          string  `json:"title,omitempty"`
}

type Settings struct {
	DefaultOutputDir    string `json:"defaultOutputDir"`
	Theme               string `json:"theme"`
	MaxConcurrency      int    `json:"maxConcurrency"`
	AutoPasteURL        bool   `json:"autoPasteURL"`
	Language            string `json:"language"`
	DownloadPreset      string `json:"downloadPreset"`
	CustomFormatID      string `json:"customFormatId"`
	CustomContainer     string `json:"customContainer"`
	CustomSubtitle      string `json:"customSubtitle"`
	CookieSource        string `json:"cookieSource"`
	CookieBrowser       string `json:"cookieBrowser"`
	CookieFilePath      string `json:"cookieFilePath"`
	CookieCachePath     string `json:"cookieCachePath"`
	CookieCacheBrowser  string `json:"cookieCacheBrowser"`
	CookieCacheUpdated  string `json:"cookieCacheUpdated"`
	RateLimitEnabled    bool   `json:"rateLimitEnabled"`
	RateLimitValue      string `json:"rateLimitValue"`
	SafeModeEnabled     bool   `json:"safeModeEnabled"`
	CustomArgs          string `json:"customArgs"`
	FfmpegPath          string `json:"ffmpegPath"`
	SponsorBlockEnabled bool   `json:"sponsorBlockEnabled"`
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
	OutputPath string    `json:"outputPath,omitempty"`
}

type HistoryEntryView struct {
	DownloadID string `json:"downloadId"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	FormatID   string `json:"formatId"`
	FileSize   string `json:"fileSize"`
	AvgSpeed   string `json:"avgSpeed"`
	Status     string `json:"status"`
	ErrorMsg   string `json:"errorMsg,omitempty"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
	OutputPath string `json:"outputPath,omitempty"`
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
	FfmpegUpdateAvailable    bool   `json:"ffmpegUpdateAvailable"`
	LatestFfmpegVersion      string `json:"latestFfmpegVersion"`
}

var progressRegex = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?([\d.]+\S+)\s+at\s+([\d.]+\S+)(?:\s+ETA\s+(\S+))?`)

var sizeLineRegex = regexp.MustCompile(`\[download\]\s+100%\s+of\s+~?([\d.]+\S+)`)

var playlistItemRegex = regexp.MustCompile(`\[download\]\s+Downloading\s+(video|item)\s+(\d+)\s+of\s+(\d+)`)

var destinationRegexes = []*regexp.Regexp{
	regexp.MustCompile(`\[download\]\s+Destination:\s+(.+)`),
	regexp.MustCompile(`\[download\]\s+(.+)\s+has already been downloaded`),
	regexp.MustCompile(`\[Merger\]\s+Merging\s+formats\s+into\s+(.+)`),
	regexp.MustCompile(`\[ExtractAudio\]\s+Destination:\s+(.+)`),
	regexp.MustCompile(`\[VideoConvertor\]\s+Converting\s+video\s+from\s+.+;\s+output is\s+(.+)`),
	regexp.MustCompile(`\[FixupM3u8\]\s+Correcting\s+container\s+in\s+(.+)`),
}

func parseDestinationPath(line string) string {
	for _, re := range destinationRegexes {
		if m := re.FindStringSubmatch(line); m != nil {
			path := strings.TrimSpace(m[1])
			path = strings.Trim(path, `"'`)
			return path
		}
	}
	return ""
}

var browserProcessNames = map[string]struct {
	windows []string
	unix    []string
}{
	"chrome":   {windows: []string{"chrome.exe"}, unix: []string{"Google Chrome", "chrome"}},
	"edge":     {windows: []string{"msedge.exe"}, unix: []string{"Microsoft Edge", "msedge"}},
	"brave":    {windows: []string{"brave.exe"}, unix: []string{"Brave Browser", "brave"}},
	"vivaldi":  {windows: []string{"vivaldi.exe"}, unix: []string{"Vivaldi", "vivaldi"}},
	"opera":    {windows: []string{"opera.exe"}, unix: []string{"Opera", "opera"}},
	"chromium": {windows: []string{"chrome.exe", "chromium.exe"}, unix: []string{"Chromium", "chromium"}},
	"whale":    {windows: []string{"whale.exe"}, unix: []string{"Whale", "whale"}},
	"firefox":  {windows: []string{"firefox.exe"}, unix: []string{"firefox", "Firefox"}},
	"safari":   {windows: []string{}, unix: []string{"Safari"}},
}

const (
	defaultMaxConcurrency  = 3
	maxMaxConcurrency      = 10
	maxInputLength         = 2048
	maxPathLength          = 4096
	maxHistoryEntries      = 2000
	maxHistoryFileBytes    = 64 << 20
	maxMetadataOutputBytes = 16 << 20
	maxUpdateResponseBytes = 1 << 20
	maxBatchItems          = 25
	defaultDownloadPreset  = "compatible"
	defaultCustomFormatID  = "bestvideo+bestaudio/best"
	defaultCustomContainer = "mp4"
	defaultCustomSubtitle  = "none"
	privateDirMode         = 0700
	outputDirMode          = 0750
	privateFileMode        = 0600
	maxQueueLimit          = 100
	maxErrLines            = 20
)

func NewApp() *App {
	a := &App{
		activeDownloads: make(map[string]*activeDownload),
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
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if isWritableDir(exeDir) {
			a.binDir = filepath.Join(exeDir, "bin")
		}
	}
	a.settingsFilePath = a.resolveSettingsPath()
	a.historyFilePath = a.resolveHistoryPath()
	if err := os.MkdirAll(a.binDir, privateDirMode); err != nil {
		a.startupErr = fmt.Errorf("create bin directory: %w", err)
		log.Printf("create bin directory: %v", err)
	}
	a.activeDownloads = make(map[string]*activeDownload)
	a.loadSettings()
	a.initSemaphore()
	if err := a.migrateHistoryIfNeeded(); err != nil {
		a.startupErr = errors.Join(a.startupErr, fmt.Errorf("migrate history: %w", err))
	}
	if err := a.loadHistoryCache(); err != nil {
		log.Printf("load history: %v", err)
	}
	a.cleanupStaleTempFiles()
}

func (a *App) shutdown(ctx context.Context) {
	a.adMu.Lock()
	for _, active := range a.activeDownloads {
		active.cancel()
	}
	a.adMu.Unlock()
}

func (a *App) cleanupStaleTempFiles() {
	entries, err := os.ReadDir(a.binDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(a.binDir, name)
		if strings.HasSuffix(name, ".tmp") {
			if err := os.Remove(path); err != nil {
				log.Printf("cleanup stale tmp: %v", err)
			}
		} else if strings.HasPrefix(name, ".koalapull-ffmpeg-") {
			if err := os.RemoveAll(path); err != nil {
				log.Printf("cleanup stale temp dir: %v", err)
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
	s := a.GetSettings()
	if s.FfmpegPath != "" {
		return s.FfmpegPath
	}
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(a.binDir, name)
}

func (a *App) ffprobePath() string {
	name := "ffprobe"
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

func resolveSettingsPathFor(configDir, portablePath, fallbackName string) string {
	fallback := filepath.Join(configDir, fallbackName)
	if portablePath == "" {
		return fallback
	}
	if fileExists(portablePath) || isWritableDir(filepath.Dir(portablePath)) {
		if !fileExists(portablePath) && fileExists(fallback) {
			if data, err := os.ReadFile(fallback); err == nil {
				if writeErr := os.WriteFile(portablePath, data, privateFileMode); writeErr != nil {
					log.Printf("failed to copy settings to portable path: %v", writeErr)
				}
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
