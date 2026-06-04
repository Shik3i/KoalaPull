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

	"golang.org/x/crypto/openpgp"
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
			URL:          url,
			SignatureURL: url + "/sig",
			MaxBytes:     maxFFmpegDownloadBytes,
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
	case artifact.SignatureURL != "":
		signature, err := fetchLimitedBytes(ctx, artifact.SignatureURL, maxSignatureBytes)
		if err != nil {
			return fmt.Errorf("fetch ffmpeg signature: %w", err)
		}
		return verifyEvermeetSignature(archivePath, signature)
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

func verifyEvermeetSignature(archivePath string, signature []byte) error {
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(evermeetSigningPublicKey))
	if err != nil {
		return fmt.Errorf("parse embedded signing key: %w", err)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	if _, err := openpgp.CheckDetachedSignature(keyring, archive, bytes.NewReader(signature)); err != nil {
		return fmt.Errorf("ffmpeg signature verification failed: %w", err)
	}
	return nil
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

const evermeetSigningPublicKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
t3FlBVsKtBiOa6FvDptN
=Q36a
-----END PGP PUBLIC KEY BLOCK-----`
