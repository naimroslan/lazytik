package scraper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoVideo means the post has no downloadable video stream (e.g. a photo/
// slideshow post that yt-dlp exposes as audio-only). Callers should skip it.
var ErrNoVideo = errors.New("post has no video stream")

// videoFormat prefers a combined stream that already carries video+audio, capped
// at 720p (plenty for half-block rendering and faster to fetch); falls back to
// any format that has video. Audio-only posts match nothing → yt-dlp errors.
const videoFormat = "best[vcodec!=none][height<=720]/best[vcodec!=none]/bv*[height<=720]+ba"

// ytEntry mirrors the subset of yt-dlp's JSON we consume. Flat-playlist mode
// already returns uploader/like_count/description for TikTok, so listing a feed
// needs no per-video calls — only the (time-limited) stream URL is fetched later.
type ytEntry struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`         // page URL in --flat-playlist output
	WebpageURL  string   `json:"webpage_url"` // page URL in full extraction
	Uploader    string   `json:"uploader"`
	Channel     string   `json:"channel"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	LikeCount   *int64   `json:"like_count"`
	Duration    *float64 `json:"duration"`
}

// ytRoot decodes either a playlist (Entries populated) or a single video (the
// embedded ytEntry fields populated), distinguished by Type.
type ytRoot struct {
	Type    string    `json:"_type"`
	Entries []ytEntry `json:"entries"`
	ytEntry           // single-video case: the root *is* the entry
}

func (e ytEntry) toVideo() Video {
	page := e.URL
	if page == "" {
		page = e.WebpageURL
	}
	author := e.Uploader
	if author == "" {
		author = e.Channel
	}
	caption := e.Description
	if caption == "" {
		caption = e.Title
	}
	likes := int64(-1)
	if e.LikeCount != nil {
		likes = *e.LikeCount
	}
	dur := 0
	if e.Duration != nil {
		dur = int(*e.Duration)
	}
	return Video{
		ID:       e.ID,
		PageURL:  page,
		Author:   strings.TrimPrefix(author, "@"),
		Caption:  caption,
		Likes:    likes,
		Duration: dur,
	}
}

// SourceURL maps a user-facing argument (@user, #hashtag, or a URL) to the
// TikTok URL yt-dlp should extract.
func SourceURL(arg string) string {
	switch {
	case strings.HasPrefix(arg, "http://"), strings.HasPrefix(arg, "https://"):
		return arg
	case strings.HasPrefix(arg, "@"):
		return "https://www.tiktok.com/" + arg
	case strings.HasPrefix(arg, "#"):
		return "https://www.tiktok.com/tag/" + strings.TrimPrefix(arg, "#")
	default:
		// Bare word: treat as a username.
		return "https://www.tiktok.com/@" + arg
	}
}

// parseFeed turns yt-dlp -J output into videos. Exposed (unexported) for testing
// against fixtures without invoking the network.
func parseFeed(data []byte) ([]Video, error) {
	var root ytRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse yt-dlp json: %w", err)
	}
	if len(root.Entries) > 0 {
		vids := make([]Video, 0, len(root.Entries))
		for _, e := range root.Entries {
			if e.ID == "" {
				continue
			}
			vids = append(vids, e.toVideo())
		}
		return vids, nil
	}
	if root.ID != "" { // single-video extraction
		return []Video{root.ytEntry.toVideo()}, nil
	}
	return nil, fmt.Errorf("no videos found in yt-dlp output")
}

// ListFeed resolves a single source (@user / #hashtag / URL) into videos,
// listing at most limit items. StreamURL is not populated; call Download to
// fetch a playable file for a given video.
func ListFeed(ctx context.Context, ytdlp, source string, limit int) ([]Video, error) {
	args := []string{"-J", "--flat-playlist"}
	if limit > 0 {
		args = append(args, "--playlist-end", fmt.Sprintf("%d", limit))
	}
	args = append(args, SourceURL(source))

	out, err := run(ctx, ytdlp, args...)
	if err != nil {
		return nil, err
	}
	return parseFeed(out)
}

// Download fetches a video to destPath. yt-dlp applies the headers/cookies the
// TikTok CDN requires (a bare stream URL handed to ffmpeg gets a 403), so the
// resulting local file is decodable and seekable (loops cleanly). Returns
// ErrNoVideo when the post has no video stream.
func Download(ctx context.Context, ytdlp, pageURL, destPath string) error {
	_, err := run(ctx, ytdlp,
		"-q", "--no-warnings",
		"--force-overwrites",           // never skip a pre-existing destination
		"--merge-output-format", "mp4", // if the bv*+ba fallback merges, land at .mp4
		"-f", videoFormat,
		"-o", destPath,
		pageURL,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Requested format is not available") {
			return ErrNoVideo
		}
		return err
	}
	return nil
}

// run executes yt-dlp and returns stdout, wrapping failures with stderr context.
func run(ctx context.Context, ytdlp string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, ytdlp, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("yt-dlp %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}
