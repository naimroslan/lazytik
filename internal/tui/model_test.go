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

func TestHumanCount(t *testing.T) {
	cases := map[int64]string{42: "42", 1500: "1.5K", 2_300_000: "2.3M"}
	for in, want := range cases {
		if got := humanCount(in); got != want {
			t.Errorf("humanCount(%d)=%q want %q", in, got, want)
		}
	}
}
