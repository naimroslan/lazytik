package render

import "strings"

const upperHalfBlock = "▀" // U+2580: fg fills the top half, bg the bottom half

// HalfBlock renders frames as colored half-block characters: each cell stacks
// two vertical pixels (top → foreground, bottom → background).
type HalfBlock struct{}

func (HalfBlock) Name() string { return "halfblock" }

// CellSize maps cells to pixels: one cell is 1 pixel wide and 2 pixels tall, so a
// pane of cols×rows displays a cols×(2·rows) image.
func (HalfBlock) CellSize(cols, rows int) (wPx, hPx int) {
	return cols, rows * 2
}

// Render encodes an RGB24 frame (len == w*h*3) into half-block text. h is rounded
// down to an even number of rows. To keep the output compact and flicker-free, a
// new color escape is emitted only when the color actually changes from the
// previous cell.
func (HalfBlock) Render(rgb []byte, w, h int) string {
	if w <= 0 || h <= 0 || len(rgb) < w*h*3 {
		return ""
	}
	h -= h % 2 // need pixel rows in pairs

	var b strings.Builder
	b.Grow(w * h / 2 * 8) // rough estimate to avoid reallocs

	// -1 sentinel forces an escape on the first cell of each line.
	lastFg, lastBg := -1, -1
	for y := 0; y < h; y += 2 {
		topRow := y * w * 3
		botRow := (y + 1) * w * 3
		for x := 0; x < w; x++ {
			ti := topRow + x*3
			bi := botRow + x*3
			tr, tg, tb := rgb[ti], rgb[ti+1], rgb[ti+2]
			br, bg2, bb := rgb[bi], rgb[bi+1], rgb[bi+2]

			fg := rgbKey(tr, tg, tb)
			bg := rgbKey(br, bg2, bb)
			if fg != lastFg {
				writeColor(&b, 38, tr, tg, tb)
				lastFg = fg
			}
			if bg != lastBg {
				writeColor(&b, 48, br, bg2, bb)
				lastBg = bg
			}
			b.WriteString(upperHalfBlock)
		}
		b.WriteString("\x1b[0m") // reset at line end
		lastFg, lastBg = -1, -1
		if y+2 < h {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func rgbKey(r, g, b byte) int {
	return int(r)<<16 | int(g)<<8 | int(b)
}

// writeColor emits a truecolor SGR escape: layer 38 = foreground, 48 = background.
func writeColor(b *strings.Builder, layer int, r, g, bl byte) {
	b.WriteString("\x1b[")
	b.WriteString(itoa(layer))
	b.WriteString(";2;")
	b.WriteString(itoa(int(r)))
	b.WriteByte(';')
	b.WriteString(itoa(int(g)))
	b.WriteByte(';')
	b.WriteString(itoa(int(bl)))
	b.WriteByte('m')
}

// itoa avoids fmt in the per-pixel hot path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [3]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
