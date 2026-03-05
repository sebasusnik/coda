package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sebasusnik/coda/internal/client"
)

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
