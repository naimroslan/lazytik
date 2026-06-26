package tui

import (
	"testing"

	"github.com/naimroslan/lazytik/internal/scraper"
)

// feed ids "a" (0) and "b" (1) come from testFeed(); helpers vids/send/loaded
// live in feed_test.go / model_test.go.

func TestWindowIDs(t *testing.T) {
	m := Model{feed: vids("0", "1", "2", "3", "4"), index: 2}
	w := m.windowIDs() // offsets -1,0,1,2 → indices 1,2,3,4
	want := map[string]bool{"1": true, "2": true, "3": true, "4": true}
	if len(w) != len(want) {
		t.Fatalf("window size %d, want %d: %v", len(w), len(want), w)
	}
	for id := range want {
		if !w[id] {
			t.Errorf("window missing %q", id)
		}
	}
	if w["0"] {
		t.Error("index 0 is outside the window at index 2 and should be excluded")
	}
}

func newTestModel(ids ...string) Model {
	m := New(Config{})
	m.feed = vids(ids...)
	m.width, m.height = 0, 0 // keep startCurrent a no-op (no real decode in tests)
	return m
}

func TestDownloadDoneCachesNeighbourInWindow(t *testing.T) {
	m := newTestModel("a", "b", "c") // current = a (index 0); b is in window
	m.downloading["b"] = true
	m = send(m, downloadDoneMsg{videoID: "b", path: "/tmp/lazytik-b.mp4"})
	if m.files["b"] != "/tmp/lazytik-b.mp4" {
		t.Errorf("expected b cached, got %q", m.files["b"])
	}
	if m.downloading["b"] {
		t.Error("downloading flag should be cleared")
	}
}

func TestDownloadDoneOutOfWindowDiscarded(t *testing.T) {
	m := newTestModel("a", "b", "c", "d", "e") // window at 0 = {a,b,c}
	m = send(m, downloadDoneMsg{videoID: "e", path: "/tmp/lazytik-e.mp4"})
	if _, ok := m.files["e"]; ok {
		t.Error("a clip outside the window must not be cached")
	}
}

func TestDownloadDoneNoVideoMarks(t *testing.T) {
	m := newTestModel("a", "b", "c")
	m.downloading["b"] = true
	m = send(m, downloadDoneMsg{videoID: "b", err: scraper.ErrNoVideo})
	if !m.noVideoIDs["b"] {
		t.Error("ErrNoVideo should record the id as video-less")
	}
}

func TestNavSettledStaleIgnored(t *testing.T) {
	m := loaded() // sized + fed; startCurrent already bumped gen
	m.navSeq = 5
	before := m.gen
	m = send(m, navSettledMsg{seq: 3}) // stale (≠ navSeq)
	if m.gen != before {
		t.Errorf("stale navSettled should be ignored; gen changed %d→%d", before, m.gen)
	}
	m = send(m, navSettledMsg{seq: 5}) // current → starts, bumps gen
	if m.gen == before {
		t.Error("matching navSettled should start playback (bump gen)")
	}
}

func TestPrefetchWindowMarksNeighbours(t *testing.T) {
	m := newTestModel("a", "b", "c", "d")
	m.index = 1 // window neighbours: c(+1)... a(-1), d(+2)
	m2, cmds := m.prefetchWindow()
	if len(cmds) == 0 {
		t.Fatal("expected prefetch commands for neighbours")
	}
	for _, id := range []string{"a", "c", "d"} {
		if !m2.downloading[id] {
			t.Errorf("neighbour %q should be marked downloading", id)
		}
	}
	if m2.downloading["b"] {
		t.Error("the current video b should not be prefetched as a neighbour")
	}
}
