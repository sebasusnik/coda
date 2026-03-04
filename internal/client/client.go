package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sebasusnik/coda/internal/auth"
	"github.com/sebasusnik/coda/internal/device"
	"github.com/sebasusnik/coda/internal/ui"
)

const spotifyBaseURL = "https://api.spotify.com/v1"

type SearchResponse struct {
	Tracks struct {
		Items []Track `json:"items"`
	} `json:"tracks"`
}

type Track struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	DurationMs int      `json:"duration_ms"`
	Artists    []Artist `json:"artists"`
	Album      Album    `json:"album"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PlaybackState struct {
	IsPlaying    bool   `json:"is_playing"`
	Item         Track  `json:"item"`
	Progress     int    `json:"progress_ms"`
	ShuffleState bool   `json:"shuffle_state"`
	RepeatState  string `json:"repeat_state"` // "off" | "track" | "context"
	Device       struct {
		ID     string `json:"id"`
		Volume int    `json:"volume_percent"`
	} `json:"device"`
}

type AlbumItem struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Artists []Artist `json:"artists"`
}

type albumSearchResponse struct {
	Albums struct {
		Items []AlbumItem `json:"items"`
	} `json:"albums"`
}

type PlaylistItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Owner struct {
		DisplayName string `json:"display_name"`
	} `json:"owner"`
}

type playlistSearchResponse struct {
	Playlists struct {
		Items []PlaylistItem `json:"items"`
	} `json:"playlists"`
}

// isSuccess returns true for any 2xx status code. Spotify's playback endpoints
// can return either 200 or 204 depending on the version and device type.
func isSuccess(code int) bool {
	return code >= 200 && code < 300
}

func makeSpotifyRequest(method, endpoint string, body []byte) (*http.Response, error) {
	token, err := auth.GetValidToken()
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, spotifyBaseURL+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

func artistNames(artists []Artist) string {
	var parts []string
	for _, a := range artists {
		parts = append(parts, a.Name)
	}
	return strings.Join(parts, ", ")
}

func SearchTracks(query string, playFirst bool) error {
	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "track")
	params.Set("limit", "10")

	resp, err := makeSpotifyRequest("GET", "/search?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("spotify API error: %s", resp.Status)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return err
	}

	results := searchResp.Tracks.Items
	if len(results) == 0 {
		ui.Info("no tracks found")
		return nil
	}

	saveSearchResults(results)

	if playFirst {
		track := results[0]
		ui.Playing(artistNames(track.Artists), track.Name)
		deviceID := device.ResolveDeviceID()
		return playTrack(track.URI, deviceID)
	}

	ui.Header(fmt.Sprintf("search results (%d)", len(results)))
	for i, track := range results {
		ui.Track(i+1, artistNames(track.Artists), track.Name, track.Album.Name)
	}

	return nil
}

func PlayByNumber(numberStr string) error {
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return fmt.Errorf("invalid track number: %s", numberStr)
	}

	results, err := loadSearchResults()
	if err != nil || len(results) == 0 {
		return fmt.Errorf("no search results available. Run 'coda search <query>' first")
	}

	if number < 1 || number > len(results) {
		return fmt.Errorf("track number must be between 1 and %d", len(results))
	}

	track := results[number-1]
	deviceID := device.ResolveDeviceID()
	return playTrack(track.URI, deviceID)
}

func playTrack(uri string, deviceID string) error {
	body := map[string]interface{}{
		"uris": []string{uri},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	endpoint := "/me/player/play"
	if deviceID != "" {
		endpoint += "?device_id=" + deviceID
	}

	resp, err := makeSpotifyRequest("PUT", endpoint, jsonBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no active Spotify device found. Run 'coda device setup' first")
	}

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to play track: %s", resp.Status)
	}

	ui.Success("playing")
	return nil
}

func Pause() error {
	resp, err := makeSpotifyRequest("PUT", "/me/player/pause", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no active Spotify device found")
	}

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to pause: %s", resp.Status)
	}

	ui.Success("paused")
	return nil
}

func Next() error {
	resp, err := makeSpotifyRequest("POST", "/me/player/next", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no active Spotify device found")
	}

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to skip to next track: %s", resp.Status)
	}

	ui.Success("next track")
	return nil
}

func Previous() error {
	resp, err := makeSpotifyRequest("POST", "/me/player/previous", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no active Spotify device found")
	}

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to go to previous track: %s", resp.Status)
	}

	ui.Success("previous track")
	return nil
}

func Status() error {
	resp, err := makeSpotifyRequest("GET", "/me/player", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		ui.Info("nothing playing")
		return nil
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to get playback status: %s", resp.Status)
	}

	var playback PlaybackState
	if err := json.NewDecoder(resp.Body).Decode(&playback); err != nil {
		return err
	}

	progress := time.Duration(playback.Progress) * time.Millisecond
	ui.Status(playback.IsPlaying, artistNames(playback.Item.Artists), playback.Item.Name, playback.Item.Album.Name, progress.Round(time.Second).String())

	return nil
}

func RadioMode() error {
	// 1. Get currently playing track
	resp1, err := makeSpotifyRequest("GET", "/me/player/currently-playing", nil)
	if err != nil {
		return err
	}
	defer resp1.Body.Close()

	if resp1.StatusCode == 204 {
		return fmt.Errorf("no music currently playing")
	}

	var current struct {
		Item Track `json:"item"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&current); err != nil {
		return err
	}

	if current.Item.ID == "" || len(current.Item.Artists) == 0 {
		return fmt.Errorf("could not read current track")
	}

	artistID := current.Item.Artists[0].ID
	artistName := current.Item.Artists[0].Name

	// 2. Fetch related artists
	resp2, err := makeSpotifyRequest("GET", "/artists/"+artistID+"/related-artists", nil)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	if !isSuccess(resp2.StatusCode) {
		return fmt.Errorf("failed to fetch related artists: %s", resp2.Status)
	}

	var relatedResp struct {
		Artists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"artists"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&relatedResp); err != nil {
		return err
	}

	// 3. Collect seed IDs: current artist + up to 4 related
	seedIDs := []string{artistID}
	for i, a := range relatedResp.Artists {
		if i >= 4 {
			break
		}
		seedIDs = append(seedIDs, a.ID)
	}

	// 4. Fetch top tracks per seed artist and build a mixed queue
	// Always start with the current track
	uris := []string{current.Item.URI}

	for _, id := range seedIDs {
		resp, err := makeSpotifyRequest("GET", "/artists/"+id+"/top-tracks?market=from_token", nil)
		if err != nil {
			continue
		}

		if !isSuccess(resp.StatusCode) {
			resp.Body.Close()
			continue
		}

		var topTracks struct {
			Tracks []Track `json:"tracks"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&topTracks); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		// Take up to 3 tracks per artist to keep the mix varied
		count := 0
		for _, t := range topTracks.Tracks {
			if count >= 3 {
				break
			}
			if t.ID == current.Item.ID {
				continue
			}
			uris = append(uris, t.URI)
			count++
		}
	}

	if len(uris) <= 1 {
		return fmt.Errorf("could not find enough tracks to start radio")
	}

	// 5. Play the queue
	deviceID := device.ResolveDeviceID()
	body := map[string]interface{}{
		"uris": uris,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	endpoint := "/me/player/play"
	if deviceID != "" {
		endpoint += "?device_id=" + deviceID
	}

	resp3, err := makeSpotifyRequest("PUT", endpoint, jsonBody)
	if err != nil {
		return err
	}
	defer resp3.Body.Close()

	if !isSuccess(resp3.StatusCode) {
		return fmt.Errorf("failed to start radio mode: %s", resp3.Status)
	}

	ui.Successf("radio started -- seeded from %s and related artists", artistName)
	return nil
}

func AlbumMode() error {
	// 1. Get currently playing track
	resp1, err := makeSpotifyRequest("GET", "/me/player/currently-playing", nil)
	if err != nil {
		return err
	}
	defer resp1.Body.Close()

	if resp1.StatusCode == 204 {
		return fmt.Errorf("no music currently playing")
	}

	var current struct {
		Item Track `json:"item"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&current); err != nil {
		return err
	}

	if current.Item.Album.ID == "" {
		return fmt.Errorf("could not read current album")
	}

	// 2. Play the entire album
	deviceID := device.ResolveDeviceID()
	body := map[string]interface{}{
		"context_uri": "spotify:album:" + current.Item.Album.ID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	endpoint := "/me/player/play"
	if deviceID != "" {
		endpoint += "?device_id=" + deviceID
	}

	resp2, err := makeSpotifyRequest("PUT", endpoint, jsonBody)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	if !isSuccess(resp2.StatusCode) {
		return fmt.Errorf("failed to start album mode: %s", resp2.Status)
	}

	ui.Success("playing album")
	return nil
}

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

func saveSearchResults(results []Track) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	configDir := filepath.Join(homeDir, ".config", "coda")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return
	}
	data, err := json.Marshal(results)
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(configDir, "last_search.json"), data, 0600)
}

