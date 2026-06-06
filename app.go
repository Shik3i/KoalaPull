package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
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
	return resolveSettingsPathFor(a.configDir, portable, "settings.json")
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
		DefaultOutputDir:    defaultOutputDir(),
		Theme:               "dark",
		MaxConcurrency:      defaultMaxConcurrency,
		AutoPasteURL:        false,
		Language:            "en",
		DownloadPreset:      defaultDownloadPreset,
		CustomFormatID:      defaultCustomFormatID,
		CustomContainer:     defaultCustomContainer,
		CustomSubtitle:      defaultCustomSubtitle,
		CookieSource:        "none",
		CookieBrowser:       "chrome",
		CookieFilePath:      "",
		CookieCachePath:     "",
		CookieCacheBrowser:  "",
		CookieCacheUpdated:  "",
		RateLimitEnabled:    false,
		RateLimitValue:      "1",
		SafeModeEnabled:     false,
		CustomArgs:          "",
		FfmpegPath:          "",
		SponsorBlockEnabled: false,
	})
	a.cachedSettings = s
	if err := a.writeSettingsLocked(s); err != nil {
		log.Printf("writeSettings: %v", err)
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
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := writeFileAtomically(a.settingsPath(), data, privateFileMode); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}
	a.cachedSettings = s
	return nil
}

