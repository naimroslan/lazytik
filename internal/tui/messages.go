package tui

import (
	"os/exec"

	"github.com/naimroslan/lazytik/internal/render"
	"github.com/naimroslan/lazytik/internal/scraper"
)

// feedLoadedMsg is emitted when a feed source has been resolved into videos.
type feedLoadedMsg struct {
	videos []scraper.Video
}

// scrapeErrMsg carries a non-fatal error to surface in the status line.
type scrapeErrMsg struct {
	err error
}

// playbackStartedMsg reports the result of starting embedded playback for a
// video. gen identifies the playback epoch so stale starts (after the user has
// scrolled on) can be discarded. On success dec and audio are non-nil and
// tmpPath is the downloaded file backing them.
type playbackStartedMsg struct {
	gen     int
	dec     *render.Decoder
	audio   *exec.Cmd
	tmpPath string
	noVideo bool // post had no video stream; skip to the next one
	err     error
}

// frameReadyMsg delivers one rendered half-block frame for playback epoch gen.
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

// prefetchDoneMsg reports that a background download of an upcoming video
// finished. On success path is the ready file; on error it's empty.
type prefetchDoneMsg struct {
	videoID string
	path    string
	err     error
}

// playbackEndedMsg is emitted after a fullscreen mpv handoff returns control.
type playbackEndedMsg struct {
	err error
}
