package tui

import (
	"errors"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/player"
	"github.com/naimroslan/lazytik/internal/scraper"
)

// navDebounce delays playback start after a scroll, so flicking through several
// videos only loads the one the user lands on.
const navDebounce = 180 * time.Millisecond

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width == m.width && msg.Height == m.height {
			return m, nil // terminals resend identical sizes
		}
		m.width, m.height = msg.Width, msg.Height
		return m.startCurrent()

	case spinnerTickMsg:
		if !m.loading {
			return m, nil
		}
		m.spinnerFrame++
		return m, spinnerTick()

	case feedLoadedMsg:
		m.feed = msg.videos
		m.loading = false
		if m.index >= len(m.feed) {
			m.index = 0
		}
		if len(m.feed) == 0 {
			m.status = "no videos found"
			return m, nil
		}
		m.status = ""
		return m.startCurrent()

	case scrapeErrMsg:
		m.loading = false
		m.status = "error: " + msg.err.Error()
		return m, nil

	case downloadDoneMsg:
		return m.handleDownloadDone(msg)

	case playbackStartedMsg:
		if msg.gen != m.gen { // scrolled on already
			if msg.dec != nil {
				_ = msg.dec.Close()
			}
			msg.audio.Close()
			return m, nil
		}
		if msg.err != nil {
			m.status = "playback error: " + msg.err.Error()
			return m, nil
		}
		m.dec, m.audio = msg.dec, msg.audio
		m.status = ""
		return m, m.nextFrameCmd(m.gen, m.dec)

	case frameReadyMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.frame = msg.content
		if m.paused {
			return m, nil // freeze on the last frame
		}
		return m, m.nextFrameCmd(m.gen, m.dec)

	case decodeEndedMsg:
		if msg.gen != m.gen {
			return m, nil // stale decoder we already replaced
		}
		// ffmpeg exited after producing frames (its -stream_loop didn't re-engage,
		// e.g. HLS) → loop by restarting the decoder on the same cached file. The
		// frames>0 guard avoids spinning on a broken clip.
		cur, ok := m.current()
		if ok && !m.paused && m.dec != nil && m.dec.Frames() > 0 && m.files[cur.ID] != "" {
			cols, rows := m.paneCells()
			wPx, hPx := m.renderer.CellSize(cols, rows)
			path := m.files[cur.ID]
			_ = m.dec.Close()
			m.dec = nil
			return m, m.restartDecodeCmd(m.gen, path, wPx, hPx)
		}
		if msg.err != nil {
			m.status = "playback stopped: " + msg.err.Error()
		}
		return m, nil

	case decoderReadyMsg:
		if msg.gen != m.gen {
			if msg.dec != nil {
				_ = msg.dec.Close()
			}
			return m, nil
		}
		if msg.err != nil {
			m.status = "loop restart failed: " + msg.err.Error()
			return m, nil
		}
		m.dec = msg.dec
		return m, m.nextFrameCmd(m.gen, m.dec)

	case navSettledMsg:
		if msg.seq != m.navSeq {
			return m, nil // superseded by a later scroll
		}
		return m.startCurrent()

	case playbackEndedMsg:
		if msg.err != nil {
			m.status = "playback error: " + msg.err.Error()
		}
		return m.startCurrent() // resume embedded after fullscreen handoff

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleDownloadDone records a finished download and, if it's the focused video
// that's waiting to play, starts it.
func (m Model) handleDownloadDone(msg downloadDoneMsg) (tea.Model, tea.Cmd) {
	delete(m.downloading, msg.videoID)
	cur, hasCur := m.current()

	if msg.err != nil {
		if errors.Is(msg.err, scraper.ErrNoVideo) {
			m.noVideoIDs[msg.videoID] = true
			if hasCur && cur.ID == msg.videoID {
				return m.skipForward("no video in this post")
			}
		} else if hasCur && cur.ID == msg.videoID {
			m.status = "download failed: " + msg.err.Error()
		}
		return m, nil
	}

	// Cache it only if still in the window; otherwise it's already stale.
	if m.windowIDs()[msg.videoID] {
		m.files[msg.videoID] = msg.path
	} else {
		_ = os.Remove(msg.path)
	}
	// If this is the focused clip and nothing is playing yet, play it now.
	if hasCur && cur.ID == msg.videoID && m.dec == nil {
		return m.startCurrent()
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.cleanupAll()
		return m, tea.Quit

	case "down", "j":
		m = m.move(1)
		m.navSeq++
		return m, navTick(m.navSeq)

	case "up", "k":
		m = m.move(-1)
		m.navSeq++
		return m, navTick(m.navSeq)

	case " ":
		m.paused = !m.paused
		audio, paused := m.audio, m.paused
		setPause := func() tea.Msg { audio.SetPaused(paused); return nil }
		if !m.paused && m.dec != nil {
			return m, tea.Batch(setPause, m.nextFrameCmd(m.gen, m.dec)) // resume
		}
		return m, setPause

	case "enter":
		return m.playFullscreen()
	}
	return m, nil
}

// navTick fires navSettledMsg after the debounce window for the given epoch.
func navTick(seq int) tea.Cmd {
	return tea.Tick(navDebounce, func(time.Time) tea.Msg { return navSettledMsg{seq} })
}

// spinnerTick advances the loading spinner ~8x/second.
func spinnerTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

// move shifts the focused index by delta (clamped, no wrap) and tears down the
// current playback (cached files are kept) so the new video starts cleanly.
func (m Model) move(delta int) Model {
	if len(m.feed) == 0 {
		return m
	}
	n := m.index + delta
	if n < 0 {
		n = 0
	}
	if n >= len(m.feed) {
		n = len(m.feed) - 1
	}
	if n != m.index {
		m.index = n
		m.stopPlayback()
	}
	return m
}

// skipForward advances past an unplayable video (e.g. a photo post) to the next
// one, or stops with a status message if it's the last in the feed.
func (m Model) skipForward(reason string) (Model, tea.Cmd) {
	if m.index >= len(m.feed)-1 {
		m.status = reason + " (end of feed)"
		return m, nil
	}
	m = m.move(1)
	m.status = reason + " — skipping"
	return m.startCurrent()
}

// playFullscreen suspends the TUI and hands the terminal to mpv for the focused
// video, resuming embedded playback when mpv exits.
func (m Model) playFullscreen() (tea.Model, tea.Cmd) {
	cur, ok := m.current()
	if !ok {
		return m, nil
	}
	m.stopPlayback() // free the terminal and audio device for mpv
	cmd := player.FullscreenCmd(m.cfg.Mpv, m.cfg.Vo, cur)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return playbackEndedMsg{err}
	})
}
