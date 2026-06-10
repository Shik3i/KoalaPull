package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) CheckDependencies() DependencyStatus {
	return DependencyStatus{
		YtDlpInstalled:  fileExists(a.ytdlpPath()),
		FfmpegInstalled: a.ffmpegToolsInstalled(),
	}
}

func (a *App) ffmpegToolsInstalled() bool {
	if fileExists(a.ffmpegPath()) {
		if runtime.GOOS == "windows" {
			return fileExists(a.ffprobePath())
		}
		if runtime.GOOS == "darwin" {
			// On darwin we check both since we download them separately
			artifact := ffmpegArtifactFor(runtime.GOOS)
			if artifact.FFprobeURL != "" {
				return fileExists(a.ffprobePath())
			}
		}
		return true
	}
	return false
}

func (a *App) GetYtdlpVersion() string {
	path := a.ytdlpPath()
	if !fileExists(path) {
		return ""
	}
	ctx, cancel := context.WithTimeout(a.appContext(), 5*time.Second)
	defer cancel()
	out, err := commandOutputLimited(ctx, 1024, path, "--version")
	if err != nil {
		log.Printf("GetYtdlpVersion: %v", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (a *App) GetFfmpegVersion() string {
	path := a.ffmpegPath()
	if !fileExists(path) {
		return ""
	}
	ctx, cancel := context.WithTimeout(a.appContext(), 5*time.Second)
	defer cancel()
	out, err := commandOutputLimited(ctx, 4096, path, "-version")
	if err != nil {
		log.Printf("GetFfmpegVersion: %v", err)
		return ""
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		fields := strings.Fields(lines[0])
		if len(fields) > 2 && fields[0] == "ffmpeg" && fields[1] == "version" {
			return fields[2]
		}
	}
	return "unknown"
}

func (a *App) UpdateDependencies() error {
	a.adMu.Lock()
	activeCount := len(a.activeDownloads)
	a.adMu.Unlock()
	if activeCount > 0 {
		return errors.New("cannot update dependencies while downloads are in progress")
	}
	a.dependencyMu.Lock()
	defer a.dependencyMu.Unlock()
	if err := a.downloadYtdlp(true); err != nil {
		a.emitProgress("yt-dlp", 0, "error", err.Error())
		return fmt.Errorf("yt-dlp update failed: %w", err)
	}
	if err := a.downloadFfmpeg(true); err != nil {
		a.emitProgress("ffmpeg", 0, "error", err.Error())
		return fmt.Errorf("ffmpeg update failed: %w", err)
	}
	return nil
}

func (a *App) OpenBinDir() error {
	cleaned, err := cleanAbsolutePath(a.binDir)
	if err != nil {
		return err
	}
	if !fileExists(cleaned) {
		return fmt.Errorf("directory does not exist")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = command("explorer.exe", cleaned)
	case "darwin":
		cmd = command("open", cleaned)
	default:
		cmd = command("xdg-open", cleaned)
	}
	return cmd.Start()
}

func (a *App) OpenOutputDir(dir string) error {
	if dir == "" {
		settings := a.GetSettings()
		dir = settings.DefaultOutputDir
	}
	cleaned, err := cleanAbsolutePath(dir)
	if err != nil {
		return err
	}
	if !a.isAllowedOpenDir(cleaned) {
		return errors.New("directory is outside KoalaPull-managed paths")
	}
	if !fileExists(cleaned) {
		return fmt.Errorf("directory does not exist")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = command("explorer.exe", cleaned)
	case "darwin":
		cmd = command("open", cleaned)
	default:
		cmd = command("xdg-open", cleaned)
	}
	return cmd.Start()
}

func (a *App) isAllowedOpenDir(dir string) bool {
	settings := a.GetSettings()
	defaultDir, err := cleanAbsolutePath(settings.DefaultOutputDir)
	if err == nil && isWithinPath(dir, defaultDir) {
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
	a.emitDetailedProgress(dep, pct, status, errMsg, 0, 0, "", "")
}

func (a *App) emitDetailedProgress(dep string, pct int, status, errMsg string, total, read int64, speed, eta string) {
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
		BytesTotal: total,
		BytesRead:  read,
		Speed:      speed,
		ETA:        eta,
	}
	if errMsg != "" {
		ev.Error = errMsg
	}
	wailsRuntime.EventsEmit(a.appContext(), "dependency-progress", ev)
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
	var startBytes int64
	var out *os.File
	var err error

	if fileExists(destPath) {
		fi, err := os.Stat(destPath)
		if err == nil {
			startBytes = fi.Size()
		}
	}

	if startBytes > 0 {
		out, err = os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY, privateFileMode)
		if err != nil {
			startBytes = 0
		}
	}

	if startBytes == 0 {
		out, err = os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, privateFileMode)
		if err != nil {
			return fmt.Errorf("create file failed: %w", err)
		}
	}

	defer func() {
		if out != nil {
			retErr = errors.Join(retErr, out.Close())
		}
		if retErr != nil && startBytes == 0 {
			_ = os.Remove(destPath)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}

	if startBytes > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startBytes))
	}

	if !isTesting && !isTrustedDependencyURL(req.URL) {
		return fmt.Errorf("untrusted dependency host: %s", req.URL.Hostname())
	}

	resp, err := trustedDependencyHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if startBytes > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		resp.Body.Close()
		startBytes = 0
		_ = out.Close()
		out, err = os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, privateFileMode)
		if err != nil {
			return fmt.Errorf("create file failed: %w", err)
		}
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err = trustedDependencyHTTPClient().Do(req)
		if err != nil {
			return fmt.Errorf("http get failed: %w", err)
		}
	} else if startBytes > 0 && resp.StatusCode != http.StatusPartialContent {
		startBytes = 0
		_ = out.Truncate(0)
		_, _ = out.Seek(0, 0)
	} else if startBytes == 0 && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	total := resp.ContentLength
	if startBytes > 0 {
		total += startBytes
	}
	if total > maxBytes {
		return fmt.Errorf("download exceeds %d bytes", maxBytes)
	}

	lastPct := -1
	buf := make([]byte, 32*1024)
	written := startBytes
	startTime := time.Now()
	lastEmitTime := time.Now()

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
				now := time.Now()
				elapsed := now.Sub(startTime)
				if pct != lastPct || now.Sub(lastEmitTime) > 200*time.Millisecond {
					lastPct = pct
					lastEmitTime = now
					var speedStr, etaStr string
					if elapsed > 100*time.Millisecond {
						speed := float64(written-startBytes) / elapsed.Seconds()
						speedStr = formatByteRate(speed)
						remaining := total - written
						if speed > 0 {
							etaSecs := float64(remaining) / speed
							etaStr = formatRemainingTime(int(etaSecs))
						}
					}
					a.emitDetailedProgress(depName, pct, "downloading", "", total, written, speedStr, etaStr)
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

func formatByteRate(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024*1024 {
		return fmt.Sprintf("%.2f GB/s", bytesPerSec/(1024*1024*1024))
	}
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/(1024*1024))
	}
	if bytesPerSec >= 1024 {
		return fmt.Sprintf("%.2f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.2f B/s", bytesPerSec)
}

func formatRemainingTime(seconds int) string {
	if seconds < 0 {
		return "00:00"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
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
