package tui

import (
	"errors"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/player"
	"github.com/naimroslan/lazytik/internal/scraper"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Ignore duplicate size events (terminals resend them); only a real
		// change should restart playback to refit the pane.
		if msg.Width == m.width && msg.Height == m.height {
			return m, nil
		}
		m.width, m.height = msg.Width, msg.Height
		return m.startCurrent()

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

	case playbackStartedMsg:
		// Discard a start that belongs to a video we've already scrolled past.
		if msg.gen != m.gen {
			_ = msg.dec.Close()
			killAudio(msg.audio)
			if msg.tmpPath != "" {
				_ = os.Remove(msg.tmpPath)
			}
			return m, nil
		}
		if msg.noVideo {
			// Photo/slideshow post with no video — skip to the next one.
			return m.skipForward("no video in this post")
		}
		if msg.err != nil {
			m.status = "playback error: " + msg.err.Error()
			return m, nil
		}
		m.dec, m.audio, m.tmpFile = msg.dec, msg.audio, msg.tmpPath
		m.status = ""
		m, pf := m.maybePrefetchNext()
		return m, tea.Batch(m.nextFrameCmd(m.gen, m.dec), pf)

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
		// If ffmpeg exited after producing frames (its own -stream_loop didn't
		// re-engage, e.g. for HLS), loop by restarting the decoder on the same
		// downloaded file. The frames>0 guard avoids spinning on a broken clip.
		if !m.paused && m.tmpFile != "" && m.dec != nil && m.dec.Frames() > 0 {
			cols, rows := m.paneCells()
			wPx, hPx := m.renderer.CellSize(cols, rows)
			path := m.tmpFile
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

	case prefetchDoneMsg:
		delete(m.prefetching, msg.videoID)
		if msg.err != nil {
			if errors.Is(msg.err, scraper.ErrNoVideo) {
				m.noVideoIDs[msg.videoID] = true
			}
			return m, nil
		}
		// Keep it only if it's still the upcoming video; otherwise discard.
		if ni := m.index + 1; ni < len(m.feed) && m.feed[ni].ID == msg.videoID {
			m.prefetched[msg.videoID] = msg.path
		} else {
			os.Remove(msg.path)
		}
		return m, nil

	case playbackEndedMsg:
		if msg.err != nil {
			m.status = "playback error: " + msg.err.Error()
		}
		// Resume embedded playback after returning from a fullscreen handoff.
		return m.startCurrent()

	case tea.KeyMsg:
		return m.handleKey(msg)
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
		return m.startCurrent()

	case "up", "k":
		m = m.move(-1)
		return m.startCurrent()

	case " ":
		m.paused = !m.paused
		if !m.paused && m.dec != nil {
			return m, m.nextFrameCmd(m.gen, m.dec) // resume
		}
		return m, nil

	case "enter":
		return m.playFullscreen()
	}
	return m, nil
}

// move shifts the focused index by delta (clamped, no wrap) and tears down the
// current playback so the new video starts cleanly.
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

// killAudio stops a detached audio process, tolerating nil.
func killAudio(c *exec.Cmd) {
	if c != nil && c.Process != nil {
		_ = c.Process.Kill()
		_ = c.Wait()
	}
}
