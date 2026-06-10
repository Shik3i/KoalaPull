package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	defer a.adMu.Unlock()
	active, ok := a.activeDownloads[downloadID]
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
	active.paused = true
	return nil
}

func (a *App) ResumeDownload(downloadID string) error {
	a.adMu.Lock()
	defer a.adMu.Unlock()
	active, ok := a.activeDownloads[downloadID]
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
	active.paused = false
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
