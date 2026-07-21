package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const lastfmEndpoint = "https://ws.audioscrobbler.com/2.0/"

type lastfmTrackPayload struct {
	Name      string `json:"name"`
	Playcount string `json:"playcount"`
	URL       string `json:"url"`
	Artist    struct {
		Name string `json:"name"`
		Text string `json:"#text"`
	} `json:"artist"`
}

type lastfmResponse struct {
	TopTracks struct {
		Track []lastfmTrackPayload `json:"track"`
	} `json:"toptracks"`
	LovedTracks struct {
		Track []lastfmTrackPayload `json:"track"`
	} `json:"lovedtracks"`
	RecentTracks struct {
		Track []lastfmTrackPayload `json:"track"`
	} `json:"recenttracks"`
	Error   int    `json:"error"`
	Message string `json:"message"`
}

func (a *App) FetchLastfmTracks(username, apiKey, source, period string, limit int) ([]LastfmTrack, error) {
	username = strings.TrimSpace(username)
	apiKey = strings.TrimSpace(apiKey)
	if username == "" || len(username) > 64 {
		return nil, errors.New("Last.fm username is required")
	}
	if apiKey == "" || len(apiKey) > 128 {
		return nil, errors.New("Last.fm API key is required")
	}
	method := map[string]string{"top": "user.gettoptracks", "loved": "user.getlovedtracks", "recent": "user.getrecenttracks"}[source]
	if method == "" {
		return nil, errors.New("unsupported Last.fm source")
	}
	if limit < 1 || limit > 50 {
		limit = 25
	}
	if period != "7day" && period != "1month" && period != "3month" && period != "6month" && period != "12month" && period != "overall" {
		period = "1month"
	}
	params := url.Values{
		"method": {method}, "user": {username}, "api_key": {apiKey},
		"format": {"json"}, "limit": {strconv.Itoa(limit)}, "period": {period},
	}
	ctx, cancel := context.WithTimeout(a.appContext(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lastfmEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Last.fm request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Last.fm returned HTTP %d", resp.StatusCode)
	}
	var payload lastfmResponse
	decoder := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 2<<20))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse Last.fm response: %w", err)
	}
	if payload.Error != 0 {
		return nil, fmt.Errorf("Last.fm: %s", payload.Message)
	}
	tracks := payload.TopTracks.Track
	if source == "loved" {
		tracks = payload.LovedTracks.Track
	} else if source == "recent" {
		tracks = payload.RecentTracks.Track
	}
	result := make([]LastfmTrack, 0, len(tracks))
	seen := make(map[string]bool)
	for _, track := range tracks {
		artist := strings.TrimSpace(track.Artist.Name)
		if artist == "" {
			artist = strings.TrimSpace(track.Artist.Text)
		}
		title := strings.TrimSpace(track.Name)
		key := strings.ToLower(artist + "\x00" + title)
		if artist == "" || title == "" || seen[key] {
			continue
		}
		plays, _ := strconv.Atoi(track.Playcount)
		seen[key] = true
		result = append(result, LastfmTrack{Artist: artist, Title: title, URL: track.URL, Plays: plays})
	}
	return result, nil
}

func (a *App) ResolveMusicTrack(artist, title string) (string, error) {
	ctx, cancel := context.WithTimeout(a.appContext(), 30*time.Second)
	defer cancel()
	result, err := a.resolveMusicTrack(ctx, artist, title)
	return result.URL, err
}

func (a *App) resolveMusicTrack(parent context.Context, artist, title string) (MusicMatchResult, error) {
	match := MusicMatchResult{Artist: strings.TrimSpace(artist), Title: strings.TrimSpace(title)}
	artist = strings.TrimSpace(artist)
	title = strings.TrimSpace(title)
	if artist == "" || title == "" || len(artist) > 256 || len(title) > 256 {
		return match, errors.New("artist and title are required")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	query := "ytsearch1:" + artist + " - " + title + " official audio"
	stdout, err := commandOutputLimited(ctx, 2<<20, a.ytdlpPath(), "--no-warnings", "--dump-single-json", "--skip-download", "--", query)
	if err != nil {
		return match, fmt.Errorf("music match failed: %w", err)
	}
	var result struct {
		WebpageURL string `json:"webpage_url"`
		URL        string `json:"url"`
		ID         string `json:"id"`
		Title      string `json:"title"`
		Uploader   string `json:"uploader"`
		Thumbnail  string `json:"thumbnail"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return match, fmt.Errorf("parse music match: %w", err)
	}
	resolved := result.WebpageURL
	if resolved == "" && result.ID != "" {
		resolved = "https://www.youtube.com/watch?v=" + result.ID
	}
	if !isAllowedDownloadURL(resolved) {
		return match, errors.New("music match returned no usable URL")
	}
	match.URL = resolved
	match.MatchedTitle = strings.TrimSpace(result.Title)
	match.Uploader = strings.TrimSpace(result.Uploader)
	match.Thumbnail = sanitizeRemoteMediaURLWithResolver(ctx, result.Thumbnail)
	return match, nil
}

func (a *App) ResolveMusicTracks(jobID string, tracks []LastfmTrack) ([]MusicMatchResult, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || len(jobID) > 128 {
		return nil, errors.New("music match job ID is required")
	}
	if len(tracks) == 0 || len(tracks) > 25 {
		return nil, errors.New("select between 1 and 25 tracks")
	}
	ctx, cancel := context.WithCancel(a.appContext())
	a.musicMatchMu.Lock()
	if a.musicMatchJobs == nil {
		a.musicMatchJobs = make(map[string]context.CancelFunc)
	}
	if _, exists := a.musicMatchJobs[jobID]; exists {
		a.musicMatchMu.Unlock()
		cancel()
		return nil, errors.New("music match job already exists")
	}
	a.musicMatchJobs[jobID] = cancel
	a.musicMatchMu.Unlock()
	defer func() {
		cancel()
		a.musicMatchMu.Lock()
		delete(a.musicMatchJobs, jobID)
		a.musicMatchMu.Unlock()
	}()

	results := make([]MusicMatchResult, len(tracks))
	for index, track := range tracks {
		results[index] = MusicMatchResult{Artist: track.Artist, Title: track.Title, Error: "matching cancelled"}
	}
	indices := make(chan int)
	var workers sync.WaitGroup
	workerCount := 3
	if len(tracks) < workerCount {
		workerCount = len(tracks)
	}
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range indices {
				track := tracks[index]
				result := MusicMatchResult{Artist: track.Artist, Title: track.Title}
				match, err := a.resolveMusicTrack(ctx, track.Artist, track.Title)
				if err != nil {
					result.Error = err.Error()
				} else {
					result = match
				}
				results[index] = result
			}
		}()
	}
	for index := range tracks {
		select {
		case indices <- index:
		case <-ctx.Done():
			close(indices)
			workers.Wait()
			return results, nil
		}
	}
	close(indices)
	workers.Wait()
	return results, nil
}

func (a *App) CancelMusicMatching(jobID string) {
	a.musicMatchMu.Lock()
	cancel := a.musicMatchJobs[strings.TrimSpace(jobID)]
	a.musicMatchMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
