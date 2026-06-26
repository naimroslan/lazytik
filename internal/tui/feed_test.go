package tui

import (
	"sort"
	"strings"
	"testing"

	"github.com/naimroslan/lazytik/internal/scraper"
)

func vids(ids ...string) []scraper.Video {
	out := make([]scraper.Video, len(ids))
	for i, id := range ids {
		out[i] = scraper.Video{ID: id}
	}
	return out
}

func ids(vs []scraper.Video) string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = v.ID
	}
	return strings.Join(s, ",")
}

func TestMixFeedInterleavesRoundRobin(t *testing.T) {
	lists := [][]scraper.Video{
		vids("a1", "a2", "a3"),
		vids("b1", "b2"),
	}
	got := ids(mixFeed(lists, false))
	want := "a1,b1,a2,b2,a3"
	if got != want {
		t.Errorf("interleave = %q, want %q", got, want)
	}
}

func TestMixFeedDedupesByID(t *testing.T) {
	lists := [][]scraper.Video{
		vids("x", "y"),
		vids("x", "z"), // x repeats across sources
	}
	got := mixFeed(lists, false)
	if len(got) != 3 {
		t.Fatalf("got %d videos %q, want 3 unique", len(got), ids(got))
	}
}

func TestMixFeedSingleList(t *testing.T) {
	got := ids(mixFeed([][]scraper.Video{vids("a", "b", "c")}, false))
	if got != "a,b,c" {
		t.Errorf("single list should pass through in order, got %q", got)
	}
}

func TestMixFeedShufflePreservesSet(t *testing.T) {
	lists := [][]scraper.Video{vids("a", "b", "c", "d", "e", "f", "g", "h")}
	got := mixFeed(lists, true)
	gotIDs := strings.Split(ids(got), ",")
	sort.Strings(gotIDs)
	if strings.Join(gotIDs, ",") != "a,b,c,d,e,f,g,h" {
		t.Errorf("shuffle changed the set of videos: %q", ids(got))
	}
}

func TestMixFeedEmpty(t *testing.T) {
	if got := mixFeed(nil, true); got != nil {
		t.Errorf("empty input should give nil, got %v", got)
	}
}