func loadSearchResults() ([]Track, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".config", "coda")
	data, err := os.ReadFile(filepath.Join(configDir, "last_search.json"))
	if err != nil {
		return nil, err
	}

	var results []Track
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func saveAlbumResults(results []AlbumItem) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	configDir := filepath.Join(homeDir, ".config", "coda")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return
	}
	data, err := json.Marshal(results)
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(configDir, "last_search_albums.json"), data, 0600)
}

func savePlaylistResults(results []PlaylistItem) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	configDir := filepath.Join(homeDir, ".config", "coda")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return
	}
	data, err := json.Marshal(results)
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(configDir, "last_search_playlists.json"), data, 0600)
}

func GetPlaybackState() (*PlaybackState, error) {
	resp, err := makeSpotifyRequest("GET", "/me/player", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		return nil, fmt.Errorf("nothing currently playing")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get playback state: %s", resp.Status)
	}

	var playback PlaybackState
	if err := json.NewDecoder(resp.Body).Decode(&playback); err != nil {
		return nil, err
	}
	return &playback, nil
}

func Resume() error {
	deviceID := device.ResolveDeviceID()
	endpoint := "/me/player/play"
	if deviceID != "" {
		endpoint += "?device_id=" + deviceID
	}

	resp, err := makeSpotifyRequest("PUT", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no active Spotify device found. Run 'coda device setup' first")
	}

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to resume: %s", resp.Status)
	}

	ui.Success("playing")
	return nil
}

