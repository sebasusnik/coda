package tui

import (
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sebasusnik/coda/internal/client"
)

// stdoutMu guards the os.Stdout redirect used by silently().
// bubbletea runs Cmds concurrently, so the mutex prevents races.
var stdoutMu sync.Mutex

func silently(fn func() error) error {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fn()
	}
	old := os.Stdout
	os.Stdout = devNull
	runErr := fn()
	os.Stdout = old
	_ = devNull.Close()
	return runErr
}

func runAction(fn func() error, successMsg string) tea.Cmd {
	return func() tea.Msg {
		if err := silently(fn); err != nil {
			return cmdDoneMsg{"error: " + err.Error()}
		}
		return cmdDoneMsg{successMsg}
	}
}

func playResult(r searchResult) tea.Cmd {
	return func() tea.Msg {
		var err error
		if r.kind == kindTrack {
			err = client.PlayURI(r.playURI)
		} else {
			err = client.PlayContextURI(r.playURI)
		}
		if err != nil {
			return cmdDoneMsg{"error: " + err.Error()}
		}
		return cmdDoneMsg{"playing: " + r.name}
	}
}

func addResult(r searchResult) tea.Cmd {
	return func() tea.Msg {
		if err := client.AddToQueue(r.playURI); err != nil {
			return cmdDoneMsg{"error: " + err.Error()}
		}
		return cmdDoneMsg{"queued: " + r.name}
	}
}

func doSearch(kind, query string) tea.Cmd {
	return func() tea.Msg {
		switch kind {
		case "album":
			items, err := client.SearchAlbumsRaw(query)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			results := make([]searchResult, len(items))
			for i, a := range items {
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
			return searchResultsMsg{results: results, query: query, label: "search", mode: "play"}

		case "playlist":
			items, err := client.SearchPlaylistsRaw(query)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
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
			return searchResultsMsg{results: results, query: query, label: "search", mode: "play"}

		default: // tracks
			items, err := client.SearchTracksRaw(query)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			results := make([]searchResult, len(items))
			for i, t := range items {
				artists := make([]string, len(t.Artists))
				for j, ar := range t.Artists {
					artists[j] = ar.Name
				}
				results[i] = searchResult{
					kind:    kindTrack,
					name:    t.Name,
					sub:     strings.Join(artists, ", "),
					playURI: t.URI,
				}
			}
			return searchResultsMsg{results: results, query: query, label: "search", mode: "play"}
		}
	}
}

func fetchQueue() tea.Cmd {
	return func() tea.Msg {
		_, queue, err := client.QueueRaw()
		if err != nil {
			return cmdDoneMsg{"error: " + err.Error()}
		}
		if len(queue) == 0 {
			return cmdDoneMsg{"queue is empty"}
		}
		limit := len(queue)
		if limit > 9 {
			limit = 9
		}
		results := make([]searchResult, limit)
		for i, t := range queue[:limit] {
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
		return searchResultsMsg{results: results, query: "", label: "queue"}
	}
}

func doSearchAndPlayFirst(kind, query string) tea.Cmd {
	return func() tea.Msg {
		var r searchResult
		switch kind {
		case "album":
			items, err := client.SearchAlbumsRaw(query)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(items) == 0 {
				return cmdDoneMsg{"no results found"}
			}
			r = searchResult{kind: kindAlbum, name: items[0].Name, playURI: "spotify:album:" + items[0].ID}
		case "playlist":
			items, err := client.SearchPlaylistsRaw(query)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(items) == 0 {
				return cmdDoneMsg{"no results found"}
			}
			r = searchResult{kind: kindPlaylist, name: items[0].Name, playURI: "spotify:playlist:" + items[0].ID}
		default:
			items, err := client.SearchTracksRaw(query)
			if err != nil {
				return cmdDoneMsg{"error: " + err.Error()}
			}
			if len(items) == 0 {
				return cmdDoneMsg{"no results found"}
			}
			r = searchResult{kind: kindTrack, name: items[0].Name, playURI: items[0].URI}
		}
		var err error
		if r.kind == kindTrack {
			err = client.PlayURI(r.playURI)
		} else {
			err = client.PlayContextURI(r.playURI)
		}
		if err != nil {
			return cmdDoneMsg{"error: " + err.Error()}
		}
		return cmdDoneMsg{"playing: " + r.name}
	}
}
