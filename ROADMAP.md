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

### seek in the TUI
- `<` / `>` keys (or `:seek <seconds>`) to jump backward/forward in the current track
- pairs nicely with `coda seek <seconds>` on the CLI
- endpoint: `PUT /me/player/seek`

---

## pick next

**liked songs browser** — the TUI results view already handles everything, it's just one new command and one API call. `:liked` would feel like a natural part of the player.

**seek** — `<` / `>` in the TUI would make it feel complete as a player, not just a controller.

**shell completions** — makes the CLI feel polished and is mostly free with `urfave/cli`.
