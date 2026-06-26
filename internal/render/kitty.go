package render

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"strconv"
	"strings"
)

// Kitty renders frames with the kitty graphics protocol: real pixels, far sharper
// than half-blocks, on terminals that support it (kitty, Ghostty, WezTerm). Each
// frame is zlib-compressed (kitty's o=z) to keep bandwidth manageable, especially
// over SSH. Terminals without the protocol must use HalfBlock instead.
//
// One image id is reused and its previous placement deleted each frame, so frames
// replace rather than stack.
type Kitty struct{}

// kittyCell is the assumed pixel size of one terminal cell. The decode box
// (cols*w × rows*h) then matches the cols×rows display box's aspect, so kitty's
// scaling doesn't distort. Cells are ~1:2, so is this. Kept small to bound the
// per-frame payload (kitty streams whole frames) — still ~16× the detail of
// half-blocks while staying light enough to sustain real-time over a link.
const (
	kittyCellW = 4
	kittyCellH = 8
	kittyImgID = 1
	kittyChunk = 4096 // max base64 bytes per escape, per the protocol
)

func (Kitty) Name() string { return "kitty" }

// CellSize maps the pane cell box to a decode resolution with real detail.
func (Kitty) CellSize(cols, rows int) (wPx, hPx int) {
	return cols * kittyCellW, rows * kittyCellH
}

// Render encodes an RGB24 frame as a kitty graphics escape sequence that displays
// it in the cols×rows cell box the frame was sized for.
func (Kitty) Render(rgb []byte, w, h int) string {
	if w <= 0 || h <= 0 || len(rgb) < w*h*3 {
		return ""
	}
	cols, rows := w/kittyCellW, h/kittyCellH
	if cols < 1 || rows < 1 {
		return ""
	}

	payload := base64.StdEncoding.EncodeToString(zlibCompress(rgb[:w*h*3]))

	var b strings.Builder
	// Synchronized update: the terminal buffers everything between these markers
	// and paints it in one shot, so the frame swaps without tearing/flicker.
	// Terminals that don't support it ignore the private mode.
	b.WriteString("\x1b[?2026h")

	// Transmit + display, chunked. Control keys go on the first chunk only.
	// Reusing a stable image id (i) and placement id (p) makes each frame REPLACE
	// the previous one in place — no delete-then-redraw flash.
	first := true
	for len(payload) > 0 {
		n := min(kittyChunk, len(payload))
		chunk := payload[:n]
		payload = payload[n:]
		more := 0
		if len(payload) > 0 {
			more = 1
		}

		b.WriteString("\x1b_G")
		if first {
			// a=T transmit+display, f=24 RGB, o=z zlib, q=2 quiet (no replies that
			// would corrupt TUI input), C=1 don't move the cursor (the caller
			// reserves the rows), s/v image px, c/r display cell box, i/p stable
			// ids so the placement is replaced rather than stacked.
			b.WriteString("a=T,f=24,o=z,q=2,C=1,i=")
			b.WriteString(strconv.Itoa(kittyImgID))
			b.WriteString(",p=1,s=")
			b.WriteString(strconv.Itoa(w))
			b.WriteString(",v=")
			b.WriteString(strconv.Itoa(h))
			b.WriteString(",c=")
			b.WriteString(strconv.Itoa(cols))
			b.WriteString(",r=")
			b.WriteString(strconv.Itoa(rows))
			b.WriteString(",m=")
			b.WriteString(strconv.Itoa(more))
			first = false
		} else {
			b.WriteString("m=")
			b.WriteString(strconv.Itoa(more))
		}
		b.WriteByte(';')
		b.WriteString(chunk)
		b.WriteString("\x1b\\")
	}
	b.WriteString("\x1b[?2026l") // end synchronized update
	return b.String()
}

func zlibCompress(p []byte) []byte {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	_, _ = zw.Write(p)
	_ = zw.Close()
	return buf.Bytes()
}
