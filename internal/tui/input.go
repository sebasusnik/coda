package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sebasusnik/coda/internal/client"
)

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits regardless of mode
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// Command mode: typing a command after pressing ':'
	if m.commandMode {
		switch msg.String() {
		case "enter":
			cmd := strings.TrimSpace(m.input)
			m.input = ""
			m.commandMode = false
			return m, execInput(cmd)
		case "esc":
			m.input = ""
			m.status = ""
			m.commandMode = false
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}
		return m, nil
	}

	// Results view: number keys play, arrows scroll, esc goes back
	if m.showResults {
		visible := resultsVisible(m.height)
		switch msg.String() {
		case "esc", "q":
			m.showResults = false
			m.resultsOffset = 0
			m.status = ""
		case ":":
			m.commandMode = true
			m.input = ""
			m.status = ""
		case "up", "k":
			if m.resultsOffset > 0 {
				m.resultsOffset--
			}
		case "down", "j":
			if m.resultsOffset+visible < len(m.results) {
				m.resultsOffset++
			}
		default:
			if k := msg.String(); len(k) == 1 && k >= "1" && k <= "9" {
				// key 1-9 maps to offset + (key-1)
				idx := m.resultsOffset + int(k[0]-'1')
				if idx < len(m.results) {
					m.showResults = false
					m.resultsOffset = 0
					if m.resultsMode == "add" {
						return m, addResult(m.results[idx])
					}
					return m, playResult(m.results[idx])
				}
			}
		}
		return m, nil
	}

	// Normal mode: single-key shortcuts
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case " ":
		return m, runAction(client.Toggle, "toggled")
	case "n":
		return m, runAction(client.Next, "→ next")
	case "p":
		return m, runAction(client.Previous, "← prev")
	case "l":
		return m, runAction(client.Like, "♥ liked")
	case "s":
		return m, runAction(client.ToggleShuffle, "shuffle toggled")
	case "r":
		return m, runAction(client.CycleRepeat, "repeat cycled")
	case "<":
		return m, func() tea.Msg {
			if err := silently(func() error { return client.Seek("-10") }); err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			return cmdDoneMsg{"seek -10s"}
		}
	case ">":
		return m, func() tea.Msg {
			if err := silently(func() error { return client.Seek("+10") }); err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			return cmdDoneMsg{"seek +10s"}
		}
	case ":":
		m.commandMode = true
		m.input = ""
		m.status = ""
	}

	return m, nil
}

