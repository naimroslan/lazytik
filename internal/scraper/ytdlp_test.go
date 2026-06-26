package scraper

import "testing"

// A trimmed but structurally-real --flat-playlist response (two entries).
const flatPlaylistJSON = `{
  "_type": "playlist",
  "id": "tiktok",
  "entries": [
    {"id":"111","url":"https://www.tiktok.com/@tiktok/video/111","uploader":"tiktok","channel":"TikTok","title":"hi","description":"first clip","like_count":2142,"duration":113.0},
    {"id":"222","url":"https://www.tiktok.com/@tiktok/video/222","uploader":"tiktok","title":"","description":"second clip","like_count":null,"duration":7.5}
  ]
}`

// A single-video extraction (no entries; root is the video).
const singleVideoJSON = `{
  "_type": "video",
  "id":"999","webpage_url":"https://www.tiktok.com/@bob/video/999",
  "uploader":"@bob","title":"only title","like_count":5,"duration":12.0
}`

func TestParseFeedPlaylist(t *testing.T) {
	vids, err := parseFeed([]byte(flatPlaylistJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(vids) != 2 {
		t.Fatalf("got %d videos, want 2", len(vids))
	}
	v := vids[0]
	if v.ID != "111" || v.Author != "tiktok" || v.Caption != "first clip" {
		t.Errorf("entry0 mapped wrong: %+v", v)
	}
	if v.Likes != 2142 || v.Duration != 113 {
		t.Errorf("entry0 numbers wrong: likes=%d dur=%d", v.Likes, v.Duration)
	}
	// Null like_count must become -1 (unknown), not 0.
	if vids[1].Likes != -1 {
		t.Errorf("null like_count should be -1, got %d", vids[1].Likes)
	}
}

func TestParseFeedSingleVideo(t *testing.T) {
	vids, err := parseFeed([]byte(singleVideoJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(vids) != 1 {
		t.Fatalf("got %d videos, want 1", len(vids))
	}
	v := vids[0]
	if v.ID != "999" || v.PageURL != "https://www.tiktok.com/@bob/video/999" {
		t.Errorf("single video mapped wrong: %+v", v)
	}
	// "@bob" should have its leading @ trimmed; caption falls back to title.
	if v.Author != "bob" || v.Caption != "only title" {
		t.Errorf("author/caption fallback wrong: %+v", v)
	}
}

func TestParseFeedEmpty(t *testing.T) {
	if _, err := parseFeed([]byte(`{"_type":"playlist","entries":[]}`)); err == nil {
		t.Error("expected error for empty feed")
	}
}

func TestSourceURL(t *testing.T) {
	cases := map[string]string{
		"@tiktok":                           "https://www.tiktok.com/@tiktok",
		"#funny":                            "https://www.tiktok.com/tag/funny",
		"tiktok":                            "https://www.tiktok.com/@tiktok",
		"https://www.tiktok.com/@a/video/1": "https://www.tiktok.com/@a/video/1",
	}
	for in, want := range cases {
		if got := SourceURL(in); got != want {
			t.Errorf("SourceURL(%q)=%q want %q", in, got, want)
		}
	}
}
