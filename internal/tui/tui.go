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

// --- styles ---

var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	boldCyan  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	boldGreen = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	dim       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	bold      = lipgloss.NewStyle().Bold(true)
)

// --- messages ---

type progressTickMsg struct{}
type syncTickMsg struct{}
type playbackMsg struct{ state *client.PlaybackState }
type errMsg struct{ err error }
type cmdDoneMsg struct{ text string }

// --- model ---

type model struct {
	playback    *client.PlaybackState
	input       string
	status      string
	loading     bool
	commandMode bool // true when the user pressed ':' to type a command
	width       int
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

// --- view ---

func progressBar(progress, duration, width int) string {
	if duration <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(progress) / float64(duration) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
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
	if m.loading {
		return "\n  " + dim.Render("connecting...") + "\n"
	}

	if m.playback == nil {
		return "\n  " + dim.Render("nothing playing") + "\n"
	}

	pb := m.playback

	// Clamp progress for display
	progress := pb.Progress
	if pb.Item.DurationMs > 0 && progress > pb.Item.DurationMs {
		progress = pb.Item.DurationMs
	}

	// Inner content width: terminal width minus box borders/padding
	// Box adds 2 (borders) + 2 (padding each side) = 6 chars overhead
	innerWidth := m.width - 6
	if innerWidth < 20 {
		innerWidth = 20
	}
	if innerWidth > 60 {
		innerWidth = 60
	}

	// State indicator
	stateStr := dim.Render("⏸ paused")
	if pb.IsPlaying {
		stateStr = boldGreen.Render("▶ playing")
	}

	// Shuffle / repeat
	shuffleStr := "shuffle off"
	if pb.ShuffleState {
		shuffleStr = "shuffle on"
	}
	repeatStr := "repeat: " + pb.RepeatState

	// Progress bar — leave room for the time string "  0:00 / 0:00"
	timeStr := fmtMs(progress) + " / " + fmtMs(pb.Item.DurationMs)
	barWidth := innerWidth - len(timeStr) - 2
	if barWidth < 4 {
		barWidth = 4
	}
	bar := progressBar(progress, pb.Item.DurationMs, barWidth)

	content := strings.Join([]string{
		boldCyan.Render(joinArtists(pb.Item.Artists)) + bold.Render("  —  "+pb.Item.Name),
		dim.Render(pb.Item.Album.Name) + "   " + stateStr,
		"",
		bar + "  " + dim.Render(timeStr),
		dim.Render(fmt.Sprintf("vol %d  ·  %s  ·  %s", pb.Device.Volume, shuffleStr, repeatStr)),
	}, "\n")

	box := boxStyle.Render(content)

	// Input line — ':' prefix in command mode, hint in normal mode
	var inputLine string
	if m.commandMode {
		inputLine = boldGreen.Render(":") + " " + m.input + "▋"
	} else {
		inputLine = dim.Render("press : to type a command")
	}

	// Status line (only shown when non-empty)
	statusLine := ""
	if m.status != "" {
		statusLine = "\n  " + dim.Render(m.status)
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
