# lazytik

Watch a scrollable TikTok feed in your terminal. Navigate with the arrow keys —
TikTok's vertical swipe, but as a TUI.

> Status: early development. M0 (runnable TUI skeleton) ✅ · M1 (real feed +
> fullscreen playback) 🚧 · M2 (embedded video pane) ⬜

## How it works

lazytik is an **orchestrator**, not a from-scratch scraper or video decoder. It
wraps three battle-tested tools:

- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) — resolves TikTok feeds and stream
  URLs. No official API needed; yt-dlp tracks TikTok's changing internals for us.
- [`mpv`](https://mpv.io) — plays audio and fullscreen video in the terminal.
- [`ffmpeg`](https://ffmpeg.org) — decodes frames for the embedded video pane.

Video in the embedded pane is drawn as colored Unicode half-blocks, so it works on
any truecolor terminal — even over SSH. Each clip is downloaded to a temp file
(yt-dlp supplies the headers TikTok's CDN requires), and the **next clip is
prefetched in the background** while you watch the current one, so scrolling down is
instant. Photo/slideshow posts (no video) are skipped automatically.

## Install

```sh
go install github.com/naimroslan/lazytik@latest
```

You also need the runtime tools:

```sh
# macOS
brew install yt-dlp mpv ffmpeg
# Debian/Ubuntu
sudo apt install -y mpv ffmpeg   # yt-dlp: pipx install yt-dlp  (or the standalone binary)
```

Check your setup any time:

```sh
lazytik doctor
```

## Usage

```sh
lazytik @username             # a creator's videos
lazytik @a @b @c              # mixed feed, interleaved across creators
lazytik --shuffle @a @b @c    # shuffled mix — a personal, FYP-like feed
lazytik <url> [url...]        # explicit TikTok video URLs
```

There is no algorithmic "For You" feed: TikTok's recommendation API needs
browser-based request signing, which yt-dlp doesn't do. The shuffled multi-creator
mix above is the practical stand-in — your taste is whoever you add.

Keys: `↑`/`↓` (or `k`/`j`) next/prev · `space` pause · `q` quit.

Flags:

- `--fullscreen` — hand the terminal to mpv instead of the embedded pane
- `--fps N` — playback frame rate (default 24; lower it over slow links)
- `--limit N` — max videos per source (default 30; 0 = no limit)
- `--render auto|halfblock|kitty` — embedded renderer. `auto` uses crisp **kitty
  graphics** pixels on terminals that support them (kitty, Ghostty, WezTerm),
  else universal half-blocks. Force `kitty` if detection misses it over SSH.
- `--vo auto|tct|sixel|kitty` — mpv's video output for `--fullscreen`. `auto`
  picks `kitty`/`sixel`/`tct` from the detected terminal. Use this for crisp
  **sixel** video (e.g. iTerm2, foot): `lazytik --fullscreen --vo sixel @user`.

Run `lazytik doctor` to see which graphics your terminal was detected as.

### Rendering quality

The embedded half-block view's resolution is your terminal's character grid (≈1
pixel wide × 2 tall per cell), so it looks pixelated — bigger window / smaller
font = sharper. For real-pixel video, use a kitty-protocol terminal (`--render`
auto-detects) or `--fullscreen --vo sixel`/`kitty`. Kitty frames are
zlib-compressed and the backend auto-caps at 15fps with a compact decode size so
it sustains real time over a link; if it still slow-motions, lower `--fps`
further (e.g. `--fps 10`) or use `--fullscreen --vo kitty` (mpv is more
bandwidth-efficient).

## Limitations

- Requires `yt-dlp`, `mpv`, and `ffmpeg` on your PATH (auto-detected).
- v1 plays **public** feeds only — no personalized "For You" feed.
- Embedded-pane A/V sync is approximate; use `--fullscreen` for exact sync.
- Subject to TikTok rate-limiting.
