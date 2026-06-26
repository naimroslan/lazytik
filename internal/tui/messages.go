package tui

import (
	"github.com/naimroslan/lazytik/internal/player"
	"github.com/naimroslan/lazytik/internal/render"
	"github.com/naimroslan/lazytik/internal/scraper"
)

// feedLoadedMsg is emitted when the feed sources have been resolved into videos.
type feedLoadedMsg struct {
	videos []scraper.Video
}

// scrapeErrMsg carries a non-fatal error to surface in the status line.
type scrapeErrMsg struct {
	err error
}

// spinnerTickMsg advances the loading spinner.
type spinnerTickMsg struct{}

// navSettledMsg fires after navigation pauses; seq guards against stale ticks so
// only the index the user landed on starts playing (debounces fast scrolling).
type navSettledMsg struct {
	seq int
}

// downloadDoneMsg reports a finished background download (current clip or a
// prefetched neighbour). On success path is the file; on error it's empty.
type downloadDoneMsg struct {
	videoID string
	path    string
	err     error
}

// playbackStartedMsg reports the result of starting embedded playback for epoch
// gen so stale starts (after scrolling on) can be discarded. On success dec and
// audio are set.
type playbackStartedMsg struct {
	gen   int
	dec   *render.Decoder
	audio *player.Audio
	err   error
}

// frameReadyMsg delivers one rendered frame for playback epoch gen.
type frameReadyMsg struct {
	gen     int
	content string
}

// decodeEndedMsg signals the decoder stopped (error or stream end) for epoch gen.
type decodeEndedMsg struct {
	gen int
	err error
}

// decoderReadyMsg delivers a freshly (re)started decoder for epoch gen, used to
// loop playback when ffmpeg exits at end-of-stream (e.g. for HLS sources).
type decoderReadyMsg struct {
	gen int
	dec *render.Decoder
	err error
}

// playbackEndedMsg is emitted after a fullscreen mpv handoff returns control.
type playbackEndedMsg struct {
	err error
}
