package client

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func codaConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(homeDir, ".config", "coda")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func saveJSON(filename string, v interface{}) {
	dir, err := codaConfigDir()
	if err != nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, filename), data, 0600)
}

func saveSearchResults(results []Track) {
	saveJSON("last_search.json", results)
}

func saveAlbumResults(results []AlbumItem) {
	saveJSON("last_search_albums.json", results)
}

func savePlaylistResults(results []PlaylistItem) {
	saveJSON("last_search_playlists.json", results)
}

func loadSearchResults() ([]Track, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(homeDir, ".config", "coda", "last_search.json"))
	if err != nil {
		return nil, err
	}

	var results []Track
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return results, nil
}
