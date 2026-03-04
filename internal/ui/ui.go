package ui

import (
	"fmt"
	"strings"
)

// ANSI color codes
const (
	reset = "\033[0m"
	bold  = "\033[1m"
	dim   = "\033[2m"
	green = "\033[32m"
	cyan  = "\033[36m"
	white = "\033[37m"
	red   = "\033[31m"
)

// Success prints a green checkmark line
func Success(msg string) {
	fmt.Printf("%s%s>%s %s\n", bold, green, reset, msg)
}

// Successf prints a formatted green checkmark line
func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

// Playing prints a now playing line in cyan
func Playing(artist, track string) {
	fmt.Printf("%s%s~>%s %s%s%s - %s\n", bold, cyan, reset, bold, artist, reset, track)
}

// Info prints a dim info line
func Info(msg string) {
	fmt.Printf("%s%s::%s %s\n", dim, white, reset, msg)
}

// Infof prints a formatted dim info line
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Error prints a red error line
func Error(msg string) {
	fmt.Printf("%s%s!!%s %s\n", bold, red, reset, msg)
}

// Errorf prints a formatted red error line
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Header prints a bold section header
func Header(msg string) {
	fmt.Printf("\n%s%s%s\n", bold, msg, reset)
}

// Track prints a numbered track row in a search list
func Track(n int, artist, name, album string) {
	fmt.Printf("  %s%s%2d.%s  %-40s %s%s%s\n",
		bold, white, n, reset,
		artist+" - "+name,
		dim, album, reset)
}

// Status prints playback status
func Status(playing bool, artist, track, album, progress string) {
	state := dim + white + "paused" + reset
	if playing {
		state = bold + cyan + "playing" + reset
	}
	fmt.Printf("  %s\n", state)
	fmt.Printf("  %s%s%s - %s\n", bold, artist, reset, track)
	fmt.Printf("  %s%s  %s  %s%s\n", dim, album, progress, reset, "")
}

// Device prints a device row
func Device(n int, name, kind string, active, preferred bool) {
	markers := ""
	if active {
		markers += "  " + bold + cyan + "* active" + reset
	}
	if preferred {
		markers += "  " + bold + green + "+ preferred" + reset
	}
	fmt.Printf("  %s%s%2d.%s  %-30s %s[%s]%s%s\n",
		bold, white, n, reset,
		name,
		dim, kind, reset,
		markers)
}

// DeviceLegend prints the legend for device markers
func DeviceLegend() {
	fmt.Printf("\n  %s%s*%s active   %s%s+%s preferred\n",
		bold, cyan, reset,
		bold, green, reset)
}

// Prompt prints an inline prompt
func Prompt(msg string) {
	fmt.Printf("  %s%s%s ", bold, msg, reset)
}

// Separator prints a dim horizontal rule
func Separator() {
	fmt.Printf("%s%s%s\n", dim, "----------------------------------------", reset)
}

// Volume prints a volume bar
// e.g.:  > volume  42%  [████████░░░░░░░░░░░░]
func Volume(percent int) {
	filled := percent / 5
	empty := 20 - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	fmt.Printf("%s%s>%s volume  %d%%  [%s]\n", bold, green, reset, percent, bar)
}

// Album prints a numbered album row in a search list
func Album(n int, artists, name string) {
	fmt.Printf("  %s%s%2d.%s  %-40s %s%s%s\n",
		bold, white, n, reset,
		artists+" - "+name,
		dim, "album", reset)
}

// Playlist prints a numbered playlist row in a search list
func Playlist(n int, owner, name string) {
	fmt.Printf("  %s%s%2d.%s  %-40s %s%s%s\n",
		bold, white, n, reset,
		name,
		dim, "by "+owner, reset)
}
