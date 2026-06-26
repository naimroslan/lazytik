// Package render turns decoded RGB video frames into terminal-drawable strings.
//
// The default backend (halfblock) encodes two vertical pixels per character cell
// using the upper-half-block rune ▀ with a 24-bit foreground (top pixel) and
// background (bottom pixel). The result is ordinary styled text, so it composites
// natively inside a Bubble Tea view and works on any truecolor terminal — even
// over SSH. Pixel-perfect backends (kitty/sixel) can be added behind this
// interface later.
package render

// Renderer converts a raw RGB24 frame (row-major, top-to-bottom, len == w*h*3)
// into a string ready to drop into a TUI pane.
type Renderer interface {
	// Name identifies the backend (e.g. "halfblock").
	Name() string
	// CellSize maps a pane size in character cells to the pixel dimensions a
	// frame should be decoded at for this backend.
	CellSize(cols, rows int) (wPx, hPx int)
	// Render encodes one RGB24 frame of the given pixel size.
	Render(rgb []byte, wPx, hPx int) string
}

// Default returns the renderer lazytik uses unless overridden.
func Default() Renderer { return HalfBlock{} }

// For returns the renderer for a backend name ("kitty" → pixels via the kitty
// graphics protocol; anything else → universal half-blocks).
func For(name string) Renderer {
	if name == "kitty" {
		return Kitty{}
	}
	return HalfBlock{}
}
