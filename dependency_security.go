package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	maxYtdlpDownloadBytes    int64 = 128 << 20
	maxFFmpegDownloadBytes   int64 = 1 << 30
	maxFFmpegBinaryBytes     int64 = 512 << 20
	maxChecksumDownloadBytes int64 = 16 << 20
	maxSignatureBytes        int64 = 1 << 20
	maxArchiveMembers              = 100_000
)

type dependencyArtifact struct {
	URL          string
	AssetName    string
	ChecksumURL  string
	SignatureURL string
	MaxBytes     int64
}

func ffmpegArtifactFor(goos string) dependencyArtifact {
	switch goos {
	case "windows":
		const name = "ffmpeg-master-latest-win64-gpl.zip"
		return dependencyArtifact{
			URL:         "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/" + name,
			AssetName:   name,
			ChecksumURL: "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/checksums.sha256",
			MaxBytes:    maxFFmpegDownloadBytes,
		}
	case "darwin":
		const url = "https://evermeet.cx/ffmpeg/get/zip"
		return dependencyArtifact{
			URL:         url,
			ChecksumURL: url + "/sha256",
			MaxBytes:    maxFFmpegDownloadBytes,
		}
	default:
		const name = "ffmpeg-master-latest-linux64-gpl.tar.xz"
		return dependencyArtifact{
			URL:         "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/" + name,
			AssetName:   name,
			ChecksumURL: "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/checksums.sha256",
			MaxBytes:    maxFFmpegDownloadBytes,
		}
	}
}

func verifyFFmpegArchive(ctx context.Context, archivePath string, artifact dependencyArtifact) error {
	switch {
	case artifact.ChecksumURL != "":
		expected, err := fetchChecksumForAsset(ctx, artifact.ChecksumURL, artifact.AssetName)
		if err != nil {
			return fmt.Errorf("fetch ffmpeg checksum: %w", err)
		}
		return verifyFileChecksum(archivePath, expected, "ffmpeg")
	default:
		return errors.New("ffmpeg artifact has no integrity verification")
	}
}

func verifyFileChecksum(path, expected, label string) error {
	actual, err := sha256File(path)
	if err != nil {
		return fmt.Errorf("hash %s: %w", label, err)
	}
	if actual != strings.ToLower(expected) {
		return fmt.Errorf("%s checksum mismatch", label)
	}
	return nil
}

func fetchLimitedBytes(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	return data, nil
}



func extractFFmpegFromZipBounded(ctx context.Context, archivePath, destPath string) error {
	return extractZipBinaryBounded(ctx, archivePath, destPath, "ffmpeg", "ffmpeg.exe")
}

func extractZipBinaryBounded(ctx context.Context, archivePath, destPath string, allowedNames ...string) error {
	if len(allowedNames) == 0 {
		return errors.New("no archive binary names provided")
	}
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	if len(r.File) > maxArchiveMembers {
		return fmt.Errorf("archive has too many members")
	}
	for _, f := range r.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if f.FileInfo().IsDir() || !stringInSlice(filepath.Base(f.Name), allowedNames) {
			continue
		}
		if f.UncompressedSize64 > uint64(maxFFmpegBinaryBytes) {
			return fmt.Errorf("ffmpeg binary exceeds %d bytes", maxFFmpegBinaryBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeReaderBounded(ctx, destPath, rc, maxFFmpegBinaryBytes)
		closeErr := rc.Close()
		return errors.Join(err, closeErr)
	}
	return fmt.Errorf("%s binary not found in archive", allowedNames[0])
}

func stringInSlice(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func extractFFmpegFromTarXzBounded(ctx context.Context, archivePath, destPath string) error {
	member, err := findFFmpegTarMember(ctx, archivePath)
	if err != nil {
		return err
	}
	cmd := commandContext(ctx, "tar", "-xJOf", archivePath, member)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	copyErr := writeReaderBounded(ctx, destPath, stdout, maxFFmpegBinaryBytes)
	if copyErr != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if copyErr != nil {
		return copyErr
	}
	if waitErr != nil {
		return fmt.Errorf("tar extraction failed: %s: %w", stderr.String(), waitErr)
	}
	return nil
}

func findFFmpegTarMember(ctx context.Context, archivePath string) (string, error) {
	cmd := commandContext(ctx, "tar", "-tJf", archivePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var found string
	count := 0
	for scanner.Scan() {
		count++
		if count > maxArchiveMembers {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return "", errors.New("archive has too many members")
		}
		name := scanner.Text()
		if err := validateArchiveMemberName(name); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return "", err
		}
		if found == "" && filepath.Base(name) == "ffmpeg" {
			found = name
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if scanErr != nil {
		return "", scanErr
	}
	if waitErr != nil {
		return "", fmt.Errorf("tar list failed: %s: %w", stderr.String(), waitErr)
	}
	if found == "" {
		return "", errors.New("ffmpeg binary not found in archive")
	}
	return found, nil
}

func writeReaderBounded(ctx context.Context, destPath string, src io.Reader, maxBytes int64) (retErr error) {
	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, dst.Close())
		if retErr != nil {
			_ = os.Remove(destPath)
		}
	}()
	n, err := io.Copy(dst, io.LimitReader(contextReader{ctx: ctx, reader: src}, maxBytes+1))
	if err != nil {
		return err
	}
	if n > maxBytes {
		return fmt.Errorf("extracted binary exceeds %d bytes", maxBytes)
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	return nil
}

func replaceFilePreservingOld(srcPath, destPath string) error {
	return replaceFileAtomically(srcPath, destPath)
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) (retErr error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".koalapull-write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			retErr = errors.Join(retErr, tmp.Close())
		}
		if retErr != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	closed = true
	if err := replaceFilePreservingOld(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func extractFFmpegArchive(ctx context.Context, archivePath, destPath string) error {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return extractFFmpegFromZipBounded(ctx, archivePath, destPath)
	}
	return extractFFmpegFromTarXzBounded(ctx, archivePath, destPath)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}


