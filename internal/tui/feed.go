package tui

import (
	"math/rand"

	"github.com/naimroslan/lazytik/internal/scraper"
)

// mixFeed combines per-source video lists into one feed. Lists are interleaved
// round-robin (source A's 1st, B's 1st, …, then each 2nd, …) so no single
// creator dominates the top, duplicates (same video id) are dropped, and when
// shuffle is set the result is randomized for a fresh order each run.
func mixFeed(lists [][]scraper.Video, shuffle bool) []scraper.Video {
	var out []scraper.Video
	seen := make(map[string]bool)

	maxLen := 0
	for _, l := range lists {
		if len(l) > maxLen {
			maxLen = len(l)
		}
	}
	for i := 0; i < maxLen; i++ {
		for _, l := range lists {
			if i >= len(l) {
				continue
			}
			v := l[i]
			if v.ID == "" || seen[v.ID] {
				continue
			}
			seen[v.ID] = true
			out = append(out, v)
		}
	}

	if shuffle {
		rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	}
	return out
}
