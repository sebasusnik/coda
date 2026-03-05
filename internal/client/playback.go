package client

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebasusnik/coda/internal/device"
	"github.com/sebasusnik/coda/internal/ui"
)

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

// SmartResume resumes playback if something is paused, or plays the most
// recently played track if nothing is active.
func SmartResume() error {
	_, err := GetPlaybackState()
	if err == nil {
		return Resume()
	}
	// Nothing playing — fall back to most recently played track
	items, err := RecentlyPlayedRaw(1)
	if err != nil || len(items) == 0 {
		return fmt.Errorf("nothing to play")
	}
	track := items[0].Track
	deviceID := device.ResolveDeviceID()
	ui.Playing(artistNames(track.Artists), track.Name)
	return playTrack(track.URI, deviceID)
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

	likeBody, _ := json.Marshal(map[string][]string{"ids": {current.Item.ID}})
	likeResp, err := makeSpotifyRequest("PUT", "/me/tracks", likeBody)
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

	if !playback.Device.SupportsVolume {
		return fmt.Errorf("this device does not support remote volume control")
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

func formatMs(ms int) string {
	secs := ms / 1000
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

func Seek(input string) error {
	playback, err := GetPlaybackState()
	if err != nil {
		return err
	}

	progressMs := playback.Progress
	durationMs := playback.Item.DurationMs

	var targetMs int
	if strings.HasPrefix(input, "+") {
		n, err := strconv.Atoi(input[1:])
		if err != nil {
			return fmt.Errorf("invalid seek argument: %s", input)
		}
		targetMs = progressMs + n*1000
	} else if strings.HasPrefix(input, "-") {
		n, err := strconv.Atoi(input[1:])
		if err != nil {
			return fmt.Errorf("invalid seek argument: %s", input)
		}
		targetMs = progressMs - n*1000
	} else {
		n, err := strconv.Atoi(input)
		if err != nil {
			return fmt.Errorf("invalid seek argument: %s", input)
		}
		targetMs = n * 1000
	}

	if targetMs < 0 {
		targetMs = 0
	}
	if targetMs > durationMs {
		targetMs = durationMs
	}

	endpoint := fmt.Sprintf("/me/player/seek?position_ms=%d", targetMs)
	if playback.Device.ID != "" {
		endpoint += "&device_id=" + playback.Device.ID
	}

	seekResp, err := makeSpotifyRequest("PUT", endpoint, nil)
	if err != nil {
		return err
	}
	defer seekResp.Body.Close()

	if !isSuccess(seekResp.StatusCode) {
		return fmt.Errorf("failed to seek: %s", seekResp.Status)
	}

	ui.Successf("seeked to %s", formatMs(targetMs))
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
