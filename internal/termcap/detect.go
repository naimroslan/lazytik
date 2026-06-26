// Package termcap guesses a terminal's inline-image capability so lazytik can
// pick a renderer. Detection is best-effort from environment variables; over SSH
// most client env is hidden (only TERM is forwarded), so the --render/--vo flags
// let the user force a mode.
package termcap

import (
	"os"
	"strings"
)

// Graphics is a terminal's inline-image capability.
type Graphics int

const (
	None  Graphics = iota // text only → half-blocks
	Sixel                 // sixel graphics (via mpv --vo=sixel)
	Kitty                 // kitty graphics protocol (embedded renderer or mpv --vo=kitty)
)

func (g Graphics) String() string {
	switch g {
	case Kitty:
		return "kitty"
	case Sixel:
		return "sixel"
	default:
		return "none"
	}
}

// Parse maps a user-supplied name to a Graphics value.
func Parse(s string) (Graphics, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "kitty":
		return Kitty, true
	case "sixel":
		return Sixel, true
	case "none", "halfblock", "half-block", "tct":
		return None, true
	default:
		return None, false
	}
}

// Detect inspects the environment to guess inline-image support.
func Detect() Graphics {
	if v := os.Getenv("LAZYTIK_GRAPHICS"); v != "" {
		if g, ok := Parse(v); ok {
			return g
		}
	}
	term := strings.ToLower(os.Getenv("TERM"))
	prog := strings.ToLower(os.Getenv("TERM_PROGRAM"))

	// Kitty graphics protocol. TERM (xterm-kitty / xterm-ghostty) survives SSH;
	// the *_WINDOW_ID / *_PROGRAM vars only help locally.
	switch {
	case os.Getenv("KITTY_WINDOW_ID") != "",
		strings.Contains(term, "kitty"),
		strings.Contains(term, "ghostty"),
		prog == "ghostty",
		os.Getenv("WEZTERM_PANE") != "",
		prog == "wezterm":
		return Kitty
	}

	// Sixel-capable terminals.
	switch {
	case prog == "iterm.app",
		prog == "foot",
		strings.Contains(term, "foot"),
		strings.Contains(term, "sixel"),
		strings.Contains(term, "mlterm"):
		return Sixel
	}

	return None
}
