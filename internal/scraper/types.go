// Package scraper resolves TikTok feeds and per-video stream URLs by wrapping
// the external yt-dlp tool. lazytik does not implement TikTok extraction itself;
// yt-dlp carries that (constantly-shifting) burden.
package scraper

// Video is a single item in a feed. Cheap fields (ID, PageURL, Author, Caption)
// are populated when the feed is listed; StreamURL is resolved lazily and is
// time-limited, so it must be fetched shortly before playback.
type Video struct {
	ID        string // TikTok video id
	PageURL   string // canonical watch URL, e.g. https://www.tiktok.com/@user/video/123
	StreamURL string // direct media URL (expires); empty until Resolve'd
	Author    string // uploader handle, without leading @
	Caption   string // video description / caption
	Likes     int64  // like_count, -1 if unknown
	Duration  int    // seconds, 0 if unknown
}

// Title returns a short human label for the video, used in list views.
func (v Video) Title() string {
	if v.Caption != "" {
		return v.Caption
	}
	return v.PageURL
}
