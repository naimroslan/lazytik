package tui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/player"
	"github.com/naimroslan/lazytik/internal/render"
	"github.com/naimroslan/lazytik/internal/scraper"
)

// chromeRows is the number of rows the non-video parts take in the single-column
// layout, leaving the rest for the video pane: 2 pane borders + CAPTION divider +
// info line + caption + COMMENTS divider + placeholder + footer, plus one slack
// row for the optional status line.
const chromeRows = 9

// twoColChromeRows is the vertical chrome in the two-column layout: the status
// line, footer, and the pane's two border rows. The caption/comments live in the
// side column, so they no longer cost vertical rows.
const twoColChromeRows = 4

// twoColMinWidth is the terminal width at or above which the side-by-side desktop
// layout (video | caption/comments) is used; narrower terminals stack vertically.
const twoColMinWidth = 80

// sideMinW is the minimum inner width reserved for the right (caption/comments)
// column in the two-column layout.
const sideMinW = 24

// kittyMaxFPS caps the frame rate for the kitty backend: each frame is a whole
// (compressed) image, so a high rate can't be sustained over a link and the
// video falls into slow motion. Half-blocks (cheap text) keep the full rate.
const kittyMaxFPS = 15

// windowOffsets are the indices, relative to the current one, whose clips are
// kept downloaded: one back and two forward. Prefetch order favours the next.
var windowOffsets = []int{1, -1, 2, 0}

// decodeFPS is the frame rate to decode at for the active renderer.
func (m Model) decodeFPS() int {
	if m.renderer.Name() == "kitty" && m.cfg.FPS > kittyMaxFPS {
		return kittyMaxFPS
	}
	return m.cfg.FPS
}

// useTwoColumn reports whether to render the side-by-side desktop layout (video
// on the left, caption/comments on the right). It is excluded for the kitty
// renderer: a kitty frame is a graphics escape that lipgloss can't measure or
// JoinHorizontal beside a text column (View places it with C=1 and reserves rows
// by hand), so kitty keeps the single-column stack. The owner runs half-blocks
// (Termius/iOS), so this costs nothing in practice.
func (m Model) useTwoColumn() bool {
	return m.width >= twoColMinWidth && m.renderer.Name() != "kitty"
}

// paneCells returns the size, in character cells, of the video pane's content
// area. Shared by the view (to draw) and playback (to size the decoder), so both
// agree on the dimensions the decoder renders frames to.
func (m Model) paneCells() (cols, rows int) {
	if m.useTwoColumn() {
		rows = m.height - twoColChromeRows
		if rows < 3 {
			rows = 3
		}
		// Terminal cells are ~1:2 (w:h), so a visually-square pane needs about
		// twice as many columns as rows. Clamp so the right column keeps at least
		// sideMinW inner columns. The two bordered boxes consume 4 border columns
		// total, so usable content columns = width-4 split between pane and side.
		cols = 2 * rows
		if max := m.width - sideMinW - 4; cols > max {
			cols = max
		}
		if cols < 10 {
			cols = 10
		}
		return cols, rows
	}
	cols = m.width - 2 // pane border columns
	if cols < 10 {
		cols = 10
	}
	rows = m.height - chromeRows
	if rows < 3 {
		rows = 3
	}
	return cols, rows
}

// windowIDs returns the set of video ids within the cache window around index i.
func (m Model) windowIDs() map[string]bool {
	ids := make(map[string]bool, len(windowOffsets))
	for _, off := range windowOffsets {
		j := m.index + off
		if j >= 0 && j < len(m.feed) {
			ids[m.feed[j].ID] = true
		}
	}
	return ids
}

// startCurrent (re)starts embedded playback of the focused video: instantly from
// the download cache when possible, otherwise it kicks off a download (showing
// "buffering…") and plays once it arrives. It also evicts out-of-window clips and
// prefetches neighbours. No-op in fullscreen mode or before size+feed are known.
func (m Model) startCurrent() (Model, tea.Cmd) {
	if m.cfg.Fullscreen {
		return m, nil
	}
	cur, ok := m.current()
	if !ok || m.width == 0 {
		return m, nil
	}
	if m.noVideoIDs[cur.ID] {
		return m.skipForward("no video in this post")
	}

	m.stopPlayback() // bumps gen, invalidating stale frames
	m.paused = false
	m.evictOutsideWindow()

	var cmds []tea.Cmd
	if path, ok := m.files[cur.ID]; ok {
		cols, rows := m.paneCells()
		wPx, hPx := m.renderer.CellSize(cols, rows)
		cmds = append(cmds, m.startFromFileCmd(m.gen, path, wPx, hPx))
	} else if !m.downloading[cur.ID] {
		m.downloading[cur.ID] = true
		cmds = append(cmds, downloadCmd(m.cfg, cur.ID, cur.PageURL))
	}
	m, pf := m.prefetchWindow()
	return m, tea.Batch(append(cmds, pf...)...)
}

