// Package player drives external mpv for playback. The fullscreen path hands the
// whole terminal to mpv (via Bubble Tea's ExecProcess); the embedded path (M2)
// uses mpv for audio only while lazytik renders frames itself.
package player

import (
	"os/exec"

	"github.com/naimroslan/lazytik/internal/scraper"
)

// target returns the best URL to feed mpv: the resolved stream URL if we have
// one, otherwise the page URL (mpv's built-in yt-dlp hook will resolve it).
func target(v scraper.Video) string {
	if v.StreamURL != "" {
		return v.StreamURL
	}
	return v.PageURL
}

// FullscreenCmd builds an mpv invocation that plays v looping, drawing video into
// the current terminal with the given video output (vo): "kitty"/"sixel" for crisp
// pixels on capable terminals, or "tct" (Unicode half-blocks) anywhere, incl. SSH.
// Run it with tea.ExecProcess so the TUI suspends while mpv owns the screen.
func FullscreenCmd(mpv, vo string, v scraper.Video) *exec.Cmd {
	if vo == "" {
		vo = "tct"
	}
	return exec.Command(mpv,
		"--loop-file=inf",                // loop like TikTok until the user quits
		"--vo="+vo,                       // terminal video output
		"--really-quiet",                 // terminal vo is corrupted by mpv's stderr chatter
		"--no-input-default-bindings=no", // keep mpv's keys (q quits back to lazytik)
		target(v),
	)
}

// AudioFileCmd builds an mpv invocation that plays only the audio of a local
// file on a loop, alongside lazytik's own frame rendering in the embedded pane.
// The file is the one lazytik already downloaded, so audio and video share a
// source and need no second network fetch.
func AudioFileCmd(mpv, path string) *exec.Cmd {
	return exec.Command(mpv,
		"--no-video",
		"--loop-file=inf",
		"--really-quiet",
		"--no-terminal", // detached: lazytik's TUI owns stdin/stdout
		path,
	)
}
