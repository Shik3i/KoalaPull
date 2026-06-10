package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

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
