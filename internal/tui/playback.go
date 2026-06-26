package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/player"
	"github.com/naimroslan/lazytik/internal/render"
	"github.com/naimroslan/lazytik/internal/scraper"
)

// chromeRows is the number of rows the header, caption, status and footer take,
// leaving the rest for the video pane.
const chromeRows = 6

// kittyMaxFPS caps the frame rate for the kitty backend: each frame is a whole
// (compressed) image, so a high rate can't be sustained over a link and the
// video falls into slow motion. Half-blocks (cheap text) keep the full rate.
// Pair this with the small kittyCell* decode size to bound the per-frame bytes.
const kittyMaxFPS = 15

// decodeFPS is the frame rate to decode at for the active renderer.
func (m Model) decodeFPS() int {
	if m.renderer.Name() == "kitty" && m.cfg.FPS > kittyMaxFPS {
		return kittyMaxFPS
	}
	return m.cfg.FPS
}

// paneCells returns the size, in character cells, of the video pane's content
// area. Shared by the view (to draw) and playback (to size the decoder) so the
// rendered frame always fits.
func (m Model) paneCells() (cols, rows int) {
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

// startCurrent (re)starts embedded playback of the focused video at the current
// pane size. It is a no-op in fullscreen mode or before the terminal size and
// feed are known. It bumps the playback epoch so older frames are discarded, and
// uses a prefetched file when one is ready (instant) instead of downloading.
func (m Model) startCurrent() (Model, tea.Cmd) {
	if m.cfg.Fullscreen {
		return m, nil
	}
	cur, ok := m.current()
	if !ok || m.width == 0 {
		return m, nil
	}
	// Prefetch already discovered this post has no video — skip it instantly.
	if m.noVideoIDs[cur.ID] {
		return m.skipForward("no video in this post")
	}

	m.stopPlayback()
	m.gen++
	m.paused = false

	cols, rows := m.paneCells()
	wPx, hPx := m.renderer.CellSize(cols, rows)

	if path, ok := m.prefetched[cur.ID]; ok {
		delete(m.prefetched, cur.ID)
		return m, m.startFromFileCmd(m.gen, path, wPx, hPx)
	}
	return m, m.startPlaybackCmd(m.gen, cur, wPx, hPx)
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

// beginPlayback starts the ffmpeg decoder (at the given fps) and (best-effort)
// audio for a local file. Audio is best-effort: a headless box has no sound
// device, but video still renders. The decoder and audio share the same file.
func beginPlayback(cfg Config, path string, wPx, hPx, fps int) (*render.Decoder, *exec.Cmd, error) {
	dec, err := render.StartDecode(cfg.FFmpeg, path, wPx, hPx, fps)
	if err != nil {
		return nil, nil, err
	}
	audio := player.AudioFileCmd(cfg.Mpv, path)
	_ = audio.Start()
	return dec, audio, nil
}

// startPlaybackCmd downloads the video to a temp file (yt-dlp supplies the
// headers the CDN demands), then begins playback, reporting a playbackStartedMsg.
func (m Model) startPlaybackCmd(gen int, v scraper.Video, wPx, hPx int) tea.Cmd {
	cfg := m.cfg
	fps := m.decodeFPS()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		tmp, err := downloadTemp(ctx, cfg.Ytdlp, v.PageURL)
		if err != nil {
			if errors.Is(err, scraper.ErrNoVideo) {
				return playbackStartedMsg{gen: gen, noVideo: true}
			}
			return playbackStartedMsg{gen: gen, err: err}
		}
		dec, audio, err := beginPlayback(cfg, tmp, wPx, hPx, fps)
		if err != nil {
			os.Remove(tmp)
			return playbackStartedMsg{gen: gen, err: err}
		}
		return playbackStartedMsg{gen: gen, dec: dec, audio: audio, tmpPath: tmp}
	}
}

// startFromFileCmd begins playback from an already-downloaded (prefetched) file.
func (m Model) startFromFileCmd(gen int, path string, wPx, hPx int) tea.Cmd {
	cfg := m.cfg
	fps := m.decodeFPS()
	return func() tea.Msg {
		dec, audio, err := beginPlayback(cfg, path, wPx, hPx, fps)
		if err != nil {
			os.Remove(path)
			return playbackStartedMsg{gen: gen, err: err}
		}
		return playbackStartedMsg{gen: gen, dec: dec, audio: audio, tmpPath: path}
	}
}

// restartDecodeCmd re-opens the decoder on an already-downloaded file to loop
// playback (audio keeps looping independently in mpv). Reports a decoderReadyMsg.
func (m Model) restartDecodeCmd(gen int, path string, wPx, hPx int) tea.Cmd {
	cfg := m.cfg
	fps := m.decodeFPS()
	return func() tea.Msg {
		dec, err := render.StartDecode(cfg.FFmpeg, path, wPx, hPx, fps)
		return decoderReadyMsg{gen: gen, dec: dec, err: err}
	}
}

// maybePrefetchNext kicks off a background download of the next clip if it isn't
// already cached, in flight, or known to lack video. It evicts any stale
// prefetched file so at most one upcoming clip is held on disk.
func (m Model) maybePrefetchNext() (Model, tea.Cmd) {
	ni := m.index + 1
	if ni >= len(m.feed) {
		return m, nil
	}
	next := m.feed[ni]
	m.evictPrefetchExcept(next.ID)
	if next.ID == "" || m.noVideoIDs[next.ID] || m.prefetching[next.ID] {
		return m, nil
	}
	if _, ok := m.prefetched[next.ID]; ok {
		return m, nil
	}
	m.prefetching[next.ID] = true
	return m, m.prefetchCmd(next)
}

// prefetchCmd downloads v in the background, reporting a prefetchDoneMsg.
func (m Model) prefetchCmd(v scraper.Video) tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		tmp, err := downloadTemp(ctx, cfg.Ytdlp, v.PageURL)
		if err != nil {
			return prefetchDoneMsg{videoID: v.ID, err: err}
		}
		return prefetchDoneMsg{videoID: v.ID, path: tmp}
	}
}

// evictPrefetchExcept deletes every prefetched file except the one for keepID.
func (m Model) evictPrefetchExcept(keepID string) {
	for id, p := range m.prefetched {
		if id != keepID {
			_ = os.Remove(p)
			delete(m.prefetched, id)
		}
	}
}

// nextFrameCmd reads and renders one frame for the given playback epoch.
func (m Model) nextFrameCmd(gen int, dec *render.Decoder) tea.Cmd {
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
