package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sebasusnik/coda/internal/client"
)

const (
	progressTickInterval = 1 * time.Second
	syncTickInterval     = 10 * time.Second
)

// --- messages ---

type progressTickMsg struct{}
type syncTickMsg struct{}
type playbackMsg struct{ state *client.PlaybackState }
type errMsg struct{ err error }
type cmdDoneMsg struct{ text string }
type searchResultsMsg struct {
	results []searchResult
	query   string
	label   string // header label, e.g. "search" or "queue"
	mode    string // "play" or "add"
}

// --- search result type ---

type resultKind int

const (
	kindTrack    resultKind = iota
	kindAlbum
	kindPlaylist
)

type searchResult struct {
	kind    resultKind
	name    string
	sub     string // artist for tracks/albums, owner for playlists
	playURI string // track URI or context URI (spotify:album:id etc.)
}

// --- model ---

type model struct {
	playback      *client.PlaybackState
	input         string
	status        string
	loading       bool
	commandMode   bool // true when the user pressed ':' to type a command
	width         int
	height        int
	results       []searchResult
	resultsQuery  string
	resultsLabel  string // "search", "queue", or "add"
	resultsMode   string // "play" or "add"
	showResults   bool
	resultsOffset int // index of first visible result
}

func initialModel() model {
	return model{loading: true}
}

// --- tick commands ---

func progressTickCmd() tea.Cmd {
	return tea.Tick(progressTickInterval, func(time.Time) tea.Msg {
		return progressTickMsg{}
	})
}

func syncTickCmd() tea.Cmd {
	return tea.Tick(syncTickInterval, func(time.Time) tea.Msg {
		return syncTickMsg{}
	})
}

func fetchPlayback() tea.Cmd {
	return func() tea.Msg {
		state, err := client.GetPlaybackState()
		if err != nil {
			return errMsg{err}
		}
		return playbackMsg{state}
	}
}

// --- init / update ---

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchPlayback(), progressTickCmd(), syncTickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case progressTickMsg:
		if m.playback != nil && m.playback.IsPlaying {
			m.playback.Progress += 1000
			// If the track ended locally, force a sync immediately
			if m.playback.Item.DurationMs > 0 && m.playback.Progress >= m.playback.Item.DurationMs {
				return m, tea.Batch(fetchPlayback(), progressTickCmd())
			}
		}
		return m, progressTickCmd()

	case syncTickMsg:
		return m, tea.Batch(fetchPlayback(), syncTickCmd())

	case playbackMsg:
		m.playback = msg.state
		m.loading = false

	case errMsg:
		m.loading = false
		m.playback = nil
		m.status = dim.Render(msg.err.Error())

	case cmdDoneMsg:
		m.status = msg.text
		// Force immediate sync after any action
		return m, fetchPlayback()

	case searchResultsMsg:
		m.results = msg.results
		m.resultsQuery = msg.query
		m.resultsLabel = msg.label
		m.resultsMode = msg.mode
		m.showResults = true
		m.resultsOffset = 0
		m.status = ""

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}
