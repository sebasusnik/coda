package device

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/sebasusnik/coda/internal/auth"
	"github.com/sebasusnik/coda/internal/config"
	"github.com/sebasusnik/coda/internal/ui"
)

const spotifyBaseURL = "https://api.spotify.com/v1"

type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	IsActive bool   `json:"is_active"`
}

type devicesResponse struct {
	Devices []Device `json:"devices"`
}

func makeRequest(token, method, endpoint string, body []byte) (*http.Response, error) {
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

func List() ([]Device, error) {
	token, err := auth.GetValidToken()
	if err != nil {
		return nil, err
	}

	resp, err := makeRequest(token, "GET", "/me/player/devices", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch devices: %s", resp.Status)
	}

	var dr devicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, err
	}

	return dr.Devices, nil
}

func printDeviceList(devices []Device, cfg *config.Config) {
	for i, d := range devices {
		preferred := cfg != nil && cfg.PreferredDevice == d.ID
		ui.Device(i+1, d.Name, d.Type, d.IsActive, preferred)
	}
	ui.DeviceLegend()
}

func PrintDevices() error {
	devices, err := List()
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		ui.Info("no active Spotify devices found")
		ui.Info("make sure Spotify is open on at least one device")
		return nil
	}

	cfg, _ := config.Load()

	ui.Header("devices")
	printDeviceList(devices, cfg)
	return nil
}

// Use lists available devices interactively and lets the user pick one by
// number. If name is non-empty it skips the prompt and matches by name
// directly (used internally by setup).
func Use(name string) error {
	devices, err := List()
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		return fmt.Errorf("no active Spotify devices found. Make sure Spotify is open on at least one device")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("not authenticated. Run 'coda auth' first")
	}

	var matched *Device

	if name != "" {
		// Called programmatically — match by name
		for i, d := range devices {
			if d.Name == name {
				matched = &devices[i]
				break
			}
		}
		if matched == nil {
			return fmt.Errorf("device %q not found", name)
		}
	} else {
		// Interactive — show list and prompt
		ui.Header("devices")
		printDeviceList(devices, cfg)

		ui.Prompt(fmt.Sprintf("select device (1-%d):", len(devices)))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > len(devices) {
			return fmt.Errorf("invalid selection %q -- enter a number between 1 and %d", input, len(devices))
		}
		matched = &devices[n-1]
	}

	cfg.PreferredDevice = matched.ID
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save preferred device: %v", err)
	}

	// Transfer playback to the selected device
	token, err := auth.GetValidToken()
	if err != nil {
		return err
	}
	transferBody, _ := json.Marshal(map[string]interface{}{
		"device_ids": []string{matched.ID},
		"play":       true,
	})
	transferResp, err := makeRequest(token, "PUT", "/me/player", transferBody)
	if err != nil {
		return fmt.Errorf("preferred device saved but failed to transfer playback: %v", err)
	}
	defer transferResp.Body.Close()

	if transferResp.StatusCode >= 300 {
		return fmt.Errorf("preferred device saved but transfer failed: %s", transferResp.Status)
	}

	ui.Successf("switched to \"%s\"", matched.Name)
	return nil
}

// ResolveDeviceID returns the preferred device ID if set, otherwise falls back
// to the currently active device. Returns an empty string if nothing is found.
func ResolveDeviceID() string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}

	if cfg.PreferredDevice != "" {
		return cfg.PreferredDevice
	}

	devices, err := List()
	if err != nil {
		return ""
	}

	for _, d := range devices {
		if d.IsActive {
			return d.ID
		}
	}

	return ""
}
