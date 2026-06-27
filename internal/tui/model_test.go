package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/scraper"
)

func testFeed() []scraper.Video {
	return []scraper.Video{
		{ID: "a", Author: "alice", Caption: "first", Likes: 1500000},
		{ID: "b", Author: "bob", Caption: "second", Likes: 42},
	}
}

// send applies a message to the model and returns the concrete Model back.
func send(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

// loaded returns a sized model populated with the test feed.
func loaded() Model {
	m := send(New(Config{}), feedLoadedMsg{videos: testFeed()})
	return send(m, tea.WindowSizeMsg{Width: 60, Height: 20})
}

func TestViewRendersHeaderAndControls(t *testing.T) {
	m := loaded()
	view := m.View()
	for _, want := range []string{"@alice", "1.5M", "1/2", "next", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n---\n%s", want, view)
		}
	}
}

func TestArrowNavigationMovesAndClamps(t *testing.T) {
	m := loaded()

	// down moves to second video
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.index != 1 {
		t.Fatalf("after down: index=%d want 1", m.index)
	}
	if !strings.Contains(m.View(), "@bob") {
		t.Errorf("expected @bob in view after scrolling down")
	}

	// down again clamps at the end (no wrap)
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.index != 1 {
		t.Fatalf("down past end: index=%d want 1 (clamped)", m.index)
	}

	// up returns to first; up again clamps at 0
	m = send(m, tea.KeyMsg{Type: tea.KeyUp})
	m = send(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.index != 0 {
		t.Fatalf("up past start: index=%d want 0 (clamped)", m.index)
	}
}

func TestNavigationClearsStaleFrame(t *testing.T) {
	m := loaded()
	m.frame = "OLD FRAME"
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.frame != "" {
		t.Errorf("frame should clear on navigation, got %q", m.frame)
	}
}

func TestSpaceTogglesPause(t *testing.T) {
	m := loaded()
	if m.paused {
		t.Fatal("should start unpaused")
	}
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	if !m.paused {
		t.Error("space should pause")
	}
}

// wide returns a model sized for the two-column desktop layout.
func wide() Model {
	m := send(New(Config{}), feedLoadedMsg{videos: testFeed()})
	return send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
}

func TestWideLayoutHasSections(t *testing.T) {
	m := wide()
	if !m.useTwoColumn() {
		t.Fatal("width 120 should use the two-column layout")
	}
	view := m.View()
	for _, want := range []string{"CAPTION", "COMMENTS", "@alice", "1.5M", "first"} {
		if !strings.Contains(view, want) {
			t.Errorf("wide view missing %q\n---\n%s", want, view)
		}
	}
}

func TestSingleColumnHasSections(t *testing.T) {
	// The narrow (Width 60) layout stacks vertically but still labels sections.
	view := loaded().View()
	for _, want := range []string{"CAPTION", "COMMENTS", "@alice", "1/2", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("narrow view missing %q\n---\n%s", want, view)
		}
	}
}

func TestKittyStaysSingleColumn(t *testing.T) {
	m := send(New(Config{Render: "kitty"}), feedLoadedMsg{videos: testFeed()})
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.useTwoColumn() {
		t.Error("kitty renderer must not use the two-column layout")
	}
}

func TestPaneCellsTwoColumnSquareAndClamp(t *testing.T) {
	// Roomy width: pane is square (cols = 2*rows), side keeps >= sideMinW.
	m := wide()
	cols, rows := m.paneCells()
	if rows != 40-twoColChromeRows {
		t.Fatalf("rows=%d want %d", rows, 40-twoColChromeRows)
	}
	if cols != 2*rows {
		t.Errorf("square pane: cols=%d want %d (2*rows)", cols, 2*rows)
	}
	if inner := m.width - cols - 4; inner < sideMinW {
		t.Errorf("side inner width %d < sideMinW %d", inner, sideMinW)
	}

	// Tight width: square would starve the side column, so cols is clamped.
	mt := send(New(Config{}), feedLoadedMsg{videos: testFeed()})
	mt = send(mt, tea.WindowSizeMsg{Width: 85, Height: 40})
	cols, rows = mt.paneCells()
	wantCols := 85 - sideMinW - 4
	if cols != wantCols {
		t.Errorf("clamped pane: cols=%d want %d", cols, wantCols)
	}
	if inner := mt.width - cols - 4; inner != sideMinW {
		t.Errorf("clamped side inner width=%d want %d", inner, sideMinW)
	}
}

func TestHumanCount(t *testing.T) {
	cases := map[int64]string{42: "42", 1500: "1.5K", 2_300_000: "2.3M"}
	for in, want := range cases {
		if got := humanCount(in); got != want {
			t.Errorf("humanCount(%d)=%q want %q", in, got, want)
		}
	}
}