// prefetchWindow starts background downloads for window neighbours not already
// cached, in flight, or known video-less. Returns the model with those marked.
func (m Model) prefetchWindow() (Model, []tea.Cmd) {
	var cmds []tea.Cmd
	for _, off := range windowOffsets {
		if off == 0 {
			continue
		}
		j := m.index + off
		if j < 0 || j >= len(m.feed) {
			continue
		}
		v := m.feed[j]
		if v.ID == "" || m.files[v.ID] != "" || m.downloading[v.ID] || m.noVideoIDs[v.ID] {
			continue
		}
		m.downloading[v.ID] = true
		cmds = append(cmds, downloadCmd(m.cfg, v.ID, v.PageURL))
	}
	return m, cmds
}

// evictOutsideWindow deletes cached files for videos outside the current window.
func (m *Model) evictOutsideWindow() {
	keep := m.windowIDs()
	for id, p := range m.files {
		if !keep[id] {
			_ = os.Remove(p)
			delete(m.files, id)
		}
	}
}

// downloadTemp fetches a video to a fresh temp file, returning its path. The
// placeholder file from CreateTemp is removed first: yt-dlp treats an existing
// destination as "already downloaded" and would skip, leaving it empty.
func downloadTemp(ctx context.Context, ytdlp, pageURL string) (string, error) {
	f, err := os.CreateTemp("", "lazytik-*.mp4")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	_ = f.Close()
	_ = os.Remove(tmp)
	if err := scraper.Download(ctx, ytdlp, pageURL, tmp); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return tmp, nil
}

// downloadCmd downloads a clip in the background, reporting a downloadDoneMsg.
// Used for both the current video and prefetched neighbours; the cache dedupes.
func downloadCmd(cfg Config, id, pageURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		path, err := downloadTemp(ctx, cfg.Ytdlp, pageURL)
		return downloadDoneMsg{videoID: id, path: path, err: err}
	}
}

// beginPlayback starts the ffmpeg decoder (at fps) and audio for a local file.
func beginPlayback(cfg Config, path string, wPx, hPx, fps int) (*render.Decoder, *player.Audio, error) {
	dec, err := render.StartDecode(cfg.FFmpeg, path, wPx, hPx, fps)
	if err != nil {
		return nil, nil, err
	}
	audio, _ := player.StartAudio(cfg.Mpv, path) // best-effort; nil-safe downstream
	return dec, audio, nil
}

// startFromFileCmd begins playback from an already-downloaded (cached) file.
func (m Model) startFromFileCmd(gen int, path string, wPx, hPx int) tea.Cmd {
	cfg := m.cfg
	fps := m.decodeFPS()
	return func() tea.Msg {
		dec, audio, err := beginPlayback(cfg, path, wPx, hPx, fps)
		if err != nil {
			return playbackStartedMsg{gen: gen, err: err}
		}
		return playbackStartedMsg{gen: gen, dec: dec, audio: audio}
	}
}

// restartDecodeCmd re-opens the decoder on a cached file to loop playback (audio
// keeps looping independently in mpv). Reports a decoderReadyMsg.
func (m Model) restartDecodeCmd(gen int, path string, wPx, hPx int) tea.Cmd {
	cfg := m.cfg
	fps := m.decodeFPS()
	return func() tea.Msg {
		dec, err := render.StartDecode(cfg.FFmpeg, path, wPx, hPx, fps)
		return decoderReadyMsg{gen: gen, dec: dec, err: err}
	}
}

// nextFrameCmd reads and renders one frame for the given playback epoch. Returns
// nil if there is no decoder, so no caller can schedule a read on a nil decoder.
func (m Model) nextFrameCmd(gen int, dec *render.Decoder) tea.Cmd {
	if dec == nil {
		return nil
	}
	renderer := m.renderer
	return func() tea.Msg {
		buf, err := dec.Next()
		if err != nil {
			return decodeEndedMsg{gen: gen, err: err}
		}
		w, h := dec.Size()
		return frameReadyMsg{gen: gen, content: renderer.Render(buf, w, h)}
	}
}
