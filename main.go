package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sebasusnik/coda/internal/auth"
	"github.com/sebasusnik/coda/internal/client"
	"github.com/sebasusnik/coda/internal/device"
	"github.com/sebasusnik/coda/internal/setup"
	"github.com/sebasusnik/coda/internal/tui"
	"github.com/urfave/cli/v2"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	app := &cli.App{
		Usage:   "Spotify CLI controller",
		Version: fmt.Sprintf("%s (%s) built %s", version, commit, buildTime),
		Commands: []*cli.Command{
			{
				Name:  "auth",
				Usage: "Authenticate with Spotify",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "headless",
						Usage: "Use headless authentication",
					},
				},
				Action: func(c *cli.Context) error {
					return auth.Authenticate(c.Bool("headless"))
				},
			},
			{
				Name:  "search",
				Usage: "Search for tracks, albums, or playlists",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "play",
						Aliases: []string{"p"},
						Usage:   "Play the first result immediately",
					},
					&cli.BoolFlag{
						Name:  "a",
						Usage: "Search albums instead of tracks",
					},
					&cli.BoolFlag{
						Name:  "pl",
						Usage: "Search playlists instead of tracks",
					},
				},
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return fmt.Errorf("search query required")
					}
					query := c.Args().First()
					play := c.Bool("play")
					switch {
					case c.Bool("a"):
						return client.SearchAlbums(query, play)
					case c.Bool("pl"):
						return client.SearchPlaylists(query, play)
					default:
						return client.SearchTracks(query, play)
					}
				},
			},
			{
				Name:  "play",
				Usage: "Resume playback, or play track by number from last search",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return client.Resume()
					}
					return client.PlayByNumber(c.Args().First())
				},
			},
			{
				Name:  "pause",
				Usage: "Pause playback",
				Action: func(c *cli.Context) error {
					return client.Pause()
				},
			},
			{
				Name:  "next",
				Usage: "Skip to next track",
				Action: func(c *cli.Context) error {
					return client.Next()
				},
			},
			{
				Name:  "prev",
				Usage: "Go to previous track",
				Action: func(c *cli.Context) error {
					return client.Previous()
				},
			},
			{
				Name:  "status",
				Usage: "Show current playback status",
				Action: func(c *cli.Context) error {
					return client.Status()
				},
			},
			{
				Name:  "queue",
				Usage: "Show the current playback queue",
				Action: func(c *cli.Context) error {
					return client.Queue()
				},
			},
			{
				Name:  "radio",
				Usage: "Start radio mode based on current track",
				Action: func(c *cli.Context) error {
					return client.RadioMode()
				},
			},
			{
				Name:  "album",
				Usage: "Play entire album of current track",
				Action: func(c *cli.Context) error {
					return client.AlbumMode()
				},
			},
			{
				Name:  "toggle",
				Usage: "Toggle play/pause",
				Action: func(c *cli.Context) error {
					return client.Toggle()
				},
			},
			{
				Name:  "recent",
				Usage: "Show recently played tracks",
				Action: func(c *cli.Context) error {
					return client.RecentlyPlayed()
				},
			},
			{
				Name:      "add",
				Usage:     "Add a track to the queue",
				ArgsUsage: "<query>",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return fmt.Errorf("search query required")
					}
					return client.AddFirstToQueue(c.Args().First())
				},
			},
			{
				Name:  "like",
				Usage: "Like the currently playing track",
				Action: func(c *cli.Context) error {
					return client.Like()
				},
			},
			{
				Name:      "vol",
				Usage:     "Set volume (0-100, up, down)",
				ArgsUsage: "<n|up|down>",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return fmt.Errorf("volume argument required (0-100, up, or down)")
					}
					return client.SetVolume(c.Args().First())
				},
			},
			{
				Name:  "shuffle",
				Usage: "Toggle shuffle",
				Action: func(c *cli.Context) error {
					return client.ToggleShuffle()
				},
			},
			{
				Name:  "repeat",
				Usage: "Cycle repeat mode (off → context → track → off)",
				Action: func(c *cli.Context) error {
					return client.CycleRepeat()
				},
			},
			{
				Name:  "ui",
				Usage: "Start the interactive player UI",
				Action: func(c *cli.Context) error {
					return tui.Start()
				},
			},
			{
				Name:  "start",
				Usage: "Start the librespot service",
				Action: func(c *cli.Context) error {
					return setup.Start()
				},
			},
			{
				Name:  "stop",
				Usage: "Stop the librespot service",
				Action: func(c *cli.Context) error {
					return setup.Stop()
				},
			},
			{
				Name:  "device",
				Usage: "Device management",
				Subcommands: []*cli.Command{
					{
						Name:  "setup",
						Usage: "Install librespot and register this machine as a device",
						Action: func(c *cli.Context) error {
							return setup.Run()
						},
					},
					{
						Name:  "list",
						Usage: "List all available Spotify Connect devices",
						Action: func(c *cli.Context) error {
							return device.PrintDevices()
						},
					},
					{
						Name:  "use",
						Usage: "Set the preferred Spotify Connect device",
						Action: func(c *cli.Context) error {
							return device.Use(c.Args().First())
						},
					},
				},
			},
			{
				Name:  "install",
				Usage: "Install coda to /usr/local/bin",
				Action: func(c *cli.Context) error {
					return setup.Install()
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