func execInput(input string) tea.Cmd {
	if input == "" {
		return nil
	}
	parts := strings.Fields(input)
	switch parts[0] {
	case "q", "quit":
		return tea.Quit
	case "toggle":
		return runAction(client.Toggle, "toggled")
	case "pause":
		return runAction(client.Pause, "paused")
	case "play":
		return runAction(client.SmartResume, "playing")
	case "next", "n":
		return runAction(client.Next, "→ next")
	case "prev", "p":
		return runAction(client.Previous, "← prev")
	case "like", "l":
		return runAction(client.Like, "♥ liked")
	case "shuffle", "s":
		return runAction(client.ToggleShuffle, "shuffle toggled")
	case "repeat", "r":
		return runAction(client.CycleRepeat, "repeat cycled")
	case "album":
		return runAction(client.AlbumMode, "playing album")
	case "radio":
		return runAction(client.RadioMode, "radio started")
	case "artist":
		return func() tea.Msg {
			playback, err := client.GetPlaybackState()
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(playback.Item.Artists) == 0 {
				return cmdDoneMsg{"no artist info available"}
			}
			artist := playback.Item.Artists[0]
			albums, err := client.ArtistAlbumsRaw(artist.ID)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(albums) == 0 {
				return cmdDoneMsg{"no albums found for " + artist.Name}
			}
			results := make([]searchResult, len(albums))
			for i, a := range albums {
				artists := make([]string, len(a.Artists))
				for j, ar := range a.Artists {
					artists[j] = ar.Name
				}
				results[i] = searchResult{
					kind:    kindAlbum,
					name:    a.Name,
					sub:     strings.Join(artists, ", "),
					playURI: "spotify:album:" + a.ID,
				}
			}
			return searchResultsMsg{results: results, query: artist.Name, label: "artist", mode: "play"}
		}
	case "addto":
		if len(parts) < 2 {
			return func() tea.Msg { return cmdDoneMsg{"addto: playlist name required"} }
		}
		name := strings.Join(parts[1:], " ")
		return func() tea.Msg {
			if err := silently(func() error { return client.AddCurrentToPlaylist(name) }); err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			return cmdDoneMsg{"added to " + name}
		}
	case "status":
		return func() tea.Msg { return cmdDoneMsg{"status is shown in the player"} }
	case "queue":
		return fetchQueue()
	case "liked":
		return func() tea.Msg {
			items, err := client.LikedTracksRaw(9)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(items) == 0 {
				return cmdDoneMsg{"no liked tracks"}
			}
			results := make([]searchResult, len(items))
			for i, item := range items {
				artists := make([]string, len(item.Track.Artists))
				for j, a := range item.Track.Artists {
					artists[j] = a.Name
				}
				results[i] = searchResult{
					kind:    kindTrack,
					name:    item.Track.Name,
					sub:     strings.Join(artists, ", "),
					playURI: item.Track.URI,
				}
			}
			return searchResultsMsg{results: results, query: "", label: "liked", mode: "play"}
		}
	case "playlists":
		return func() tea.Msg {
			items, err := client.UserPlaylistsRaw(9)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(items) == 0 {
				return cmdDoneMsg{"no playlists found"}
			}
			results := make([]searchResult, len(items))
			for i, p := range items {
				results[i] = searchResult{
					kind:    kindPlaylist,
					name:    p.Name,
					sub:     p.Owner.DisplayName,
					playURI: "spotify:playlist:" + p.ID,
				}
			}
			return searchResultsMsg{results: results, query: "", label: "playlists", mode: "play"}
		}
	case "recent":
		return func() tea.Msg {
			items, err := client.RecentlyPlayedRaw(9)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(items) == 0 {
				return cmdDoneMsg{"no recently played tracks"}
			}
			results := make([]searchResult, len(items))
			for i, item := range items {
				artists := make([]string, len(item.Track.Artists))
				for j, a := range item.Track.Artists {
					artists[j] = a.Name
				}
				results[i] = searchResult{
					kind:    kindTrack,
					name:    item.Track.Name,
					sub:     strings.Join(artists, ", "),
					playURI: item.Track.URI,
				}
			}
			return searchResultsMsg{results: results, query: "", label: "recent", mode: "play"}
		}
	case "add":
		if len(parts) < 2 {
			return func() tea.Msg { return cmdDoneMsg{"add: query required"} }
		}
		query := strings.Join(parts[1:], " ")
		return func() tea.Msg {
			items, err := client.SearchTracksRaw(query)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(items) == 0 {
				return cmdDoneMsg{"no tracks found"}
			}
			results := make([]searchResult, len(items))
			for i, t := range items {
				artists := make([]string, len(t.Artists))
				for j, a := range t.Artists {
					artists[j] = a.Name
				}
				results[i] = searchResult{
					kind:    kindTrack,
					name:    t.Name,
					sub:     strings.Join(artists, ", "),
					playURI: t.URI,
				}
			}
			return searchResultsMsg{results: results, query: query, label: "add to queue", mode: "add"}
		}
	case "search":
		if len(parts) < 2 {
			return func() tea.Msg { return cmdDoneMsg{"search: query required"} }
		}
		kind := "track"
		playFirst := false
		var queryParts []string
		for _, p := range parts[1:] {
			switch p {
			case "-a":
				kind = "album"
			case "-pl":
				kind = "playlist"
			case "-p":
				playFirst = true
			default:
				queryParts = append(queryParts, p)
			}
		}
		if len(queryParts) == 0 {
			return func() tea.Msg { return cmdDoneMsg{"search: query required"} }
		}
		query := strings.Join(queryParts, " ")
		if playFirst {
			return doSearchAndPlayFirst(kind, query)
		}
		return doSearch(kind, query)
	case "vol":
		if len(parts) < 2 {
			return func() tea.Msg { return cmdDoneMsg{"vol: argument required (0-100, up, down)"} }
		}
		arg := parts[1]
		return func() tea.Msg {
			if err := silently(func() error { return client.SetVolume(arg) }); err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			return cmdDoneMsg{"vol " + arg}
		}
	case "seek":
		if len(parts) < 2 {
			return func() tea.Msg { return cmdDoneMsg{"seek: argument required (seconds, +N, -N)"} }
		}
		arg := parts[1]
		return func() tea.Msg {
			if err := silently(func() error { return client.Seek(arg) }); err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			return cmdDoneMsg{"seeked"}
		}
	default:
		return func() tea.Msg { return cmdDoneMsg{"unknown: " + parts[0]} }
	}
}
