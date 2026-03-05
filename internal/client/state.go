package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sebasusnik/coda/internal/device"
	"github.com/sebasusnik/coda/internal/ui"
)

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

// UserPlaylistsRaw returns the current user's saved playlists without printing.
func UserPlaylistsRaw(limit int) ([]PlaylistItem, error) {
	resp, err := makeSpotifyRequest("GET", fmt.Sprintf("/me/playlists?limit=%d", limit), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get playlists: %s", resp.Status)
	}

	var result userPlaylistsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// AddCurrentToPlaylist finds a playlist by name and adds the currently playing
// track to it. It tries an exact case-insensitive match first, then falls back
// to a partial contains match.
func AddCurrentToPlaylist(query string) error {
	playback, err := GetPlaybackState()
	if err != nil {
		return err
	}
	if playback.Item.URI == "" {
		return fmt.Errorf("nothing currently playing")
	}

	playlists, err := UserPlaylistsRaw(50)
	if err != nil {
		return err
	}
	if len(playlists) == 0 {
		return fmt.Errorf("no playlists found")
	}

	q := strings.ToLower(query)
	var matched *PlaylistItem
	for i, p := range playlists {
		if strings.ToLower(p.Name) == q {
			matched = &playlists[i]
			break
		}
	}
	if matched == nil {
		for i, p := range playlists {
			if strings.Contains(strings.ToLower(p.Name), q) {
				matched = &playlists[i]
				break
			}
		}
	}
	if matched == nil {
		return fmt.Errorf("no playlist matching %q", query)
	}

	body, _ := json.Marshal(map[string][]string{"uris": {playback.Item.URI}})
	resp, err := makeSpotifyRequest("POST", "/playlists/"+matched.ID+"/tracks", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to add to playlist: %s", resp.Status)
	}

	ui.Successf("added \"%s\" to %s", playback.Item.Name, matched.Name)
	return nil
}

// ArtistAlbumsRaw returns albums for the given artist ID without printing.
func ArtistAlbumsRaw(artistID string) ([]AlbumItem, error) {
	endpoint := fmt.Sprintf("/artists/%s/albums?include_groups=album,single&limit=9", artistID)
	resp, err := makeSpotifyRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get artist albums: %s", resp.Status)
	}

	var result artistAlbumsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
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
