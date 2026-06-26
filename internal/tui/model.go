// Package tui implements the lazytik terminal UI: a vertically-scrolled TikTok
// feed navigated with the arrow keys, built on Bubble Tea.
package tui

import (
	"context"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/render"
	"github.com/naimroslan/lazytik/internal/scraper"
)

// Config holds everything the TUI needs to fetch and play a feed.
type Config struct {
	Sources    []string // @user / #hashtag / URLs
	Ytdlp      string   // path to yt-dlp
	Mpv        string   // path to mpv
	FFmpeg     string   // path to ffmpeg
	Limit      int      // max videos to list per source (0 = yt-dlp default)
	FPS        int      // target playback fps for the embedded pane
	Fullscreen bool     // play via mpv handoff instead of the embedded pane
	Render     string   // embedded renderer backend: "kitty" or "halfblock"
	Vo         string   // mpv video output for fullscreen: "kitty"/"sixel"/"tct"
	Shuffle    bool     // randomize the mixed feed order
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg      Config
	renderer render.Renderer

	feed   []scraper.Video
	index  int // currently focused video
	width  int
	height int

	loading bool
	paused  bool
	status  string // transient status / error line

	// Embedded playback state for the focused video.
	gen     int             // playback epoch; bumped whenever playback (re)starts
	dec     *render.Decoder // current ffmpeg decoder, nil when not playing
	audio   *exec.Cmd       // current mpv audio process, nil when not playing
	tmpFile string          // downloaded clip backing dec/audio, removed on stop
	frame   string          // latest rendered frame for the current video

	// Prefetch: the next clip is downloaded in the background while the current
	// one plays, so scrolling down is instant.
	prefetched  map[string]string // videoID -> ready temp file (the next clip)
	prefetching map[string]bool   // videoID -> download in flight
	noVideoIDs  map[string]bool   // videoID -> known to have no video stream
}

// New builds a Model that will load its feed asynchronously on Init.
func New(cfg Config) Model {
	if cfg.FPS <= 0 {
		cfg.FPS = 24
	}
	return Model{
		cfg:         cfg,
		renderer:    render.For(cfg.Render),
		loading:     true,
		status:      "loading feed…",
		prefetched:  map[string]string{},
		prefetching: map[string]bool{},
		noVideoIDs:  map[string]bool{},
	}
}

// Init kicks off the initial feed load.
func (m Model) Init() tea.Cmd {
	return m.loadFeedCmd()
}

// loadFeedCmd fetches every configured source and concatenates the results.
func (m Model) loadFeedCmd() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		var lists [][]scraper.Video
		var lastErr error
		for _, src := range cfg.Sources {
			vids, err := scraper.ListFeed(ctx, cfg.Ytdlp, src, cfg.Limit)
			if err != nil {
				lastErr = err
				continue // a bad source shouldn't sink the whole mix
			}
			lists = append(lists, vids)
		}
		all := mixFeed(lists, cfg.Shuffle)
		if len(all) == 0 && lastErr != nil {
			return scrapeErrMsg{lastErr}
		}
		return feedLoadedMsg{videos: all}
	}
}

// current returns the focused video and whether the feed is non-empty.
func (m Model) current() (scraper.Video, bool) {
	if len(m.feed) == 0 {
		return scraper.Video{}, false
	}
	return m.feed[m.index], true
}

// stopPlayback tears down the current decoder and audio process, if any.
func (m *Model) stopPlayback() {
	if m.dec != nil {
		_ = m.dec.Close()
		m.dec = nil
	}
	if m.audio != nil && m.audio.Process != nil {
		_ = m.audio.Process.Kill()
		_ = m.audio.Wait()
		m.audio = nil
	}
	if m.tmpFile != "" {
		_ = os.Remove(m.tmpFile)
		m.tmpFile = ""
	}
	m.frame = ""
}

// cleanupAll tears down playback and removes every prefetched file. Called on quit.
func (m *Model) cleanupAll() {
	m.stopPlayback()
	for id, p := range m.prefetched {
		_ = os.Remove(p)
		delete(m.prefetched, id)
	}
}
