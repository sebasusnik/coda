package client

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/sebasusnik/coda/internal/ui"
)

func Queue() error {
	resp, err := makeSpotifyRequest("GET", "/me/player/queue", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		ui.Info("nothing playing")
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to get queue: %s", resp.Status)
	}

	var result struct {
		CurrentlyPlaying Track   `json:"currently_playing"`
		Queue            []Track `json:"queue"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.CurrentlyPlaying.Name != "" {
		ui.Header("now playing")
		ui.Playing(artistNames(result.CurrentlyPlaying.Artists), result.CurrentlyPlaying.Name)
	}

	if len(result.Queue) == 0 {
		ui.Info("queue is empty")
		return nil
	}

	ui.Header("up next")
	for i, track := range result.Queue {
		if i >= 10 {
			ui.Infof("... and %d more", len(result.Queue)-10)
			break
		}
		ui.Track(i+1, artistNames(track.Artists), track.Name, track.Album.Name)
	}

	return nil
}

// QueueRaw returns the currently playing track and queue without printing.
func QueueRaw() (Track, []Track, error) {
	resp, err := makeSpotifyRequest("GET", "/me/player/queue", nil)
	if err != nil {
		return Track{}, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		return Track{}, nil, fmt.Errorf("nothing playing")
	}

	if resp.StatusCode != 200 {
		return Track{}, nil, fmt.Errorf("failed to get queue: %s", resp.Status)
	}

	var result struct {
		CurrentlyPlaying Track   `json:"currently_playing"`
		Queue            []Track `json:"queue"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Track{}, nil, err
	}

	return result.CurrentlyPlaying, result.Queue, nil
}

// AddToQueue adds a track URI to the user's playback queue.
func AddToQueue(trackURI string) error {
	endpoint := "/me/player/queue?uri=" + url.QueryEscape(trackURI)
	resp, err := makeSpotifyRequest("POST", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no active Spotify device found")
	}
	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to add to queue: %s", resp.Status)
	}
	return nil
}

// AddFirstToQueue searches for query and adds the top track result to the queue.
func AddFirstToQueue(query string) error {
	tracks, err := SearchTracksRaw(query)
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return fmt.Errorf("no tracks found for %q", query)
	}
	track := tracks[0]
	if err := AddToQueue(track.URI); err != nil {
		return err
	}
	ui.Successf("queued: %s — %s", artistNames(track.Artists), track.Name)
	return nil
}

// RecentlyPlayedRaw returns recently played tracks without printing.
func RecentlyPlayedRaw(limit int) ([]RecentlyPlayedItem, error) {
	resp, err := makeSpotifyRequest("GET", fmt.Sprintf("/me/player/recently-played?limit=%d", limit), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get recently played: %s", resp.Status)
	}

	var result recentlyPlayedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// RecentlyPlayed prints the last 10 recently played tracks.
func RecentlyPlayed() error {
	items, err := RecentlyPlayedRaw(10)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		ui.Info("no recently played tracks")
		return nil
	}
	ui.Header(fmt.Sprintf("recently played (%d)", len(items)))
	for i, item := range items {
		ui.Track(i+1, artistNames(item.Track.Artists), item.Track.Name, item.Track.Album.Name)
	}
	return nil
}

// LikedTracksRaw returns saved/liked tracks without printing.
func LikedTracksRaw(limit int) ([]SavedTrackItem, error) {
	resp, err := makeSpotifyRequest("GET", fmt.Sprintf("/me/tracks?limit=%d", limit), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get liked tracks: %s", resp.Status)
	}

	var result savedTracksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// LikedTracks prints the user's saved tracks.
func LikedTracks() error {
	items, err := LikedTracksRaw(10)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		ui.Info("no liked tracks")
		return nil
	}
	ui.Header(fmt.Sprintf("liked tracks (%d)", len(items)))
	for i, item := range items {
		ui.Track(i+1, artistNames(item.Track.Artists), item.Track.Name, item.Track.Album.Name)
	}
	return nil
}
