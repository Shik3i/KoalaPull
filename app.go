package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	binDir string
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

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	a.binDir = filepath.Join(configDir, "KoalaPull", "bin")
	if err := os.MkdirAll(a.binDir, 0755); err != nil {
		println("Failed to create bin directory:", err.Error())
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (a *App) CheckDependencies() DependencyStatus {
	return DependencyStatus{
		YtDlpInstalled:  fileExists(a.ytdlpPath()),
		FfmpegInstalled: fileExists(a.ffmpegPath()),
	}
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
	if runtime.GOOS == "windows" {
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
		if !strings.HasSuffix(f.Name, "ffmpeg.exe") && !strings.HasSuffix(f.Name, "ffprobe.exe") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}

		binName := filepath.Base(f.Name)
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
	cmd := exec.Command("tar", "-xJf", archivePath, "-C", destDir, "--strip-components=2", "--wildcards", "*/bin/ffmpeg")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar failed: %s: %w", string(output), err)
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
	base := "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest"
	switch runtime.GOOS {
	case "windows":
		return base + "/ffmpeg-master-latest-win64-gpl.zip"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return base + "/ffmpeg-master-latest-macos-arm64-gpl.tar.xz"
		}
		return base + "/ffmpeg-master-latest-macos-x86_64-gpl.tar.xz"
	default:
		return base + "/ffmpeg-master-latest-linux64-gpl.tar.xz"
	}
}