func (a *App) UpdateSettings(s Settings) error {
	s = validateSettings(s)
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	old := a.getSettingsLocked()
	if s.CookieCachePath == "" &&
		old.CookieCachePath != "" &&
		s.CookieSource == "browser" &&
		strings.EqualFold(s.CookieBrowser, old.CookieCacheBrowser) {
		s.CookieCachePath = old.CookieCachePath
		s.CookieCacheBrowser = old.CookieCacheBrowser
		s.CookieCacheUpdated = old.CookieCacheUpdated
	}
	if err := a.writeSettingsLocked(s); err != nil {
		return err
	}
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
	if cleaned, err := cleanAbsolutePath(s.DefaultOutputDir); err == nil && !isFilesystemRoot(cleaned) {
		s.DefaultOutputDir = cleaned
	} else {
		s.DefaultOutputDir = defaultOutputDir()
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
	isValidContainer := false
	switch s.CustomContainer {
	case "mp4", "mkv", "webm", "mp3", "aac", "m4a", "opus", "flac", "wav":
		isValidContainer = true
	}
	if !isValidContainer {
		s.CustomContainer = defaultCustomContainer
	}
	if s.CustomSubtitle != "none" && s.CustomSubtitle != "auto" && s.CustomSubtitle != "embed" {
		s.CustomSubtitle = defaultCustomSubtitle
	}
	if s.CookieSource != "none" && s.CookieSource != "browser" && s.CookieSource != "file" {
		s.CookieSource = "none"
	}
	switch s.CookieBrowser {
	case "brave", "chrome", "chromium", "edge", "firefox", "opera", "safari", "vivaldi", "whale":
	default:
		s.CookieBrowser = "chrome"
	}
	if len(s.CookieFilePath) > maxPathLength {
		s.CookieFilePath = truncateToValidUTF8Prefix(s.CookieFilePath, maxPathLength)
	}
	if s.CookieFilePath != "" {
		if cleaned, err := cleanAbsolutePath(s.CookieFilePath); err == nil {
			s.CookieFilePath = cleaned
		} else {
			s.CookieFilePath = ""
		}
	}
	if s.CookieCacheBrowser != "" {
		if _, ok := browserProcessNames[strings.ToLower(s.CookieCacheBrowser)]; !ok {
			s.CookieCacheBrowser = ""
		} else {
			s.CookieCacheBrowser = strings.ToLower(s.CookieCacheBrowser)
		}
	}
	if len(s.CookieCachePath) > maxPathLength {
		s.CookieCachePath = truncateToValidUTF8Prefix(s.CookieCachePath, maxPathLength)
	}
	if s.CookieCachePath != "" {
		if cleaned, err := cleanAbsolutePath(s.CookieCachePath); err == nil && !isFilesystemRoot(cleaned) {
			s.CookieCachePath = cleaned
		} else {
			s.CookieCachePath = ""
			s.CookieCacheBrowser = ""
			s.CookieCacheUpdated = ""
		}
	}
	if len(s.CookieCacheUpdated) > 64 {
		s.CookieCacheUpdated = truncateToValidUTF8Prefix(s.CookieCacheUpdated, 64)
	}
	if s.RateLimitValue == "" {
		s.RateLimitValue = "1"
	}
	if len(s.RateLimitValue) > 32 {
		s.RateLimitValue = truncateToValidUTF8Prefix(s.RateLimitValue, 32)
	}
	cleanedVal := strings.ReplaceAll(s.RateLimitValue, ",", ".")
	cleanedVal = strings.TrimSpace(cleanedVal)
	if _, err := strconv.ParseFloat(cleanedVal, 64); err != nil {
		s.RateLimitValue = "1"
	}
	if len(s.CustomArgs) > 1024 {
		s.CustomArgs = truncateToValidUTF8Prefix(s.CustomArgs, 1024)
	}
	if s.FfmpegPath != "" {
		cleaned, err := cleanAbsolutePath(s.FfmpegPath)
		base := strings.ToLower(filepath.Base(cleaned))
		if err != nil || (!strings.HasPrefix(base, "ffmpeg") && base != "ffmpeg.exe") {
			s.FfmpegPath = ""
		} else {
			s.FfmpegPath = cleaned
		}
	}
	return s
}

func parseRateLimitToBytes(valStr string) int64 {
	valStr = strings.ReplaceAll(valStr, ",", ".")
	valStr = strings.TrimSpace(valStr)
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil || val <= 0 {
		return 0
	}
	return int64(val * 1024 * 1024)
}

func parseCustomArgs(s string) []string {
	var args []string
	var current strings.Builder
	inDoubleQuotes := false
	inSingleQuotes := false
	escaped := false

	for i := 0; i < len(s); i++ {
		r := s[i]
		if escaped {
			current.WriteByte(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingleQuotes {
			escaped = true
			continue
		}
		if r == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			continue
		}
		if r == '\'' && !inDoubleQuotes {
			inSingleQuotes = !inSingleQuotes
			continue
		}
		if (r == ' ' || r == '\t' || r == '\n' || r == '\r') && !inDoubleQuotes && !inSingleQuotes {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

var blockedCustomArgNames = map[string]struct{}{
	"--exec":                     {},
	"--exec-before-download":     {},
	"--external-downloader":      {},
	"--external-downloader-args": {},
	"--downloader-args":          {},
	"--use-postprocessor":        {},
	"--postprocessor-args":       {},
	"--load-info-json":           {},
	"--config-locations":         {},
	"--alias":                    {},
	"--plugin-dirs":              {},
	"--enable-file-urls":         {},
	"--paths":                    {},
	"-P":                         {},
	"--output":                   {},
	"-o":                         {},
	"--download-archive":         {},
	"--cache-dir":                {},
	"--rm-cache-dir":             {},
	"--write-pages":              {},
	"--cookies":                  {},
	"--cookies-from-browser":     {},
	"--ffmpeg-location":          {},
}

var safeModeBlockedCustomArgNames = map[string]struct{}{
	"--force-ipv4":         {},
	"--force-ipv6":         {},
	"--geo-bypass":         {},
	"--geo-bypass-country": {},
	"--max-sleep-interval": {},
	"--retry-sleep":        {},
	"--sleep-interval":     {},
	"--sleep-requests":     {},
}

type customArgSpec struct {
	requiresValue bool
	validator     func(string) error
}

var allowedCustomArgNames = map[string]customArgSpec{
	"--concurrent-fragments": {requiresValue: true, validator: validateSmallPositiveIntArg(1, 16)},
	"--convert-thumbnails":   {requiresValue: true, validator: validateEnumArg("jpg", "png", "webp")},
	"--embed-metadata":       {},
	"--embed-thumbnail":      {},
	"--extractor-retries":    {requiresValue: true, validator: validateSmallPositiveIntArg(0, 10)},
	"--force-ipv4":           {},
	"--force-ipv6":           {},
	"--fragment-retries":     {requiresValue: true, validator: validateSmallPositiveIntArg(0, 25)},
	"--geo-bypass":           {},
	"--geo-bypass-country":   {requiresValue: true, validator: validateCountryArg},
	"--max-sleep-interval":   {requiresValue: true, validator: validateSmallPositiveFloatArg(0, 60)},
	"--no-mtime":             {},
	"--no-part":              {},
	"--restrict-filenames":   {},
	"--retries":              {requiresValue: true, validator: validateSmallPositiveIntArg(0, 25)},
	"--retry-sleep":          {requiresValue: true, validator: validateRetrySleepArg},
	"--sleep-interval":       {requiresValue: true, validator: validateSmallPositiveFloatArg(0, 60)},
	"--sleep-requests":       {requiresValue: true, validator: validateSmallPositiveFloatArg(0, 10)},
	"--add-header":           {requiresValue: true, validator: validateHeaderArg},
	"--referer":              {requiresValue: true, validator: validateURLArg},
	"--user-agent":           {requiresValue: true, validator: validateUserAgentArg},
	"--sub-langs":            {requiresValue: true, validator: validateSubLangsArg},
	"--trim-filenames":       {requiresValue: true, validator: validateSmallPositiveIntArg(16, 255)},
	"--windows-filenames":    {},
	"--write-auto-subs":      {},
	"--write-subs":           {},
	"--write-thumbnail":      {},
}

func sanitizeCustomArgs(raw []string, safeModeEnabled bool) ([]string, error) {
	expectingValueFor := ""
	var expectingValidator func(string) error
	for _, arg := range raw {
		if arg == "" {
			continue
		}
		if expectingValueFor != "" {
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("custom argument %q requires a value", expectingValueFor)
			}
			if expectingValidator != nil {
				if err := expectingValidator(arg); err != nil {
					return nil, fmt.Errorf("custom argument %q has invalid value: %w", expectingValueFor, err)
				}
			}
			expectingValueFor = ""
			expectingValidator = nil
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("custom argument value %q has no option", arg)
		}
		name := arg
		value := ""
		if idx := strings.Index(name, "="); idx >= 0 {
			value = name[idx+1:]
			name = name[:idx]
		}
		if _, blocked := blockedCustomArgNames[name]; blocked {
			return nil, fmt.Errorf("custom argument %q is not allowed", name)
		}
		if safeModeEnabled {
			if _, blocked := safeModeBlockedCustomArgNames[name]; blocked {
				return nil, fmt.Errorf("custom argument %q is blocked by safe mode", name)
			}
		}
		spec, allowed := allowedCustomArgNames[name]
		if !allowed {
			return nil, fmt.Errorf("custom argument %q is not supported", name)
		}
		if spec.requiresValue && !strings.Contains(arg, "=") {
			expectingValueFor = name
			expectingValidator = spec.validator
			continue
		}
		if spec.requiresValue && spec.validator != nil {
			if err := spec.validator(value); err != nil {
				return nil, fmt.Errorf("custom argument %q has invalid value: %w", name, err)
			}
		}
		if !spec.requiresValue && strings.Contains(arg, "=") {
			return nil, fmt.Errorf("custom argument %q does not accept a value", name)
		}
	}
	if expectingValueFor != "" {
		return nil, fmt.Errorf("custom argument %q requires a value", expectingValueFor)
	}
	return raw, nil
}

func validateSmallPositiveIntArg(min, max int) func(string) error {
	return func(raw string) error {
		v, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || v < min || v > max {
			return fmt.Errorf("must be an integer from %d to %d", min, max)
		}
		return nil
	}
}

func validateSmallPositiveFloatArg(min, max float64) func(string) error {
	return func(raw string) error {
		v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil || v < min || v > max {
			return fmt.Errorf("must be a number from %.0f to %.0f", min, max)
		}
		return nil
	}
}

func validateEnumArg(allowed ...string) func(string) error {
	return func(raw string) error {
		for _, value := range allowed {
			if raw == value {
				return nil
			}
		}
		return fmt.Errorf("unsupported value")
	}
}

func validateCountryArg(raw string) error {
	if regexp.MustCompile(`^[A-Za-z]{2}$`).MatchString(raw) {
		return nil
	}
	return errors.New("must be a two-letter country code")
}

func validateSubLangsArg(raw string) error {
	if len(raw) > 128 {
		return errors.New("too long")
	}
	if regexp.MustCompile(`^[A-Za-z0-9_*,-]+$`).MatchString(raw) {
		return nil
	}
	return errors.New("contains unsupported characters")
}

func validateRetrySleepArg(raw string) error {
	if len(raw) > 64 {
		return errors.New("too long")
	}
	if regexp.MustCompile(`^((linear|exp)=)?[0-9]+(\.[0-9]+)?(:[0-9]+(\.[0-9]+)?)?$`).MatchString(raw) {
		return nil
	}
	return errors.New("unsupported retry sleep format")
}

func validateHeaderArg(raw string) error {
	if len(raw) > 512 {
		return errors.New("too long")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+:\s?.+$`).MatchString(raw) {
		return errors.New("must be in format HeaderName: value")
	}
	return nil
}

func validateURLArg(raw string) error {
	if len(raw) > 1024 {
		return errors.New("too long")
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return errors.New("must start with http:// or https://")
	}
	return nil
}

func validateUserAgentArg(raw string) error {
	if len(raw) > 256 {
		return errors.New("too long")
	}
	return nil
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

func (a *App) SelectCookieFile() (string, error) {
	title := "Select Cookies File"
	switch a.GetSettings().Language {
	case "de":
		title = "Cookie-Datei auswählen"
	case "fr":
		title = "Choisir le fichier de cookies"
	}
	file, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: title,
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "Text Files (*.txt)",
				Pattern:     "*.txt",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return file, nil
}

func (a *App) defaultCookieCachePath(browser string) string {
	browser = strings.ToLower(strings.TrimSpace(browser))
	if _, ok := browserProcessNames[browser]; !ok {
		browser = "browser"
	}
	baseDir := a.configDir
	if baseDir == "" {
		baseDir = filepath.Dir(a.settingsPath())
	}
	return filepath.Join(baseDir, "cookies", browser+".txt")
}

func isUsableCookieCache(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func (a *App) cookieCachePathForSettings(s Settings) string {
	browser := strings.ToLower(s.CookieBrowser)
	if s.CookieCachePath != "" && s.CookieCacheBrowser == browser {
		return s.CookieCachePath
	}
	return a.defaultCookieCachePath(browser)
}

func (a *App) BrowserCookieCacheAvailable(browser string) (bool, error) {
	browser = strings.ToLower(strings.TrimSpace(browser))
	if _, ok := browserProcessNames[browser]; !ok {
		return false, fmt.Errorf("unknown browser: %s", browser)
	}
	s := a.GetSettings()
	if s.CookieSource != "browser" || strings.ToLower(s.CookieBrowser) != browser {
		return false, nil
	}
	return isUsableCookieCache(a.cookieCachePathForSettings(s)), nil
}

func (a *App) persistBrowserCookieCache(s Settings) {
	if s.CookieSource != "browser" || s.CookieBrowser == "" {
		return
	}
	browser := strings.ToLower(s.CookieBrowser)
	path := a.cookieCachePathForSettings(s)
	if !isUsableCookieCache(path) {
		return
	}

	a.cookieCacheMu.Lock()
	defer a.cookieCacheMu.Unlock()
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	current := a.getSettingsLocked()
	if current.CookieSource != "browser" || strings.ToLower(current.CookieBrowser) != browser {
		return
	}
	current.CookieCachePath = path
	current.CookieCacheBrowser = browser
	current.CookieCacheUpdated = time.Now().UTC().Format(time.RFC3339)
	if err := a.writeSettingsLocked(current); err != nil {
		log.Printf("persist cookie cache: %v", err)
	}
}

func (a *App) getCookieArgs(s Settings) []string {
	var args []string
	if s.CookieSource == "browser" && s.CookieBrowser != "" {
		browser := strings.ToLower(s.CookieBrowser)
		cachePath := a.cookieCachePathForSettings(s)
		if s.CookieCacheBrowser == browser && isUsableCookieCache(cachePath) {
			return append(args, "--cookies", cachePath)
		}
		if err := os.MkdirAll(filepath.Dir(cachePath), privateDirMode); err != nil {
			log.Printf("create cookie cache dir: %v", err)
			return append(args, "--cookies-from-browser", browser)
		}
		return append(args, "--cookies-from-browser", browser, "--cookies", cachePath)
	} else if s.CookieSource == "file" && s.CookieFilePath != "" {
		args = append(args, "--cookies", s.CookieFilePath)
	}
	return args
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

// ---------- History ----------

func (a *App) portableHistoryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "history.json")
}

func (a *App) historyPath() string {
	if a.historyFilePath != "" {
		return a.historyFilePath
	}
	return a.resolveHistoryPath()
}

func (a *App) resolveHistoryPath() string {
	portable := a.portableHistoryPath()
	return resolveSettingsPathFor(a.configDir, portable, "history.json")
}

func (a *App) migrateHistoryIfNeeded() error {
	path := a.historyPath()
	data, err := readFileBounded(path, maxHistoryFileBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if data[0] != '[' {
		return nil
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return writeHistoryEntriesToFile(path, entries)
}

func readFileBounded(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", filepath.Base(path), maxBytes)
	}
	return data, nil
}

func readHistoryEntriesFromFile(path string) ([]HistoryEntry, error) {
	data, err := readFileBounded(path, maxHistoryFileBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries := make([]HistoryEntry, 0, maxHistoryEntries)
	next := 0
	wrapped := false
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var e HistoryEntry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode history: %w", err)
		}
		if len(entries) < maxHistoryEntries {
			entries = append(entries, e)
			continue
		}
		entries[next] = e
		next = (next + 1) % maxHistoryEntries
		wrapped = true
	}
	if wrapped {
		ordered := make([]HistoryEntry, 0, maxHistoryEntries)
		ordered = append(ordered, entries[next:]...)
		ordered = append(ordered, entries[:next]...)
		return ordered, nil
	}
	return entries, nil
}

func writeHistoryEntriesToFile(path string, entries []HistoryEntry) error {
	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".history-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	enc := json.NewEncoder(tmp)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := replaceFilePreservingOld(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (a *App) loadHistoryCache() error {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	return a.ensureHistoryLoadedLocked()
}

func (a *App) ensureHistoryLoadedLocked() error {
	if a.historyLoaded {
		return nil
	}
	entries, err := readHistoryEntriesFromFile(a.historyPath())
	if err != nil {
		return err
	}
	a.historyCache = entries
	a.historyLoaded = true
	return nil
}

func reverseHistoryEntries(entries []HistoryEntry) []HistoryEntry {
	out := make([]HistoryEntry, len(entries))
	for i := range entries {
		out[len(entries)-1-i] = entries[i]
	}
	return out
}

func (a *App) GetHistory() ([]HistoryEntryView, error) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := a.ensureHistoryLoadedLocked(); err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}
	reversed := reverseHistoryEntries(a.historyCache)
	out := make([]HistoryEntryView, 0, len(reversed))
	for _, entry := range reversed {
		out = append(out, historyEntryView(entry))
	}
	return out, nil
}

func historyEntryView(entry HistoryEntry) HistoryEntryView {
	return HistoryEntryView{
		DownloadID: entry.DownloadID,
		URL:        entry.URL,
		Title:      entry.Title,
		FormatID:   entry.FormatID,
		FileSize:   entry.FileSize,
		AvgSpeed:   entry.AvgSpeed,
		Status:     entry.Status,
		ErrorMsg:   entry.ErrorMsg,
		StartTime:  formatHistoryTime(entry.StartTime),
		EndTime:    formatHistoryTime(entry.EndTime),
		OutputPath: entry.OutputPath,
	}
}

func formatHistoryTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func (a *App) isKnownHistoryOutputFile(path string) bool {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := a.ensureHistoryLoadedLocked(); err != nil {
		return false
	}
	for _, entry := range a.historyCache {
		if entry.OutputPath == "" {
			continue
		}
		cleaned, err := cleanAbsolutePath(entry.OutputPath)
		if err == nil && samePath(path, cleaned) {
			return true
		}
	}
	return false
}

func (a *App) isKnownHistoryOutputDir(dir string) bool {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := a.ensureHistoryLoadedLocked(); err != nil {
		return false
	}
	for _, entry := range a.historyCache {
		if entry.OutputPath == "" {
			continue
		}
		cleaned, err := cleanAbsolutePath(entry.OutputPath)
		if err == nil && samePath(dir, filepath.Dir(cleaned)) {
			return true
		}
	}
	return false
}

func (a *App) saveHistoryEntry(entry HistoryEntry) error {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := a.ensureHistoryLoadedLocked(); err != nil {
		log.Printf("saveHistoryEntry load: %v", err)
		return err
	}
	next := append(append([]HistoryEntry(nil), a.historyCache...), entry)
	if len(next) > maxHistoryEntries {
		next = next[len(next)-maxHistoryEntries:]
	}
	if err := writeHistoryEntriesToFile(a.historyPath(), next); err != nil {
		log.Printf("saveHistoryEntry: %v", err)
		return err
	}
	a.historyCache = next
	return nil
}

func (a *App) ClearHistory() error {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := writeHistoryEntriesToFile(a.historyPath(), nil); err != nil {
		return fmt.Errorf("clear history: %w", err)
	}
	a.historyCache = nil
	a.historyLoaded = true
	return nil
}

func (a *App) DeleteHistoryEntry(downloadID string) error {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	if err := a.ensureHistoryLoadedLocked(); err != nil {
		return fmt.Errorf("load history: %w", err)
	}
	path := a.historyPath()
	filtered := make([]HistoryEntry, 0, len(a.historyCache))
	for _, e := range a.historyCache {
		if e.DownloadID != downloadID {
			filtered = append(filtered, e)
		}
	}
	if err := writeHistoryEntriesToFile(path, filtered); err != nil {
		return fmt.Errorf("delete history entry: %w", err)
	}
	a.historyCache = filtered
	return nil
}

// ---------- Dependency Management ----------

func (a *App) CheckDependencies() DependencyStatus {
	return DependencyStatus{
		YtDlpInstalled:  fileExists(a.ytdlpPath()),
		FfmpegInstalled: a.ffmpegToolsInstalled(),
	}
}

func (a *App) ffmpegToolsInstalled() bool {
	return fileExists(a.ffmpegPath()) && (runtime.GOOS != "windows" || fileExists(a.ffprobePath()))
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
			select {
			case <-time.After(2 * time.Second):
			case <-a.appContext().Done():
				return ""
			}
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
			select {
			case <-time.After(2 * time.Second):
			case <-a.appContext().Done():
				return ""
			}
		}
	}
	return ""
}

func (a *App) UpdateDependencies() error {
	a.adMu.Lock()
	hasActive := len(a.activeDownloads) > 0
	a.adMu.Unlock()
	if hasActive {
		return errors.New("cannot update dependencies while downloads are in progress")
	}

	a.dependencyMu.Lock()
	defer a.dependencyMu.Unlock()
	if err := a.downloadYtdlp(true); err != nil {
		return err
	}
	s := a.GetSettings()
	if s.FfmpegPath == "" {
		return a.downloadFfmpeg(true)
	}
	return nil
}

func (a *App) OpenBinDir() error {
	return a.OpenOutputDir(a.binDir)
}

func (a *App) OpenOutputDir(dir string) error {
	if dir == "" {
		settings := a.GetSettings()
		dir = settings.DefaultOutputDir
		if dir == "" {
			dir = defaultOutputDir()
		}
	}
	cleaned, err := cleanAbsolutePath(dir)
	if err != nil {
		return err
	}
	if !a.isAllowedOpenDir(cleaned) {
		return errors.New("directory is outside KoalaPull-managed paths")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = command("open", cleaned)
	case "windows":
		// Do not use the command() helper here; explorer needs to show its GUI window and will fail/exit with code 1 if run with CREATE_NO_WINDOW/HideWindow attributes.
		cmd = exec.Command("explorer", cleaned)
	default:
		cmd = command("xdg-open", cleaned)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("OpenOutputDir wait: %v", err)
		}
	}()
	return nil
}

func (a *App) isAllowedOpenDir(dir string) bool {
	settings := a.GetSettings()
	defaultDir, err := cleanAbsolutePath(settings.DefaultOutputDir)
	if err == nil && samePath(dir, defaultDir) {
		return true
	}
	binDir, err := cleanAbsolutePath(a.binDir)
	if err == nil && samePath(dir, binDir) {
		return true
	}
	return a.isKnownHistoryOutputDir(dir)
}

func (a *App) GetVersionInfo() VersionInfo {
	vi := VersionInfo{App: AppVersion}
	ytdlpCh := make(chan string, 1)
	ffmpegCh := make(chan string, 1)
	go func() {
		ytdlpCh <- a.GetYtdlpVersion()
	}()
	go func() {
		ffmpegCh <- a.GetFfmpegVersion()
	}()
	vi.Ytdlp = <-ytdlpCh
	vi.Ffmpeg = <-ffmpegCh
	return vi
}

func (a *App) GetAppVersion() string {
	return AppVersion
}

func (a *App) CheckForUpdates() UpdateInfo {
	info := UpdateInfo{}
	if latest, err := fetchLatestYtdlpVersion(a.appContext()); err == nil && latest != "" {
		info.LatestYtdlpVersion = latest
		if isVersionNewer(latest, a.GetYtdlpVersion()) {
			info.YtdlpUpdateAvailable = true
		}
	}
	if AppVersion != "" && AppVersion != "dev" && !strings.Contains(AppVersion, "-") {
		if latest, err := fetchLatestKoalaPullVersion(a.appContext()); err == nil && latest != "" {
			info.LatestKoalaPullVersion = latest
			if isVersionNewer(latest, AppVersion) {
				info.KoalaPullUpdateAvailable = true
			}
		}
	}
	s := a.GetSettings()
	if s.FfmpegPath == "" {
		info.LatestFfmpegVersion = a.GetFfmpegVersion()
		if latest, err := fetchLatestFfmpegBuildDate(a.appContext()); err == nil {
			if fi, e := os.Stat(a.ffmpegPath()); e == nil {
				if latest.After(fi.ModTime()) {
					info.FfmpegUpdateAvailable = true
				}
			}
		}
	}
	return info
}

func isVersionNewer(latest, current string) bool {
	cmp, ok := compareNumericVersions(latest, current)
	return ok && cmp > 0
}

func compareNumericVersions(left, right string) (int, bool) {
	parse := func(raw string) ([]int, bool) {
		raw = strings.TrimPrefix(strings.TrimSpace(raw), "v")
		if suffix := strings.IndexAny(raw, "-+"); suffix >= 0 {
			raw = raw[:suffix]
		}
		if raw == "" {
			return nil, false
		}
		parts := strings.Split(raw, ".")
		values := make([]int, len(parts))
		for i, part := range parts {
			if part == "" {
				return nil, false
			}
			value, err := strconv.Atoi(part)
			if err != nil || value < 0 {
				return nil, false
			}
			values[i] = value
		}
		return values, true
	}

	leftParts, leftOK := parse(left)
	rightParts, rightOK := parse(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	length := len(leftParts)
	if len(rightParts) > length {
		length = len(rightParts)
	}
	for i := 0; i < length; i++ {
		leftValue := 0
		if i < len(leftParts) {
			leftValue = leftParts[i]
		}
		rightValue := 0
		if i < len(rightParts) {
			rightValue = rightParts[i]
		}
		if leftValue < rightValue {
			return -1, true
		}
		if leftValue > rightValue {
			return 1, true
		}
	}
	return 0, true
}

func fetchLatestYtdlpVersion(ctx context.Context) (string, error) {
	dlCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "KoalaPull/"+AppVersion)
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
	if err := decodeJSONResponseLimited(resp, &result, maxUpdateResponseBytes); err != nil {
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
	req.Header.Set("User-Agent", "KoalaPull/"+AppVersion)
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
	if err := decodeJSONResponseLimited(resp, &result, maxUpdateResponseBytes); err != nil {
		return "", err
	}
	return result.TagName, nil
}

func fetchLatestFfmpegBuildDate(ctx context.Context) (time.Time, error) {
	dlCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, "https://api.github.com/repos/BtbN/FFmpeg-Builds/releases/latest", nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "KoalaPull/"+AppVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var result struct {
		PublishedAt string `json:"published_at"`
	}
	if err := decodeJSONResponseLimited(resp, &result, maxUpdateResponseBytes); err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, result.PublishedAt)
}

func decodeJSONResponseLimited(resp *http.Response, dest any, maxBytes int64) error {
	if resp.ContentLength > maxBytes {
		return fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	return json.Unmarshal(data, dest)
}

func (a *App) SelectFfmpegPath() (string, error) {
	title := "Select FFmpeg Executable"
	file, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: title,
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "FFmpeg Executable (ffmpeg*)",
				Pattern:     "ffmpeg*",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return file, nil
}

func (a *App) OpenExternalLink(url string) error {
	if !isAllowedDownloadURL(url) {
		return errors.New("external link must use http or https")
	}
	if !isAllowedExternalLinkHost(url) {
		return errors.New("external link host is not allowed")
	}
	wailsRuntime.BrowserOpenURL(a.appContext(), url)
	return nil
}

func (a *App) emitProgress(dep string, pct int, status, errMsg string) {
	if isTesting {
		return
	}
	defer func() {
		_ = recover()
	}()
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

func (a *App) emitDownloadProgress(downloadID string, percent float64, speed, eta, fileSize, status, errMsg, playlistStatus, outputPath, title string) {
	if isTesting {
		return
	}
	defer func() {
		_ = recover()
	}()
	ev := DownloadProgress{
		DownloadID:     downloadID,
		Percent:        percent,
		Speed:          speed,
		ETA:            eta,
		FileSize:       fileSize,
		Status:         status,
		PlaylistStatus: playlistStatus,
		OutputPath:     outputPath,
		Title:          title,
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

func createRollbackBackup(path, tmpDir string) (string, error) {
	if !fileExists(path) {
		return "", nil
	}
	backupPath := filepath.Join(tmpDir, filepath.Base(path)+".bak")
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return "", err
	}
	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode())
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		_ = os.Remove(backupPath)
		return "", err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		_ = os.Remove(backupPath)
		return "", err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(backupPath)
		return "", err
	}
	return backupPath, nil
}

func restoreRollbackBackup(backupPath, destPath string) error {
	if backupPath == "" {
		return nil
	}
	return replaceFilePreservingOld(backupPath, destPath)
}

func verifyManagedBinary(ctx context.Context, path string, args ...string) error {
	if !fileExists(path) {
		return errors.New("binary missing after install")
	}
	_, err := commandOutputLimited(ctx, maxCommandOutputBytes, path, args...)
	return err
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
	tmpDir, err := os.MkdirTemp(a.binDir, ".koalapull-ytdlp-rollback-*")
	if err != nil {
		return fmt.Errorf("rollback temp dir failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	if err := a.downloadFile(dlCtx, url, tmpPath, "yt-dlp", maxYtdlpDownloadBytes); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := verifyYtdlpChecksum(dlCtx, tmpPath, filepath.Base(url)); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, privateDirMode); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("chmod failed: %w", err)
		}
	}
	backupPath, err := createRollbackBackup(destPath, tmpDir)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup failed: %w", err)
	}
	if err := replaceFilePreservingOld(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace failed: %w", err)
	}
	if runtime.GOOS == "darwin" {
		if err := command("xattr", "-d", "com.apple.quarantine", destPath).Run(); err != nil {
			log.Printf("xattr warning: %v", err)
		}
	}
	verifyCtx, verifyCancel := context.WithTimeout(a.appContext(), 10*time.Second)
	defer verifyCancel()
	if err := verifyManagedBinary(verifyCtx, destPath, "--version"); err != nil {
		restoreErr := restoreRollbackBackup(backupPath, destPath)
		return errors.Join(fmt.Errorf("yt-dlp verification failed: %w", err), restoreErr)
	}
	a.emitProgress("yt-dlp", 100, "completed", "")
	return nil
}

func (a *App) downloadFfmpeg(force bool) error {
	a.emitProgress("ffmpeg", 0, "downloading", "")
	destPath := a.ffmpegPath()
	if !force && a.ffmpegToolsInstalled() {
		a.emitProgress("ffmpeg", 100, "completed", "")
		return nil
	}
	artifact := ffmpegArtifactFor(runtime.GOOS)
	tmpDir, err := os.MkdirTemp(a.binDir, ".koalapull-ffmpeg-*")
	if err != nil {
		return fmt.Errorf("temp dir failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	archivePath := filepath.Join(tmpDir, "ffmpeg-archive")
	extractedPath := filepath.Join(tmpDir, "ffmpeg-extracted")
	ffmpegBackupPath, err := createRollbackBackup(destPath, tmpDir)
	if err != nil {
		return fmt.Errorf("backup ffmpeg failed: %w", err)
	}
	ffprobeBackupPath, err := createRollbackBackup(a.ffprobePath(), tmpDir)
	if err != nil {
		return fmt.Errorf("backup ffprobe failed: %w", err)
	}
	dlCtx, cancel := context.WithTimeout(a.appContext(), 10*time.Minute)
	defer cancel()
	if err := a.downloadFile(dlCtx, artifact.URL, archivePath, "ffmpeg", artifact.MaxBytes); err != nil {
		return err
	}
	if err := verifyFFmpegArchive(dlCtx, archivePath, artifact); err != nil {
		return err
	}
	if err := extractFFmpegArchive(dlCtx, archivePath, extractedPath); err != nil {
		return fmt.Errorf("ffmpeg extraction failed: %w", err)
	}
	if !fileExists(extractedPath) {
		return fmt.Errorf("ffmpeg binary not found after extraction")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(extractedPath, privateDirMode); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	} else {
		ffprobePath := filepath.Join(tmpDir, "ffprobe-extracted.exe")
		if err := extractZipBinaryBounded(dlCtx, archivePath, ffprobePath, "ffprobe.exe"); err != nil {
			return fmt.Errorf("ffprobe extraction failed: %w", err)
		}
		if err := replaceFilePreservingOld(ffprobePath, a.ffprobePath()); err != nil {
			return fmt.Errorf("replace ffprobe: %w", err)
		}
	}
	if runtime.GOOS == "darwin" && artifact.FFprobeURL != "" {
		ffprobeArchivePath := filepath.Join(tmpDir, "ffprobe-archive")
		ffprobeExtractedPath := filepath.Join(tmpDir, "ffprobe-extracted")
		if err := a.downloadFile(dlCtx, artifact.FFprobeURL, ffprobeArchivePath, "ffprobe", artifact.MaxBytes); err != nil {
			return fmt.Errorf("ffprobe download failed: %w", err)
		}
		if err := verifyFileChecksumFromURL(dlCtx, ffprobeArchivePath, artifact.FFprobeChecksumURL, artifact.FFprobeAssetName, "ffprobe"); err != nil {
			return err
		}
		if err := extractZipBinaryBounded(dlCtx, ffprobeArchivePath, ffprobeExtractedPath, "ffprobe"); err != nil {
			return fmt.Errorf("ffprobe extraction failed: %w", err)
		}
		if err := os.Chmod(ffprobeExtractedPath, privateDirMode); err != nil {
			return fmt.Errorf("chmod ffprobe failed: %w", err)
		}
		if err := replaceFilePreservingOld(ffprobeExtractedPath, a.ffprobePath()); err != nil {
			return fmt.Errorf("replace ffprobe: %w", err)
		}
	}
	if err := replaceFilePreservingOld(extractedPath, destPath); err != nil {
		return fmt.Errorf("replace ffmpeg: %w", err)
	}
	verifyCtx, verifyCancel := context.WithTimeout(a.appContext(), 10*time.Second)
	defer verifyCancel()
	verifyErr := verifyManagedBinary(verifyCtx, destPath, "-version")
	if verifyErr == nil && runtime.GOOS == "windows" {
		verifyErr = verifyManagedBinary(verifyCtx, a.ffprobePath(), "-version")
	}
	if verifyErr == nil && runtime.GOOS == "darwin" && artifact.FFprobeURL != "" {
		verifyErr = verifyManagedBinary(verifyCtx, a.ffprobePath(), "-version")
	}
	if verifyErr != nil {
		restoreErr := errors.Join(
			restoreRollbackBackup(ffmpegBackupPath, destPath),
			restoreRollbackBackup(ffprobeBackupPath, a.ffprobePath()),
		)
		return errors.Join(fmt.Errorf("ffmpeg verification failed: %w", verifyErr), restoreErr)
	}
	a.emitProgress("ffmpeg", 100, "completed", "")
	return nil
}

func (a *App) downloadFile(ctx context.Context, url, destPath, depName string, maxBytes int64) (retErr error) {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, privateFileMode)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer func() {
		if out != nil {
			retErr = errors.Join(retErr, out.Close())
		}
		if retErr != nil {
			_ = os.Remove(destPath)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	if !isTesting && !isTrustedDependencyURL(req.URL) {
		return fmt.Errorf("untrusted dependency host: %s", req.URL.Hostname())
	}
	resp, err := trustedDependencyHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	total := resp.ContentLength
	if total > maxBytes {
		return fmt.Errorf("download exceeds %d bytes", maxBytes)
	}
	lastPct := -1
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if written+int64(n) > maxBytes {
				return fmt.Errorf("download exceeds %d bytes", maxBytes)
			}
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
	return verifyFileChecksum(filePath, expected, "yt-dlp")
}

func verifyFileChecksumFromURL(ctx context.Context, filePath, checksumURL, assetName, label string) error {
	if checksumURL == "" {
		return fmt.Errorf("%s artifact has no checksum URL", label)
	}
	expected, err := fetchChecksumForAsset(ctx, checksumURL, assetName)
	if err != nil {
		return fmt.Errorf("fetch %s checksum: %w", label, err)
	}
	return verifyFileChecksum(filePath, expected, label)
}

func fetchChecksumForAsset(ctx context.Context, checksumURL, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	if !isTesting && !isTrustedDependencyURL(req.URL) {
		return "", fmt.Errorf("untrusted checksum host: %s", req.URL.Hostname())
	}
	resp, err := trustedDependencyHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad checksum status: %s", resp.Status)
	}
	if resp.ContentLength > maxChecksumDownloadBytes {
		return "", fmt.Errorf("checksum response exceeds %d bytes", maxChecksumDownloadBytes)
	}
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxChecksumDownloadBytes+1))
	scanner.Buffer(make([]byte, 0, 64*1024), int(maxChecksumDownloadBytes))
	firstValidChecksum := ""
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 1 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) == sha256.Size*2 {
			if _, err := hex.DecodeString(sum); err == nil && firstValidChecksum == "" {
				firstValidChecksum = sum
			}
		}
		if assetName == "" {
			continue
		}
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "./")
		if filepath.Base(name) == assetName {
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
	if assetName == "" && firstValidChecksum != "" {
		return firstValidChecksum, nil
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

func validateArchiveMemberName(name string) error {
	cleaned := filepath.Clean(name)
	if name == "" || filepath.IsAbs(name) || strings.HasPrefix(cleaned, "..") || filepath.VolumeName(name) != "" {
		return fmt.Errorf("unsafe archive path: %q", name)
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
	if err := validateDownloadURLForLaunch(ctxWithDefault(a.appContext()), url); err != nil {
		return nil, err
	}
	args := []string{"--no-check-formats", "--no-warnings", "--dump-json", "--skip-download", "--flat-playlist"}
	settings := a.GetSettings()
	args = append(args, a.getCookieArgs(settings)...)
	if settings.CustomArgs != "" {
		customArgs, err := sanitizeCustomArgs(parseCustomArgs(settings.CustomArgs), settings.SafeModeEnabled)
		if err != nil {
			return nil, err
		}
		args = append(args, customArgs...)
	}
	args = append(args, "--", url)
	ctx, cancel := context.WithTimeout(a.appContext(), 30*time.Second)
	defer cancel()
	stdout, err := commandOutputLimited(ctx, maxMetadataOutputBytes, a.ytdlpPath(), args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("yt-dlp metadata timed out: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("yt-dlp exec failed: %w", err)
	}
	a.persistBrowserCookieCache(settings)
	var raw rawMetadata
	if err := json.Unmarshal(stdout, &raw); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	if raw.Type == "playlist" {
		entries := make([]PlaylistEntry, 0, len(raw.Entries))
		for _, e := range raw.Entries {
			entries = append(entries, PlaylistEntry{
				ID:    e.ID,
				Title: e.Title,
			})
		}
		meta := &VideoMetadata{
			ID:         raw.ID,
			Title:      raw.Title,
			Thumbnail:  sanitizeRemoteMediaURLWithResolver(ctx, raw.Thumbnail),
			Uploader:   raw.Uploader,
			IsPlaylist: true,
			EntryCount: len(raw.Entries),
			Formats:    make([]FormatInfo, 0),
			Entries:    entries,
		}
		return meta, nil
	}

	meta := &VideoMetadata{
		ID:        raw.ID,
		Title:     raw.Title,
		Thumbnail: sanitizeRemoteMediaURLWithResolver(ctx, raw.Thumbnail),
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
const maxQueueLimit = 100

func (a *App) StartDownload(url, formatID, outputDir, container, subtitle, title string) (string, error) {
	return a.StartDownloadWithPreset(url, formatID, outputDir, container, subtitle, title, defaultDownloadPreset, "")
}

func (a *App) StartDownloadWithPreset(url, formatID, outputDir, container, subtitle, title, preset, playlistItems string) (string, error) {
	if url == "" || formatID == "" {
		return "", fmt.Errorf("url and formatID are required")
	}
	if !isAllowedDownloadURL(url) {
		return "", fmt.Errorf("url must use http or https")
	}
	if err := validateDownloadURLForLaunch(ctxWithDefault(a.appContext()), url); err != nil {
		return "", err
	}
	if len(url) > maxInputLength || len(outputDir) > maxPathLength {
		return "", fmt.Errorf("input too long")
	}
	outputDir, err := a.normalizeOutputDir(outputDir)
	if err != nil {
		return "", err
	}
	if len(title) > 1024 {
		title = truncateToValidUTF8Prefix(title, 1024)
	}
	if err := os.MkdirAll(outputDir, outputDirMode); err != nil {
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
	settings := a.GetSettings()
	args = append(args, a.getCookieArgs(settings)...)
	if settings.SponsorBlockEnabled {
		args = append(args, "--sponsorblock-remove", "all")
	}
	if settings.RateLimitEnabled {
		limitBytes := parseRateLimitToBytes(settings.RateLimitValue)
		if limitBytes > 0 {
			args = append(args, "--limit-rate", strconv.FormatInt(limitBytes, 10))
		}
	}
	if settings.CustomArgs != "" {
		customArgs, err := sanitizeCustomArgs(parseCustomArgs(settings.CustomArgs), settings.SafeModeEnabled)
		if err != nil {
			return "", err
		}
		args = append(args, customArgs...)
	}
	if playlistItems != "" {
		if err := validatePlaylistItems(playlistItems); err != nil {
			return "", err
		}
		args = append(args, "--playlist-items", playlistItems)
	}
	args = append(args, downloadPostProcessingArgs(preset, container, subtitle)...)
	args = append(args, "--", url)

	ctx, cancel := context.WithCancel(a.appContext())
	a.adMu.Lock()
	if len(a.activeDownloads) >= maxQueueLimit {
		a.adMu.Unlock()
		cancel()
		return "", fmt.Errorf("download queue limit reached (%d)", maxQueueLimit)
	}
	a.activeDownloads[downloadID] = &activeDownload{cancel: cancel}
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
		isAudio := false
		switch container {
		case "mp3", "aac", "m4a", "opus", "flac", "wav":
			isAudio = true
		}
		if isAudio {
			args = append(args, "-x", "--audio-format", container)
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

func cleanAbsolutePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is empty")
	}
	if len(path) > maxPathLength {
		return "", errors.New("path too long")
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", err
		}
		cleaned = abs
	}
	return cleaned, nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func isWithinPath(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if samePath(path, root) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isFilesystemRoot(path string) bool {
	cleaned := filepath.Clean(path)
	parent := filepath.Dir(cleaned)
	return samePath(parent, cleaned)
}

func (a *App) normalizeOutputDir(outputDir string) (string, error) {
	settings := a.GetSettings()
	if outputDir == "" {
		outputDir = settings.DefaultOutputDir
	}
	cleaned, err := cleanAbsolutePath(outputDir)
	if err != nil {
		return "", fmt.Errorf("output dir: %w", err)
	}
	if isFilesystemRoot(cleaned) {
		return "", errors.New("output dir cannot be a filesystem root")
	}
	defaultDir, err := cleanAbsolutePath(settings.DefaultOutputDir)
	if err != nil {
		return "", fmt.Errorf("default output dir: %w", err)
	}
	if !samePath(cleaned, defaultDir) {
		return "", errors.New("output dir must match the configured download directory")
	}
	return cleaned, nil
}

func isAllowedDownloadURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isAllowedRemoteIP(ip) {
			return false
		}
	}
	return true
}

func ctxWithDefault(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func validateDownloadURLForLaunch(ctx context.Context, raw string) error {
	if isTesting {
		return nil
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve url host: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("url host has no addresses")
	}
	for _, ip := range ips {
		if !isAllowedRemoteIP(ip) {
			return errors.New("url host resolves to a blocked network address")
		}
	}
	return nil
}

var allowedSourceHosts = []string{
	"youtube.com", "youtu.be", "vimeo.com", "dailymotion.com", "twitch.tv", "tiktok.com", "x.com", "twitter.com",
	"instagram.com", "facebook.com", "reddit.com", "ardmediathek.de", "zdf.de", "arte.tv", "3sat.de", "ndr.de",
	"bbc.com", "ted.com", "cnn.com", "discovery.com", "bilibili.com", "nicovideo.jp", "rumble.com", "odysee.com",
	"soundcloud.com", "bandcamp.com",
}

func isAllowedSourceHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, allowed := range allowedSourceHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func isAllowedRemoteIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}

func isAllowedExternalLinkHost(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	switch host {
	case "github.com", "www.github.com", "yt-dlp.org", "www.yt-dlp.org", "ffmpeg.org", "www.ffmpeg.org", "evermeet.cx", "www.evermeet.cx":
		return true
	}
	return strings.HasSuffix(host, ".github.com")
}

func sanitizeRemoteMediaURL(raw string) string {
	if isAllowedDownloadURL(raw) {
		return raw
	}
	return ""
}

func sanitizeRemoteMediaURLWithResolver(ctx context.Context, raw string) string {
	if !isAllowedDownloadURL(raw) {
		return ""
	}
	if err := validateDownloadURLForLaunch(ctxWithDefault(ctx), raw); err != nil {
		return ""
	}
	return raw
}

func validatePlaylistItems(raw string) error {
	items := strings.Split(raw, ",")
	if len(items) > maxBatchItems {
		return fmt.Errorf("playlist item limit is %d", maxBatchItems)
	}
	rangeRe := regexp.MustCompile(`^\d+(-\d+)?$`)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || !rangeRe.MatchString(item) {
			return fmt.Errorf("invalid playlist item selection")
		}
		if strings.Contains(item, "-") {
			parts := strings.SplitN(item, "-", 2)
			start, _ := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			if start < 1 || end < start || end-start+1 > maxBatchItems {
				return fmt.Errorf("playlist item limit is %d", maxBatchItems)
			}
			continue
		}
		n, _ := strconv.Atoi(item)
		if n < 1 {
			return fmt.Errorf("invalid playlist item selection")
		}
	}
	return nil
}

func (a *App) CancelDownload(downloadID string) {
	a.adMu.Lock()
	active, ok := a.activeDownloads[downloadID]
	a.adMu.Unlock()
	if ok {
		active.cancel()
	}
}

func (a *App) PauseDownload(downloadID string) error {
	a.adMu.Lock()
	active, ok := a.activeDownloads[downloadID]
	a.adMu.Unlock()
	if !ok {
		return errors.New("download not found")
	}
	if active.process == nil {
		return errors.New("download process is not ready yet")
	}
	if active.paused {
		return nil
	}
	if err := suspendProcess(active.process); err != nil {
		return err
	}
	a.adMu.Lock()
	if current, exists := a.activeDownloads[downloadID]; exists && current == active {
		current.paused = true
	}
	a.adMu.Unlock()
	return nil
}

func (a *App) ResumeDownload(downloadID string) error {
	a.adMu.Lock()
	active, ok := a.activeDownloads[downloadID]
	a.adMu.Unlock()
	if !ok {
		return errors.New("download not found")
	}
	if active.process == nil {
		return errors.New("download process is not ready yet")
	}
	if !active.paused {
		return nil
	}
	if err := resumeProcess(active.process); err != nil {
		return err
	}
	a.adMu.Lock()
	if current, exists := a.activeDownloads[downloadID]; exists && current == active {
		current.paused = false
	}
	a.adMu.Unlock()
	return nil
}

func (a *App) runDownload(ctx context.Context, cancel context.CancelFunc, downloadID string, args []string, title, url, formatID string) {
	defer func() {
		cancel()
		if r := recover(); r != nil {
			log.Printf("runDownload panic: %v", r)
			debug.PrintStack()
		}
		a.adMu.Lock()
		delete(a.activeDownloads, downloadID)
		a.adMu.Unlock()
	}()

	if !a.semAcquire(ctx) {
		a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "", "", title)
		return
	}
	defer a.semRelease()

	startTime := time.Now()
	a.emitDownloadProgress(downloadID, 0, "", "", "", "starting", "", "", "", title)

	var lastProgress atomic.Value
	lastProgress.Store(time.Now())

	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				_ = a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "cancelled", StartTime: startTime, EndTime: time.Now()})
				a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "", "", title)
				return
			}
			a.emitDownloadProgress(downloadID, 0, "", "", "", "retrying", "", "", "", title)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				_ = a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "cancelled", StartTime: startTime, EndTime: time.Now()})
				a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "", "", title)
				return
			}
			lastProgress.Store(time.Now())
		}

		attemptCtx, attemptCancel := context.WithCancel(ctx)
		var idleTimedOut atomic.Bool

		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					val := lastProgress.Load()
					if lastTime, ok := val.(time.Time); ok {
						if time.Since(lastTime) > 5*time.Minute {
							idleTimedOut.Store(true)
							attemptCancel()
							return
						}
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
				a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: fmt.Sprintf("stderr pipe: %v", err), StartTime: startTime, EndTime: time.Now()})
				a.emitDownloadProgress(downloadID, 0, "", "", "", "error", fmt.Sprintf("stderr pipe: %v", err), "", "", title)
				return
			}

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: fmt.Sprintf("stdout pipe: %v", err), StartTime: startTime, EndTime: time.Now()})
				a.emitDownloadProgress(downloadID, 0, "", "", "", "error", fmt.Sprintf("stdout pipe: %v", err), "", "", title)
				return
			}

			cleanup, err := startCommand(attemptCtx, cmd)
			if err != nil {
				if ctx.Err() == context.Canceled {
					a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "cancelled", StartTime: startTime, EndTime: time.Now()})
					a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "", "", title)
					return
				}
				a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: fmt.Sprintf("start command: %v", err), StartTime: startTime, EndTime: time.Now()})
				a.emitDownloadProgress(downloadID, 0, "", "", "", "error", fmt.Sprintf("start command: %v", err), "", "", title)
				return
			}
			defer cleanup()
			a.adMu.Lock()
			if active, ok := a.activeDownloads[downloadID]; ok {
				active.process = cmd.Process
				active.paused = false
			}
			a.adMu.Unlock()

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
						log.Printf("scanPipe panic: %v", r)
					}
				}()
				scanner := bufio.NewScanner(r)
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				for scanner.Scan() {
					lineCh <- downloadLine{source: source, line: scanner.Text()}
				}
				if scanErr := scanner.Err(); scanErr != nil {
					attemptCancel()
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
			detectedOutputPath := ""
			errLines := make([]string, 0, maxErrLines)
			var stdoutScanErr error
			var stderrScanErr error
			var lastEmitTime time.Time
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
				lastProgress.Store(time.Now())
				if item.source == "stderr" {
					if len(errLines) >= maxErrLines {
						errLines = append(errLines[1:], line)
					} else {
						errLines = append(errLines, line)
					}
				} else {
					if path := parseDestinationPath(line); path != "" {
						detectedOutputPath = path
						if clean := cleanTitleFromPath(path); clean != "" {
							title = clean
						}
					}
				}
				now := time.Now()
				if matches := sizeLineRegex.FindStringSubmatch(line); matches != nil {
					lastFileSz = matches[1]
				}
				if pct, fileSz, speed, eta, ok := parseProgressLine(line); ok {
					lastPct, lastSpeed, lastETA, lastFileSz = pct, speed, eta, fileSz
					if now.Sub(lastEmitTime) > 150*time.Millisecond || pct == 100 {
						lastEmitTime = now
						a.emitDownloadProgress(downloadID, pct, speed, eta, fileSz, "downloading", "", currentPS, detectedOutputPath, title)
					}
				} else if ps := parsePlaylistStatus(line); ps != "" {
					currentPS = ps
					if now.Sub(lastEmitTime) > 150*time.Millisecond {
						lastEmitTime = now
						a.emitDownloadProgress(downloadID, lastPct, lastSpeed, lastETA, lastFileSz, "downloading", "", currentPS, detectedOutputPath, title)
					}
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
					a.emitDownloadProgress(downloadID, 0, "", "", "", "cancelled", "", "", "", title)
					return
				}
				suppress, normalizedErr := normalizeDownloadAttemptError(attempt, idleTimedOut.Load(), err)
				if suppress {
					return
				}
				errMsg := strings.Join(errLines, "\n")
				if errMsg == "" || idleTimedOut.Load() {
					errMsg = normalizedErr.Error()
				}
				a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, Status: "error", ErrorMsg: errMsg, StartTime: startTime, EndTime: endTime})
				a.emitDownloadProgress(downloadID, 0, "", "", "", "error", errMsg, "", "", title)
				return
			}

			var errText string
			if historyErr := a.saveHistoryEntry(HistoryEntry{DownloadID: downloadID, URL: url, Title: title, FormatID: formatID, FileSize: lastFileSz, AvgSpeed: lastSpeed, Status: "completed", StartTime: startTime, EndTime: endTime, OutputPath: detectedOutputPath}); historyErr != nil {
				errText = fmt.Sprintf("History error: %v", historyErr)
			}
			a.persistBrowserCookieCache(a.GetSettings())
			a.emitDownloadProgress(downloadID, 100, "", "", lastFileSz, "completed", errText, "", detectedOutputPath, title)
		}()

		attemptCancel()

		// Only retry if attempt was cancelled by idle timeout (not user cancel)
		if shouldRetryDownloadAttempt(attempt, idleTimedOut.Load(), ctx.Err()) {
			continue
		}
		return
	}
}

func shouldRetryDownloadAttempt(attempt int, idleTimedOut bool, parentErr error) bool {
	return attempt == 0 && idleTimedOut && parentErr == nil
}

func normalizeDownloadAttemptError(attempt int, idleTimedOut bool, err error) (suppress bool, normalized error) {
	if !idleTimedOut {
		return false, err
	}
	if attempt == 0 {
		return true, err
	}
	return false, fmt.Errorf("download stalled for more than 5 minutes: %w", err)
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

func (a *App) PlayFile(filePath string) error {
	if filePath == "" {
		return errors.New("file path is empty")
	}
	cleaned, err := cleanAbsolutePath(filePath)
	if err != nil {
		return err
	}
	if !a.isAllowedOutputFile(cleaned) {
		return errors.New("file is outside KoalaPull-managed output paths")
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return fmt.Errorf("file error: %w", err)
	}
	if info.IsDir() {
		return errors.New("path is a directory, not a file")
	}
	if !isSafePlayableFile(cleaned) {
		return errors.New("file type is not safe to play from KoalaPull")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		return openFileWithDefaultApp(cleaned)
	case "darwin":
		cmd = command("open", cleaned)
	default:
		cmd = command("xdg-open", cleaned)
	}
	return cmd.Start()
}

var playableFileExtensions = map[string]struct{}{
	".3gp":  {},
	".aac":  {},
	".flac": {},
	".m4a":  {},
	".m4v":  {},
	".mkv":  {},
	".mov":  {},
	".mp3":  {},
	".mp4":  {},
	".oga":  {},
	".ogg":  {},
	".opus": {},
	".wav":  {},
	".webm": {},
}

func isSafePlayableFile(path string) bool {
	_, ok := playableFileExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func (a *App) ShowFileInFolder(filePath string) error {
	if filePath == "" {
		return errors.New("file path is empty")
	}
	cleaned, err := cleanAbsolutePath(filePath)
	if err != nil {
		return err
	}
	if !a.isAllowedOutputFile(cleaned) {
		return errors.New("file is outside KoalaPull-managed output paths")
	}
	_, err = os.Stat(cleaned)
	if err != nil {
		return fmt.Errorf("file error: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = command("explorer.exe", "/select,", cleaned)
	case "darwin":
		cmd = command("open", "-R", cleaned)
	default:
		cmd = command("xdg-open", filepath.Dir(cleaned))
	}
	return cmd.Start()
}

func (a *App) isAllowedOutputFile(path string) bool {
	settings := a.GetSettings()
	defaultDir, err := cleanAbsolutePath(settings.DefaultOutputDir)
	if err == nil && isWithinPath(path, defaultDir) {
		return true
	}
	return a.isKnownHistoryOutputFile(path)
}

var ytIDSuffixRegex = regexp.MustCompile(`\s+\[[A-Za-z0-9_-]{11}\]$`)

func cleanTitleFromPath(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = ytIDSuffixRegex.ReplaceAllString(name, "")
	return name
}
