// Package tui implements the lazytik terminal UI: a vertically-scrolled TikTok
// feed navigated with the arrow keys, built on Bubble Tea.
package tui

import (
	"context"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/player"
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

	loading      bool
	spinnerFrame int
	paused       bool
	status       string // transient status / error line

	// Embedded playback state for the focused video.
	gen   int             // playback epoch; bumped whenever playback (re)starts
	dec   *render.Decoder // current ffmpeg decoder, nil when not playing
	audio *player.Audio   // current mpv audio, nil when not playing
	frame string          // latest rendered frame for the current video

	// Download cache + prefetch: keep clips for a window of indices around the
	// current one so scrolling to neighbours (and back) is instant.
	files       map[string]string // videoID -> downloaded local file
	downloading map[string]bool   // videoID -> download in flight
	noVideoIDs  map[string]bool   // videoID -> known to have no video stream
	navSeq      int               // navigation epoch, for debouncing fast scroll
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
		files:       map[string]string{},
		downloading: map[string]bool{},
		noVideoIDs:  map[string]bool{},
	}
}

// Init kicks off the initial feed load and the loading spinner.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadFeedCmd(), spinnerTick())
}

// loadFeedCmd fetches every configured source concurrently and mixes the results.
func (m Model) loadFeedCmd() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		lists := make([][]scraper.Video, len(cfg.Sources))
		errs := make([]error, len(cfg.Sources))
		var wg sync.WaitGroup
		for i, src := range cfg.Sources {
			wg.Add(1)
			go func(i int, src string) {
				defer wg.Done()
				lists[i], errs[i] = scraper.ListFeed(ctx, cfg.Ytdlp, src, cfg.Limit)
			}(i, src)
		}
		wg.Wait()

		var ok [][]scraper.Video
		var lastErr error
		for i, l := range lists {
			if errs[i] != nil {
				lastErr = errs[i] // a bad source shouldn't sink the whole mix
				continue
			}
			ok = append(ok, l)
		}
		all := mixFeed(ok, cfg.Shuffle)
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

// stopPlayback tears down the current decoder and audio, leaving the cached files
// in place so navigating back is instant. It bumps the playback epoch so any
// in-flight frame/decoder messages for the old clip are discarded (without this,
// a stale frameReadyMsg arriving after dec is nil would nil-deref).
func (m *Model) stopPlayback() {
	if m.dec != nil {
		_ = m.dec.Close()
		m.dec = nil
	}
	m.audio.Close()
	m.audio = nil
	m.frame = ""
	m.gen++
}

// cleanupAll tears down playback and removes every cached file. Called on quit.
func (m *Model) cleanupAll() {
	m.stopPlayback()
	for id, p := range m.files {
		_ = os.Remove(p)
		delete(m.files, id)
	}
}
