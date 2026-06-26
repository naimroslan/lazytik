// Command lazytik plays a scrollable TikTok feed in the terminal.
//
// Usage:
//
//	lazytik @username        feed of a creator's videos
//	lazytik '#funny'         feed for a hashtag
//	lazytik <url> [url...]    explicit TikTok video URLs
//	lazytik doctor           check that yt-dlp / mpv / ffmpeg are installed
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/naimroslan/lazytik/internal/deps"
	"github.com/naimroslan/lazytik/internal/termcap"
	"github.com/naimroslan/lazytik/internal/tui"
)

func main() {
	fs := flag.NewFlagSet("lazytik", flag.ExitOnError)
	fullscreen := fs.Bool("fullscreen", false, "play via fullscreen mpv handoff instead of the embedded pane")
	fps := fs.Int("fps", 24, "target playback fps for the embedded video pane")
	limit := fs.Int("limit", 30, "max videos to list per source (0 = no limit; slow for big accounts)")
	render := fs.String("render", "auto", "embedded renderer: auto|halfblock|kitty")
	vo := fs.String("vo", "auto", "mpv video output for --fullscreen: auto|tct|sixel|kitty")
	shuffle := fs.Bool("shuffle", false, "shuffle the mixed feed (great with multiple creators)")
	fs.Usage = usage

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "doctor" {
		os.Exit(runDoctor())
	}
	_ = fs.Parse(args)
	sources := fs.Args()

	statuses := deps.Check()
	if missing := deps.MissingRequired(statuses); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "lazytik needs: %v\n", missing)
		fmt.Fprintf(os.Stderr, "install them with:\n  %s\n\n", deps.InstallHint(missing))
		fmt.Fprintln(os.Stderr, "run `lazytik doctor` for details.")
		os.Exit(1)
	}
	if len(sources) == 0 {
		usage()
		os.Exit(2)
	}

	gfx := termcap.Detect()
	cfg := tui.Config{
		Sources:    sources,
		Ytdlp:      lookup("yt-dlp"),
		Mpv:        lookup("mpv"),
		FFmpeg:     lookup("ffmpeg"),
		Limit:      *limit,
		FPS:        *fps,
		Fullscreen: *fullscreen,
		Render:     resolveRender(*render, gfx),
		Vo:         resolveVo(*vo, gfx),
		Shuffle:    *shuffle,
	}

	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazytik:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: lazytik [flags] @user [@user2 ...] | '#hashtag' | <url>...")
	fmt.Fprintln(os.Stderr, "       lazytik --shuffle @a @b @c     # a mixed, FYP-like feed")
	fmt.Fprintln(os.Stderr, "       lazytik doctor")
	fmt.Fprintln(os.Stderr, "\nflags:")
	fmt.Fprintln(os.Stderr, "  --fullscreen   play via mpv handoff instead of the embedded pane")
	fmt.Fprintln(os.Stderr, "  --fps N        target playback fps (default 24)")
	fmt.Fprintln(os.Stderr, "  --limit N      max videos per source (default 30; 0 = no limit)")
	fmt.Fprintln(os.Stderr, "  --render MODE  embedded renderer: auto|halfblock|kitty")
	fmt.Fprintln(os.Stderr, "  --vo MODE      mpv output for --fullscreen: auto|tct|sixel|kitty")
	fmt.Fprintln(os.Stderr, "  --shuffle      shuffle a multi-creator mixed feed")
}

// resolveRender picks the embedded renderer backend. "auto" uses the kitty
// graphics protocol only when the terminal is detected to support it.
func resolveRender(flagVal string, gfx termcap.Graphics) string {
	if flagVal == "auto" {
		if gfx == termcap.Kitty {
			return "kitty"
		}
		return "halfblock"
	}
	return flagVal
}

// resolveVo picks mpv's video output for the fullscreen handoff. "auto" matches
// the detected terminal capability, falling back to tct (half-blocks) anywhere.
func resolveVo(flagVal string, gfx termcap.Graphics) string {
	if flagVal != "auto" {
		return flagVal
	}
	switch gfx {
	case termcap.Kitty:
		return "kitty"
	case termcap.Sixel:
		return "sixel"
	default:
		return "tct"
	}
}

// lookup resolves a tool's path; deps.Check already confirmed it exists.
func lookup(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

// runDoctor prints dependency status and returns a process exit code.
func runDoctor() int {
	fmt.Println("lazytik doctor — external dependencies:")
	statuses := deps.Check()
	for _, s := range statuses {
		mark, where := "✗", "not found"
		if s.Found {
			mark, where = "✓", s.Path
		}
		fmt.Printf("  %s %-8s %-45s (%s)\n", mark, s.Name, where, s.Purpose)
	}
	gfx := termcap.Detect()
	fmt.Printf("\nterminal graphics: %s", gfx)
	switch gfx {
	case termcap.Kitty:
		fmt.Println("  → embedded pane uses crisp kitty pixels")
	case termcap.Sixel:
		fmt.Println("  → use --fullscreen for crisp sixel video (embedded pane stays half-blocks)")
	default:
		fmt.Println("  → half-blocks (force with --render kitty / --vo sixel if your terminal supports it)")
	}

	if hint := deps.InstallHint(deps.MissingRequired(statuses)); hint != "" {
		fmt.Printf("\ninstall missing tools with:\n  %s\n", hint)
		return 1
	}
	fmt.Println("\nall set — lazytik is ready to roll.")
	return 0
}
