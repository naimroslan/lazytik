package render

import (
	"strings"
	"testing"
)

func TestCellSize(t *testing.T) {
	w, h := HalfBlock{}.CellSize(40, 30)
	if w != 40 || h != 60 {
		t.Fatalf("CellSize(40,30)=(%d,%d) want (40,60)", w, h)
	}
}

// A single cell from a 1x2 image: top pixel red, bottom pixel blue.
func TestRenderSingleCell(t *testing.T) {
	rgb := []byte{
		255, 0, 0, // top: red
		0, 0, 255, // bottom: blue
	}
	got := HalfBlock{}.Render(rgb, 1, 2)
	want := "\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m" + upperHalfBlock + "\x1b[0m"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// Adjacent identical cells must not repeat the color escape.
func TestRenderColorDeduplication(t *testing.T) {
	// 2x2 all white: two cells in one row, same color.
	rgb := make([]byte, 2*2*3)
	for i := range rgb {
		rgb[i] = 255
	}
	got := HalfBlock{}.Render(rgb, 2, 2)
	// Exactly one fg + one bg escape for the whole row, then two blocks.
	if n := strings.Count(got, "38;2;255;255;255"); n != 1 {
		t.Errorf("fg escape emitted %d times, want 1 (dedup): %q", n, got)
	}
	if n := strings.Count(got, upperHalfBlock); n != 2 {
		t.Errorf("expected 2 block runes, got %d", n)
	}
}

func TestRenderMultipleRows(t *testing.T) {
	// 1x4 image → 2 rows of output, separated by one newline.
	rgb := make([]byte, 1*4*3)
	got := HalfBlock{}.Render(rgb, 1, 4)
	if n := strings.Count(got, "\n"); n != 1 {
		t.Errorf("1x4 frame should have 1 newline, got %d", n)
	}
	if n := strings.Count(got, "\x1b[0m"); n != 2 {
		t.Errorf("expected a reset per row (2), got %d", n)
	}
}

func TestRenderOddHeightRoundsDown(t *testing.T) {
	// h=3 should be treated as h=2 (one output row); needs 1*3*3 bytes minimum.
	rgb := make([]byte, 1*3*3)
	got := HalfBlock{}.Render(rgb, 1, 3)
	if strings.Contains(got, "\n") {
		t.Errorf("odd height should round to a single row (no newline): %q", got)
	}
}

func TestRenderRejectsBadInput(t *testing.T) {
	r := HalfBlock{}
	if got := r.Render([]byte{1, 2, 3}, 4, 4); got != "" {
		t.Errorf("undersized buffer should render empty, got %q", got)
	}
	if got := r.Render(nil, 0, 0); got != "" {
		t.Errorf("zero size should render empty, got %q", got)
	}
}

func TestItoa(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{{0, "0"}, {7, "7"}, {42, "42"}, {255, "255"}}
	for _, c := range cases {
		if got := itoa(c.in); got != c.want {
			t.Errorf("itoa(%d)=%q want %q", c.in, got, c.want)
		}
	}
}
