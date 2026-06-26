// Package player drives external mpv for playback. The fullscreen path hands the
// whole terminal to mpv (via Bubble Tea's ExecProcess); the embedded path uses mpv
// for audio only (controllable over an IPC socket) while lazytik renders frames.
package player

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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

// Audio is a looping, audio-only mpv playing the already-downloaded clip,
// controllable over an IPC socket so it can pause in lock-step with the video.
type Audio struct {
	cmd  *exec.Cmd
	sock string
}

var audioSeq int // disambiguates socket names within one process

// StartAudio launches audio-only mpv for a local file with an IPC server, so the
// embedded video pane can pause/resume it. Best-effort: on a headless box with no
// sound device mpv simply produces nothing; video still renders.
func StartAudio(mpv, path string) (*Audio, error) {
	audioSeq++
	sock := filepath.Join(os.TempDir(), fmt.Sprintf("lazytik-audio-%d-%d.sock", os.Getpid(), audioSeq))
	cmd := exec.Command(mpv,
		"--no-video",
		"--loop-file=inf",
		"--really-quiet",
		"--no-terminal", // detached: lazytik's TUI owns stdin/stdout
		"--input-ipc-server="+sock,
		path,
	)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Audio{cmd: cmd, sock: sock}, nil
}

// pauseCommand is the mpv JSON IPC line to set the pause property.
func pauseCommand(paused bool) []byte {
	return []byte(fmt.Sprintf(`{"command":["set_property","pause",%t]}`+"\n", paused))
}

// SetPaused pauses or resumes audio over the IPC socket. Best-effort: errors
// (socket not yet ready, already gone) are ignored. Safe on a nil receiver.
func (a *Audio) SetPaused(paused bool) {
	if a == nil || a.sock == "" {
		return
	}
	conn, err := net.DialTimeout("unix", a.sock, 300*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(300 * time.Millisecond))
	_, _ = conn.Write(pauseCommand(paused))
}

// Close stops mpv and removes its socket. Safe on a nil receiver.
func (a *Audio) Close() {
	if a == nil {
		return
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
	}
	if a.sock != "" {
		_ = os.Remove(a.sock)
	}
}
