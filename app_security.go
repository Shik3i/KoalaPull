package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

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

func isSafePlayableFile(path string) bool {
	_, ok := playableFileExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
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
