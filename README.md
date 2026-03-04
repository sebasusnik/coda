# Coda

A Spotify CLI controller. Search, play and control your music from the terminal — auth and streaming happen behind the scenes.

> Spotify Premium is required for playback control via the Web API.

---

## Requirements

- Go 1.21+
- Spotify Premium account
- A Spotify app registered at [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard)

---

## Installation

### 1. Clone and build

```bash
git clone https://github.com/sebasusnik/coda
cd coda
go build -o coda .
```

### 2. Install globally

```bash
./coda install
```

This copies the binary to `/usr/local/bin` so you can run `coda` from anywhere. If it needs elevated permissions it will ask for your password via `sudo`.

### 3. Set up your Spotify app

1. Go to [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard)
2. Create a new app
3. Under **Redirect URIs** add `http://127.0.0.1:8080/callback`
4. Copy your **Client ID** and **Client Secret**

### 4. Configure credentials

Create a `.env` file at the project root:

```bash
CODA_CLIENT_ID=your_client_id_here
CODA_CLIENT_SECRET=your_client_secret_here
```

Or export them in your shell — coda will fall back to `~/.config/coda/config.json` after the first auth.

### 5. Authenticate

```bash
coda auth
# opens a browser window — log in and authorize
# for headless machines:
coda auth --headless
```

### 6. Set up a playback device

```bash
coda device setup
# > Device name (default: my-machine):
# installs librespot, registers it as a Spotify Connect device,
# sets it as your preferred device automatically
```

That's it. You're ready to use coda.

---

## Commands

### Playback

```bash
coda play                          # resume playback
coda play 1                        # play track by number from last search
coda pause                         # pause playback
coda toggle                        # toggle play/pause
coda next                          # skip to next track
coda prev                          # go to previous track
coda status                        # show what's currently playing
coda queue                         # show the current playback queue
coda radio                         # start radio based on current track
coda album                         # play the full album of the current track
```

### Search

```bash
coda search "black sabbath"        # search for tracks
coda search -p "black sabbath"     # search and immediately play the first result
coda search -a "dark side"         # search for albums
coda search -a -p "abbey road"     # search albums and play the first result
coda search -pl "workout"          # search for playlists
coda search -pl -p "chill"         # search playlists and play the first result
```

### Volume & playback state

```bash
coda vol 50                        # set volume to 50%
coda vol up                        # increase volume by 10%
coda vol down                      # decrease volume by 10%
coda shuffle                       # toggle shuffle on/off
coda repeat                        # cycle repeat: off → context → track → off
coda like                          # like the currently playing track
```

### Service

```bash
coda start                         # start the librespot service
coda stop                          # stop the librespot service
```

### Devices

```bash
coda device setup                  # install librespot and register this machine as a device
coda device list                   # list all available Spotify Connect devices
coda device use                    # interactively select a preferred device
coda device use "my-machine"       # set a specific device by name
```

### Setup

```bash
coda auth                          # authenticate with Spotify
coda auth --headless               # authenticate without a browser (for servers/Pi)
coda install                       # install coda to /usr/local/bin
```

---

## Workflow examples

```bash
# Quick play
coda search -p "pink floyd comfortably numb"

# Browse and pick
coda search "necrophagist"
coda play 1

# Browse albums and pick
coda search -a "pink floyd"
coda play                          # resumes if something was already queued

# Jump straight into a playlist
coda search -pl -p "late night lofi"

# Start a radio session from current track
coda radio

# Play the full album
coda album

# Like what's playing and set a comfortable volume
coda like
coda vol 60

# Quick playback controls
coda toggle                        # pause / resume
coda shuffle                       # toggle shuffle
coda repeat                        # cycle repeat mode

# Check what's coming up
coda queue

# Switch to a different device
coda device use
```

---

## Raspberry Pi

Since coda is a single binary, the setup is identical on a Raspberry Pi. Just build for arm64 on your Mac:

```bash
GOOS=linux GOARCH=arm64 go build -o coda-arm64 .
scp coda-arm64 pi@raspberrypi.local:~/coda
```

Then on the Pi:

```bash
./coda install
./coda auth --headless
./coda device setup
```

Your Pi will appear as a Spotify Connect device. Switch to it from your Mac with:

```bash
coda device use "raspberrypi"
```

---

## Configuration

All config is stored in `~/.config/coda/`:

| File | Contents |
|------|----------|
| `config.json` | OAuth tokens, Client ID/Secret, preferred device |
| `last_search.json` | Cached results from the last `coda search` (tracks) |
| `last_search_albums.json` | Cached results from the last `coda search -a` |
| `last_search_playlists.json` | Cached results from the last `coda search -pl` |

Librespot credentials cache is stored in `~/.cache/coda/librespot/`.

Service logs:
- **macOS**: `~/Library/Logs/coda-librespot.log`
- **Linux**: `journalctl --user -u coda-librespot.service -f`

---

## Troubleshooting

### "no active Spotify device found"
Run `coda device setup` to install and register a device on this machine, then `coda start` to make sure the service is running.

### Authentication issues
1. Make sure your Client ID and Secret are correct
2. Confirm `http://127.0.0.1:8080/callback` is in your app's redirect URIs
3. Try `coda auth --headless` if the browser flow fails

### Device not showing up after setup
Wait a few seconds and run `coda device list`. If it still doesn't appear, check the logs and try `coda stop && coda start`.

---

## License

MIT — see LICENSE for details.
