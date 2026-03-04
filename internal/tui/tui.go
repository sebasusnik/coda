package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sebasusnik/coda/internal/client"
)

const (
	progressTickInterval = 1 * time.Second
	syncTickInterval     = 10 * time.Second
)

// --- styles — Catppuccin Mocha ---
// https://github.com/catppuccin/catppuccin

const (
	cMauve    = "#cba6f7" // purple — track name, progress filled, prompt
	cLavender = "#b4befe" // blue-purple — artist
	cGreen    = "#a6e3a1" // green — playing state, success status
	cRed      = "#f38ba8" // red — error status
	cPeach    = "#fab387" // orange — paused state
	cOverlay1 = "#7f849c" // mid-gray — album, secondary text
	cSurface1 = "#45475a" // dark — progress bar empty, volume empty
	cSurface2 = "#585b70" // mid-dark — box border
)

var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cSurface2)).
			Padding(1, 3)

	compactBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cSurface2)).
			Padding(0, 3)

	trackStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cMauve))
	artistStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cLavender))
	albumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(cOverlay1))
	playStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cGreen))
	pauseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(cPeach))
	barOn       = lipgloss.NewStyle().Foreground(lipgloss.Color(cMauve))
	barOff      = lipgloss.NewStyle().Foreground(lipgloss.Color(cSurface1))
	dim         = lipgloss.NewStyle().Foreground(lipgloss.Color(cOverlay1))
	successSt   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cGreen))
	errorSt     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cRed))
	promptSt    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cMauve))
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

// --- commands ---

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
	devNull.Close()
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
	case ":":
		m.commandMode = true
		m.input = ""
		m.status = ""
	}

	return m, nil
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
		return runAction(client.Resume, "playing")
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
	case "status":
		return func() tea.Msg { return cmdDoneMsg{"status is shown in the player"} }
	case "queue":
		return fetchQueue()
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
		var queryParts []string
		for _, p := range parts[1:] {
			switch p {
			case "-a":
				kind = "album"
			case "-pl":
				kind = "playlist"
			default:
				queryParts = append(queryParts, p)
			}
		}
		if len(queryParts) == 0 {
			return func() tea.Msg { return cmdDoneMsg{"search: query required"} }
		}
		return doSearch(kind, strings.Join(queryParts, " "))
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
	default:
		return func() tea.Msg { return cmdDoneMsg{"unknown: " + parts[0]} }
	}
}

// resultsVisible returns how many results fit given the terminal height.
// Layout: 1 blank + box(1 border + 1 pad + 2 header + N*2 results + 1 pad + 1 border)
// + 1 blank + 1 input + 1 status + 1 blank + 1 help + 1 blank = 11 overhead lines outside results.
func resultsVisible(termHeight int) int {
	v := (termHeight - 11) / 2
	if v < 1 {
		v = 1
	}
	return v
}

// --- results view ---

func (m model) resultsView() string {
	innerWidth := m.width - 14
	if innerWidth < 30 {
		innerWidth = 30
	}
	if innerWidth > 80 {
		innerWidth = 80
	}

	kindLabel := map[resultKind]string{
		kindTrack:    "track",
		kindAlbum:    "album",
		kindPlaylist: "playlist",
	}

	visible := resultsVisible(m.height)
	end := m.resultsOffset + visible
	if end > len(m.results) {
		end = len(m.results)
	}
	page := m.results[m.resultsOffset:end]

	label := m.resultsLabel
	if label == "" {
		label = "search"
	}
	suffix := ""
	if m.resultsQuery != "" {
		suffix = "  ·  " + m.resultsQuery
	}
	if len(m.results) > visible {
		suffix = fmt.Sprintf("  ·  %s  (%d-%d of %d)", m.resultsQuery, m.resultsOffset+1, end, len(m.results))
		if m.resultsQuery == "" {
			suffix = fmt.Sprintf("  (%d-%d of %d)", m.resultsOffset+1, end, len(m.results))
		}
	}
	header := trackStyle.Render(label) + dim.Render(suffix)

	rows := make([]string, 0, len(page)*2+2)
	rows = append(rows, header, "")
	for i, r := range page {
		num := artistStyle.Render(fmt.Sprintf("%d", i+1))
		badge := dim.Render(kindLabel[r.kind])
		maxName := innerWidth - 8
		name := r.name
		if len([]rune(name)) > maxName {
			name = string([]rune(name)[:maxName-1]) + "…"
		}
		rows = append(rows,
			fmt.Sprintf("%s  %s  %s", num, name, badge),
			"   "+albumStyle.Render(r.sub),
		)
	}

	box := boxStyle.Render(strings.Join(rows, "\n"))

	// Compact mode: terminal too short — just the results box
	if m.height > 0 && m.height < 10 {
		return "\n" + box + "\n"
	}

	var inputLine string
	if m.commandMode {
		inputLine = promptSt.Render(":") + " " + m.input + "▋"
	} else {
		inputLine = dim.Render("press : to type a command")
	}

	statusLine := ""
	if m.status != "" {
		if strings.HasPrefix(m.status, "error") {
			statusLine = "\n  " + errorSt.Render(m.status)
		} else {
			statusLine = "\n  " + successSt.Render(m.status)
		}
	}

	action := "play"
	if m.resultsMode == "add" {
		action = "queue"
	}
	helpStr := fmt.Sprintf("1-9 %s · esc back · : command · q quit", action)
	if len(m.results) > visible {
		helpStr = fmt.Sprintf("1-9 %s · ↑↓ scroll · esc back · : command · q quit", action)
	}
	help := dim.Render(helpStr)
	return "\n" + box + "\n\n  " + inputLine + statusLine + "\n\n  " + help + "\n"
}

