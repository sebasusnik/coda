package client

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/sebasusnik/coda/internal/device"
	"github.com/sebasusnik/coda/internal/ui"
)

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
