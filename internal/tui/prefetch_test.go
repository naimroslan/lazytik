package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/scraper"
)

// feed ids are "a" (index 0) and "b" (index 1) from testFeed().

func TestPrefetchedFileUsedAndConsumed(t *testing.T) {
	m := loaded()
	m.prefetched["b"] = "/tmp/lazytik-fake-b.mp4"
	// Scroll down to "b"; startCurrent should consume the prefetched entry.
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	if _, ok := m.prefetched["b"]; ok {
		t.Error("prefetched entry for current video should be consumed (deleted)")
	}
}

func TestPrefetchDoneStoresForUpcoming(t *testing.T) {
	m := loaded() // index 0; upcoming is "b"
	m.prefetching["b"] = true
	m = send(m, prefetchDoneMsg{videoID: "b", path: "/tmp/lazytik-b.mp4"})
	if m.prefetched["b"] != "/tmp/lazytik-b.mp4" {
		t.Errorf("expected prefetched[b] stored, got %q", m.prefetched["b"])
	}
	if m.prefetching["b"] {
		t.Error("prefetching flag should be cleared when done")
	}
}

func TestPrefetchDoneDiscardsStale(t *testing.T) {
	m := loaded() // index 0; upcoming is "b", not "zzz"
	m = send(m, prefetchDoneMsg{videoID: "zzz", path: "/tmp/lazytik-zzz.mp4"})
	if _, ok := m.prefetched["zzz"]; ok {
		t.Error("a download that is no longer the upcoming video must be discarded")
	}
}

func TestPrefetchNoVideoMarksSkip(t *testing.T) {
	m := loaded()
	m.prefetching["b"] = true
	m = send(m, prefetchDoneMsg{videoID: "b", err: scraper.ErrNoVideo})
	if !m.noVideoIDs["b"] {
		t.Error("ErrNoVideo during prefetch should record the id as video-less")
	}
}

func TestEvictPrefetchExcept(t *testing.T) {
	m := loaded()
	m.prefetched["keep"] = "/tmp/keep.mp4"
	m.prefetched["drop"] = "/tmp/drop.mp4"
	m.evictPrefetchExcept("keep")
	if _, ok := m.prefetched["keep"]; !ok {
		t.Error("keep should remain")
	}
	if _, ok := m.prefetched["drop"]; ok {
		t.Error("drop should be evicted")
	}
}

func TestMaybePrefetchNextMarksInFlight(t *testing.T) {
	m := loaded() // index 0; next is "b"
	m, cmd := m.maybePrefetchNext()
	if !m.prefetching["b"] {
		t.Error("maybePrefetchNext should mark the next video as in-flight")
	}
	if cmd == nil {
		t.Error("expected a prefetch command for the next video")
	}
	// Calling again must not re-issue while in flight.
	m2, cmd2 := m.maybePrefetchNext()
	if cmd2 != nil {
		t.Error("should not prefetch again while already in flight")
	}
	_ = m2
}
