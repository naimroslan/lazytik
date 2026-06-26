package render

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

// Decoder reads a video stream through ffmpeg and yields fixed-size RGB24 frames.
// The input is looped forever (-stream_loop -1) so short TikToks repeat like the
// app does; Close stops ffmpeg and releases the pipe.
type Decoder struct {
	cmd  *exec.Cmd
	out  io.ReadCloser
	buf  []byte
	w, h int

	frames int64 // total frames returned, read via Frames()

	mu     sync.Mutex
	stderr bytes.Buffer // ffmpeg diagnostics, surfaced on failure
}

// StartDecode launches ffmpeg to decode streamURL, scaling each frame to fit a
// w×h pixel box (aspect preserved, letterboxed with black) at the target fps.
func StartDecode(ffmpeg, streamURL string, w, h, fps int) (*Decoder, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid frame size %dx%d", w, h)
	}
	vf := fmt.Sprintf(
		"fps=%d,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
		fps, w, h, w, h,
	)
	cmd := exec.Command(ffmpeg,
		"-loglevel", "error",
		"-re",                // emit frames in real time, not as fast as possible
		"-stream_loop", "-1", // loop the input forever
		"-i", streamURL,
		"-an", // no audio (mpv handles audio separately)
		"-vf", vf,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-",
	)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	d := &Decoder{buf: make([]byte, w*h*3), w: w, h: h}
	cmd.Stderr = &lockedWriter{mu: &d.mu, buf: &d.stderr}
	d.cmd, d.out = cmd, out
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return d, nil
}

// lockedWriter lets ffmpeg's stderr goroutine and Err() share the buffer safely.
type lockedWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// Next blocks until the next full frame is available and returns it. The returned
// slice is reused on the following call, so render or copy it before calling Next
// again. On stream end / read failure it returns an error that includes ffmpeg's
// stderr, so a premature exit explains itself instead of being a bare EOF.
func (d *Decoder) Next() ([]byte, error) {
	if _, err := io.ReadFull(d.out, d.buf); err != nil {
		if msg := d.stderrText(); msg != "" {
			return nil, fmt.Errorf("%w: ffmpeg: %s", err, msg)
		}
		return nil, err
	}
	atomic.AddInt64(&d.frames, 1)
	return d.buf, nil
}

// Frames reports how many frames have been returned so far.
func (d *Decoder) Frames() int64 { return atomic.LoadInt64(&d.frames) }

// stderrText returns the last line of ffmpeg's diagnostics, if any.
func (d *Decoder) stderrText() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := strings.TrimSpace(d.stderr.String())
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Size reports the pixel dimensions frames are decoded at.
func (d *Decoder) Size() (w, h int) { return d.w, d.h }

// Close stops ffmpeg and releases its pipe. Safe to call on a nil Decoder.
func (d *Decoder) Close() error {
	if d == nil {
		return nil
	}
	if d.out != nil {
		_ = d.out.Close()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
		_ = d.cmd.Wait()
	}
	return nil
}
