package client

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
		ID             string `json:"id"`
		Volume         int    `json:"volume_percent"`
		SupportsVolume bool   `json:"supports_volume"`
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

type userPlaylistsResponse struct {
	Items []PlaylistItem `json:"items"`
}

type RecentlyPlayedItem struct {
	Track    Track  `json:"track"`
	PlayedAt string `json:"played_at"`
}

type recentlyPlayedResponse struct {
	Items []RecentlyPlayedItem `json:"items"`
}

type SavedTrackItem struct {
	AddedAt string `json:"added_at"`
	Track   Track  `json:"track"`
}

type savedTracksResponse struct {
	Items []SavedTrackItem `json:"items"`
}

type artistAlbumsResponse struct {
	Items []AlbumItem `json:"items"`
}
