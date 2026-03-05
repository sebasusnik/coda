package main

import (
	"fmt"
	"log"
	"os"
	"strings"

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
		Usage:                "Spotify CLI controller",
		Version:              fmt.Sprintf("%s (%s) built %s", version, commit, buildTime),
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:  "auth",
				Usage: "Authenticate with Spotify",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "headless",
						Usage: "Use headless authentication",
					},
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Force re-authentication even if already authenticated",
					},
				},
				Action: func(c *cli.Context) error {
					return auth.Authenticate(c.Bool("headless"), c.Bool("force"))
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
						return client.SmartResume()
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
				Name:      "addto",
				Usage:     "Add the current track to a playlist",
				ArgsUsage: "<playlist name>",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return fmt.Errorf("playlist name required")
					}
					return client.AddCurrentToPlaylist(strings.Join(c.Args().Slice(), " "))
				},
			},
			{
				Name:  "liked",
				Usage: "Show your liked/saved tracks",
				Action: func(c *cli.Context) error {
					return client.LikedTracks()
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
				Name:      "seek",
				Usage:     "Seek to a position in the current track",
				ArgsUsage: "<seconds>",
				Action: func(c *cli.Context) error {
					if c.NArg() == 0 {
						return fmt.Errorf("position argument required (seconds, +N, or -N)")
					}
					return client.Seek(c.Args().First())
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
				Name:  "completion",
				Usage: "Print shell completion script",
				Subcommands: []*cli.Command{
					{
						Name:  "bash",
						Usage: "Print bash completion setup",
						Action: func(c *cli.Context) error {
							fmt.Println("# Add to ~/.bashrc or ~/.bash_profile:")
							fmt.Println("complete -o nospace -C coda coda")
							return nil
						},
					},
					{
						Name:  "zsh",
						Usage: "Print zsh completion setup",
						Action: func(c *cli.Context) error {
							fmt.Print(`#compdef coda

_coda() {
  local state

  _arguments \
    '1: :->command' \
    '*: :->args'

  case $state in
    command)
      local commands=(
        'auth:Authenticate with Spotify'
        'search:Search for tracks, albums, or playlists'
        'play:Resume playback or play track by number'
        'pause:Pause playback'
        'toggle:Toggle play/pause'
        'next:Skip to next track'
        'prev:Go to previous track'
        'status:Show current playback status'
        'queue:Show the current playback queue'
        'recent:Show recently played tracks'
        'liked:Show your liked/saved tracks'
        'add:Add a track to the queue'
        'addto:Add the current track to a playlist'
        'like:Like the currently playing track'
        'vol:Set volume (0-100, up, down)'
        'seek:Seek to a position in the current track'
        'shuffle:Toggle shuffle'
        'repeat:Cycle repeat mode'
        'radio:Start radio mode based on current track'
        'album:Play entire album of current track'
        'ui:Start the interactive player UI'
        'start:Start the librespot service'
        'stop:Stop the librespot service'
        'device:Device management'
        'completion:Print shell completion script'
        'install:Install coda to /usr/local/bin'
      )
      _describe 'command' commands
      ;;
    args)
      case $words[2] in
        auth)
          _arguments \
            '--headless[Use headless authentication]' \
            '--force[Force re-authentication]' \
            '-f[Force re-authentication]'
          ;;
        search)
          _arguments \
            '--play[Play the first result immediately]' \
            '-p[Play the first result immediately]' \
            '--a[Search albums instead of tracks]' \
            '--pl[Search playlists instead of tracks]'
          ;;
        vol)
          local vol_args=('up:Increase volume' 'down:Decrease volume')
          _describe 'volume' vol_args
          ;;
        device)
          local device_cmds=(
            'setup:Install librespot and register this machine'
            'list:List all available Spotify Connect devices'
            'use:Set the preferred Spotify Connect device'
          )
          _describe 'device command' device_cmds
          ;;
        completion)
          local completion_cmds=(
            'bash:Print bash completion setup'
            'zsh:Print zsh completion setup'
            'fish:Print fish completion setup'
          )
          _describe 'shell' completion_cmds
          ;;
      esac
      ;;
  esac
}

_coda "$@"
`)
							return nil
						},
					},
					{
						Name:  "fish",
						Usage: "Print fish completion setup",
						Action: func(c *cli.Context) error {
							fmt.Println("# Save to ~/.config/fish/completions/coda.fish")
							cmds := []struct{ name, desc string }{
								{"auth", "Authenticate with Spotify"},
								{"search", "Search for tracks, albums, or playlists"},
								{"play", "Resume playback or play track by number"},
								{"pause", "Pause playback"},
								{"toggle", "Toggle play/pause"},
								{"next", "Skip to next track"},
								{"prev", "Go to previous track"},
								{"status", "Show current playback status"},
								{"queue", "Show the current playback queue"},
								{"recent", "Show recently played tracks"},
								{"liked", "Show your liked/saved tracks"},
								{"add", "Add a track to the queue"},
								{"addto", "Add the current track to a playlist"},
								{"like", "Like the currently playing track"},
								{"vol", "Set volume (0-100, up, down)"},
								{"seek", "Seek to a position in the current track"},
								{"shuffle", "Toggle shuffle"},
								{"repeat", "Cycle repeat mode"},
								{"radio", "Start radio mode based on current track"},
								{"album", "Play entire album of current track"},
								{"ui", "Start the interactive player UI"},
								{"device", "Device management"},
								{"completion", "Print shell completion script"},
								{"install", "Install coda to /usr/local/bin"},
							}
							fmt.Println("complete -c coda -f")
							for _, cmd := range cmds {
								fmt.Printf("complete -c coda -n '__fish_use_subcommand' -a '%s' -d '%s'\n", cmd.name, cmd.desc)
							}
							return nil
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
