# coda roadmap

Ideas for future features, roughly ordered by effort.

---

## quick wins

| idea | description | spotify endpoint |
|------|-------------|-----------------|
| `coda add <query>` | add a track to the queue without interrupting playback | `POST /me/player/queue` |
| `coda recent` | show recently played tracks | `GET /me/player/recently-played` |
| `coda seek <seconds>` | jump to a position in the current track | `PUT /me/player/seek` |

---

## medium effort

### playlist browsing in the TUI
- `:playlists` lists your saved playlists
- pick one by number to start playing it
- endpoint: `GET /me/playlists`

### add to playlist
- `:addto <playlist name>` saves the currently playing track to one of your playlists
- endpoints: `GET /me/playlists` + `POST /playlists/{id}/tracks`

### shell completions
- zsh / fish / bash completions for all subcommands
- `urfave/cli` can auto-generate these

---

## bigger features

### artist view
- `:artist` from the TUI player opens a results view of the current artist's albums
- pick one by number to play it
- endpoints: `GET /artists/{id}/albums`

### liked songs browser
- `:liked` lets you browse and play from your saved tracks library
- supports scrolling and number-to-play like the search results view
- endpoint: `GET /me/tracks`

---

## pick next

`coda add` is probably the best next step — single API call, works as both a CLI command and a TUI command (`:add some song`), and it's something you'd use constantly while the player is running.