func Toggle() error {
	playback, err := GetPlaybackState()
	if err != nil {
		return err
	}

	if playback.IsPlaying {
		return Pause()
	}
	return Resume()
}

func Like() error {
	resp, err := makeSpotifyRequest("GET", "/me/player/currently-playing", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		return fmt.Errorf("nothing currently playing")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to get current track: %s", resp.Status)
	}

	var current struct {
		Item Track `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&current); err != nil {
		return err
	}

	if current.Item.ID == "" {
		return fmt.Errorf("could not read current track")
	}

	likeResp, err := makeSpotifyRequest("PUT", "/me/tracks?ids="+current.Item.ID, nil)
	if err != nil {
		return err
	}
	defer likeResp.Body.Close()

	if !isSuccess(likeResp.StatusCode) {
		return fmt.Errorf("failed to like track: %s", likeResp.Status)
	}

	ui.Successf("liked: %s — %s", artistNames(current.Item.Artists), current.Item.Name)
	return nil
}

func SetVolume(input string) error {
	playback, err := GetPlaybackState()
	if err != nil {
		return err
	}

	current := playback.Device.Volume
	var target int

	switch input {
	case "up":
		target = current + 10
		if target > 100 {
			target = 100
		}
	case "down":
		target = current - 10
		if target < 0 {
			target = 0
		}
	default:
		n, err := strconv.Atoi(input)
		if err != nil || n < 0 || n > 100 {
			return fmt.Errorf("volume must be 0-100, 'up', or 'down'")
		}
		target = n
	}

	endpoint := fmt.Sprintf("/me/player/volume?volume_percent=%d", target)
	if playback.Device.ID != "" {
		endpoint += "&device_id=" + playback.Device.ID
	}

	volResp, err := makeSpotifyRequest("PUT", endpoint, nil)
	if err != nil {
		return err
	}
	defer volResp.Body.Close()

	if !isSuccess(volResp.StatusCode) {
		return fmt.Errorf("failed to set volume: %s", volResp.Status)
	}

	ui.Volume(target)
	return nil
}

func ToggleShuffle() error {
	playback, err := GetPlaybackState()
	if err != nil {
		return err
	}

	next := !playback.ShuffleState
	endpoint := fmt.Sprintf("/me/player/shuffle?state=%t", next)
	if playback.Device.ID != "" {
		endpoint += "&device_id=" + playback.Device.ID
	}

	resp, err := makeSpotifyRequest("PUT", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to toggle shuffle: %s", resp.Status)
	}

	if next {
		ui.Success("shuffle on")
	} else {
		ui.Success("shuffle off")
	}
	return nil
}

func CycleRepeat() error {
	playback, err := GetPlaybackState()
	if err != nil {
		return err
	}

	var next string
	switch playback.RepeatState {
	case "off":
		next = "context"
	case "context":
		next = "track"
	default:
		next = "off"
	}

	endpoint := "/me/player/repeat?state=" + next
	if playback.Device.ID != "" {
		endpoint += "&device_id=" + playback.Device.ID
	}

	resp, err := makeSpotifyRequest("PUT", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to set repeat: %s", resp.Status)
	}

	ui.Successf("repeat: %s", next)
	return nil
}

func SearchAlbums(query string, playFirst bool) error {
	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "album")
	params.Set("limit", "10")

	resp, err := makeSpotifyRequest("GET", "/search?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("spotify API error: %s", resp.Status)
	}

	var searchResp albumSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return err
	}

	results := searchResp.Albums.Items
	if len(results) == 0 {
		ui.Info("no albums found")
		return nil
	}

	saveAlbumResults(results)

	if playFirst {
		album := results[0]
		deviceID := device.ResolveDeviceID()
		body := map[string]interface{}{
			"context_uri": "spotify:album:" + album.ID,
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %v", err)
		}
		endpoint := "/me/player/play"
		if deviceID != "" {
			endpoint += "?device_id=" + deviceID
		}
		playResp, err := makeSpotifyRequest("PUT", endpoint, jsonBody)
		if err != nil {
			return err
		}
		defer playResp.Body.Close()
		if !isSuccess(playResp.StatusCode) {
			return fmt.Errorf("failed to play album: %s", playResp.Status)
		}
		ui.Playing(artistNames(album.Artists), album.Name)
		return nil
	}

	ui.Header(fmt.Sprintf("search results (%d)", len(results)))
	for i, album := range results {
		ui.Album(i+1, artistNames(album.Artists), album.Name)
	}

	return nil
}

func SearchPlaylists(query string, playFirst bool) error {
	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "playlist")
	params.Set("limit", "10")

	resp, err := makeSpotifyRequest("GET", "/search?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("spotify API error: %s", resp.Status)
	}

	var searchResp playlistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return err
	}

	results := searchResp.Playlists.Items
	if len(results) == 0 {
		ui.Info("no playlists found")
		return nil
	}

	savePlaylistResults(results)

	if playFirst {
		pl := results[0]
		deviceID := device.ResolveDeviceID()
		body := map[string]interface{}{
			"context_uri": "spotify:playlist:" + pl.ID,
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %v", err)
		}
		endpoint := "/me/player/play"
		if deviceID != "" {
			endpoint += "?device_id=" + deviceID
		}
		playResp, err := makeSpotifyRequest("PUT", endpoint, jsonBody)
		if err != nil {
			return err
		}
		defer playResp.Body.Close()
		if !isSuccess(playResp.StatusCode) {
			return fmt.Errorf("failed to play playlist: %s", playResp.Status)
		}
		ui.Playing(pl.Owner.DisplayName, pl.Name)
		return nil
	}

	ui.Header(fmt.Sprintf("search results (%d)", len(results)))
	for i, pl := range results {
		ui.Playlist(i+1, pl.Owner.DisplayName, pl.Name)
	}

	return nil
}

// --- raw search + play helpers for the TUI ---

// SearchTracksRaw returns up to 9 track results without printing or caching.
func SearchTracksRaw(query string) ([]Track, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "track")
	params.Set("limit", "9")

	resp, err := makeSpotifyRequest("GET", "/search?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("spotify API error: %s", resp.Status)
	}

	var sr SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	return sr.Tracks.Items, nil
}

// SearchAlbumsRaw returns up to 9 album results without printing or caching.
func SearchAlbumsRaw(query string) ([]AlbumItem, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "album")
	params.Set("limit", "9")

	resp, err := makeSpotifyRequest("GET", "/search?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("spotify API error: %s", resp.Status)
	}

	var sr albumSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	return sr.Albums.Items, nil
}

// SearchPlaylistsRaw returns up to 9 playlist results without printing or caching.
func SearchPlaylistsRaw(query string) ([]PlaylistItem, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "playlist")
	params.Set("limit", "9")

	resp, err := makeSpotifyRequest("GET", "/search?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("spotify API error: %s", resp.Status)
	}

	var sr playlistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	return sr.Playlists.Items, nil
}

type RecentlyPlayedItem struct {
	Track    Track  `json:"track"`
	PlayedAt string `json:"played_at"`
}

type recentlyPlayedResponse struct {
	Items []RecentlyPlayedItem `json:"items"`
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

// PlayURI plays a single track by Spotify URI.
func PlayURI(trackURI string) error {
	return playTrack(trackURI, device.ResolveDeviceID())
}

// PlayContextURI plays an album or playlist by Spotify context URI.
func PlayContextURI(contextURI string) error {
	deviceID := device.ResolveDeviceID()
	body := map[string]interface{}{"context_uri": contextURI}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}
	endpoint := "/me/player/play"
	if deviceID != "" {
		endpoint += "?device_id=" + deviceID
	}
	resp, err := makeSpotifyRequest("PUT", endpoint, jsonBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return fmt.Errorf("no active Spotify device found. Run 'coda device setup' first")
	}
	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to play: %s", resp.Status)
	}
	return nil
}
