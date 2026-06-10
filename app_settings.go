package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) resolveSettingsPath() string {
	if fileExists(a.portableSettingsPath()) {
		return a.portableSettingsPath()
	}
	return filepath.Join(a.configDir, "settings.json")
}

func (a *App) loadSettings() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	a.cachedSettings = validateSettings(Settings{})
	path := a.settingsPath()
	if !fileExists(path) {
		_ = a.writeSettingsLocked(a.cachedSettings)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("read settings error: %v", err)
		return
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("parse settings error: %v", err)
		return
	}
	a.cachedSettings = validateSettings(s)
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
			for i := 0; i < s.MaxConcurrency-oldLimit; i++ {
				select {
				case a.semWake <- struct{}{}:
				default:
				}
			}
		}
	}
	return nil
}

func (a *App) SelectDirectory() (string, error) {
	s := a.GetSettings()
	title := "Select Download Directory"
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		DefaultDirectory: s.DefaultOutputDir,
		Title:            title,
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (a *App) SelectCookieFile() (string, error) {
	s := a.GetSettings()
	title := "Select Cookies File"
	file, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		DefaultDirectory: filepath.Dir(s.CookieFilePath),
		Title:            title,
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
	return filepath.Join(a.configDir, "cookies", strings.ToLower(browser)+".txt")
}

func (a *App) cookieCachePathForSettings(s Settings) string {
	if s.CookieSource != "browser" || s.CookieBrowser == "" {
		return ""
	}
	return a.defaultCookieCachePath(s.CookieBrowser)
}

func (a *App) BrowserCookieCacheAvailable(browser string) (bool, error) {
	if _, ok := browserProcessNames[strings.ToLower(browser)]; !ok {
		return false, fmt.Errorf("unknown browser: %s", browser)
	}
	path := a.defaultCookieCachePath(browser)
	return isUsableCookieCache(path), nil
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
		if isUsableCookieCache(s.CookieCachePath) && strings.ToLower(s.CookieCacheBrowser) == browser {
			args = append(args, "--cookies", s.CookieCachePath)
		} else {
			args = append(args, "--cookies-from-browser", browser, "--cookies", a.cookieCachePathForSettings(s))
		}
	} else if s.CookieSource == "file" && s.CookieFilePath != "" {
		args = append(args, "--cookies", s.CookieFilePath)
	}
	return args
}

func isUsableCookieCache(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
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