// --- view ---

// progressBar returns (filled, empty) block counts for the given progress.
func progressBar(progress, duration, width int) (int, int) {
	if duration <= 0 || width <= 0 {
		return 0, width
	}
	filled := int(float64(progress) / float64(duration) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return filled, width - filled
}

func fmtMs(ms int) string {
	if ms < 0 {
		ms = 0
	}
	s := ms / 1000
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func joinArtists(artists []client.Artist) string {
	names := make([]string, len(artists))
	for i, a := range artists {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

func (m model) View() string {
	if m.showResults {
		return m.resultsView()
	}

	if m.loading {
		return "\n  " + dim.Render("connecting...") + "\n"
	}

	if m.playback == nil {
		var inputLine string
		if m.commandMode {
			inputLine = promptSt.Render(":") + " " + m.input + "▋"
		} else {
			inputLine = dim.Render("press : to type a command")
		}
		statusLine := ""
		if m.status != "" {
			if strings.HasPrefix(m.status, "error") {
				statusLine = "\n  " + errorSt.Render(m.status)
			} else {
				statusLine = "\n  " + successSt.Render(m.status)
			}
		}
		idle := boxStyle.Render(dim.Render("nothing playing"))
		help := dim.Render("search · recent · : command · q quit")
		return "\n" + idle + "\n\n  " + inputLine + statusLine + "\n\n  " + help + "\n"
	}

	pb := m.playback

	// Clamp progress for display
	progress := pb.Progress
	if pb.Item.DurationMs > 0 && progress > pb.Item.DurationMs {
		progress = pb.Item.DurationMs
	}

	// State indicator — both strings must be the same visible width so the
	// right-aligned position stays fixed when toggling play/pause.
	var stateStr string
	if pb.IsPlaying {
		stateStr = playStyle.Render("▶ playing")
	} else {
		stateStr = pauseStyle.Render("⏸ paused ") // trailing space matches "▶ playing" width
	}

	// Shuffle / repeat
	shuffleStr := dim.Render("shuffle off")
	if pb.ShuffleState {
		shuffleStr = artistStyle.Render("shuffle on")
	}
	var repeatStr string
	switch pb.RepeatState {
	case "track":
		repeatStr = artistStyle.Render("repeat: track")
	case "context":
		repeatStr = artistStyle.Render("repeat: context")
	default:
		repeatStr = dim.Render("repeat: off")
	}

	// innerWidth grows with the terminal, minimum 30
	innerWidth := m.width - 14
	if innerWidth < 30 {
		innerWidth = 30
	}

	// Progress bar — leave room for the time string "  0:00 / 0:00"
	timeStr := fmtMs(progress) + " / " + fmtMs(pb.Item.DurationMs)
	barWidth := innerWidth - len(timeStr) - 2
	if barWidth < 4 {
		barWidth = 4
	}
	filled, empty := progressBar(progress, pb.Item.DurationMs, barWidth)
	bar := barOn.Render(strings.Repeat("█", filled)) + barOff.Render(strings.Repeat("░", empty))

	compact := m.height > 0 && m.height < 10

	// Track name left, state indicator right-aligned on the same line.
	// Truncate the track name if it would crowd out the state indicator.
	stateWidth := lipgloss.Width(stateStr)
	maxTrackRunes := innerWidth - stateWidth - 1
	trackName := pb.Item.Name
	if maxTrackRunes > 0 && len([]rune(trackName)) > maxTrackRunes {
		trackName = string([]rune(trackName)[:maxTrackRunes-1]) + "…"
	}
	trackRendered := trackStyle.Render(trackName)
	pad := innerWidth - lipgloss.Width(trackRendered) - stateWidth
	if pad < 1 {
		pad = 1
	}
	trackLine := trackRendered + strings.Repeat(" ", pad) + stateStr

	content := strings.Join([]string{
		trackLine,
		artistStyle.Render(joinArtists(pb.Item.Artists)) + "  " + albumStyle.Render("· "+pb.Item.Album.Name),
		"",
		bar + "  " + dim.Render(timeStr),
		dim.Render(fmt.Sprintf("vol %d", pb.Device.Volume)) + "  ·  " + shuffleStr + "  ·  " + repeatStr,
	}, "\n")

	var box string
	if compact {
		box = compactBoxStyle.Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Center, box)
	}
	box = boxStyle.Render(content)

	// Player only — no chrome yet
	if m.height < 15 {
		return "\n" + box + "\n"
	}

	// Input line — ':' prefix in command mode, hint in normal mode
	var inputLine string
	if m.commandMode {
		inputLine = promptSt.Render(":") + " " + m.input + "▋"
	} else {
		inputLine = dim.Render("press : to type a command")
	}

	// Status line (only shown when non-empty)
	statusLine := ""
	if m.status != "" {
		if strings.HasPrefix(m.status, "error") {
			statusLine = "\n  " + errorSt.Render(m.status)
		} else {
			statusLine = "\n  " + successSt.Render(m.status)
		}
	}

	help := dim.Render("spc toggle · n next · p prev · l like · s shuffle · r repeat · : command · q quit")

	return "\n" + box + "\n\n  " + inputLine + statusLine + "\n\n  " + help + "\n"
}

// --- entry point ---

func Start() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
