// Package deps detects the external command-line tools lazytik orchestrates
// (yt-dlp, mpv, ffmpeg) and produces friendly install hints when they are missing.
package deps

import (
	"os/exec"
	"strings"
)

// Tool describes one external dependency.
type Tool struct {
	Name     string // executable name as found on PATH
	Purpose  string // one-line explanation of why lazytik needs it
	Required bool   // if false, lazytik degrades gracefully without it
}

// Status is the result of probing for a Tool.
type Status struct {
	Tool
	Found bool
	Path  string
}

// Tools is the full set lazytik may use, in display order.
var Tools = []Tool{
	{Name: "yt-dlp", Purpose: "resolve TikTok feeds and stream URLs", Required: true},
	{Name: "mpv", Purpose: "play audio and fullscreen video", Required: true},
	{Name: "ffmpeg", Purpose: "decode frames for the embedded video pane", Required: true},
}

// Check probes every entry in Tools and reports its status.
func Check() []Status {
	out := make([]Status, 0, len(Tools))
	for _, t := range Tools {
		s := Status{Tool: t}
		if p, err := exec.LookPath(t.Name); err == nil {
			s.Found = true
			s.Path = p
		}
		out = append(out, s)
	}
	return out
}

// MissingRequired returns the names of required tools that were not found.
func MissingRequired(statuses []Status) []string {
	var missing []string
	for _, s := range statuses {
		if s.Required && !s.Found {
			missing = append(missing, s.Name)
		}
	}
	return missing
}

// InstallHint returns the install command for the given missing tool names using
// the host's detected package manager, or a manual fallback if none is found.
// Returns "" if nothing is missing.
func InstallHint(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if m, ok := DetectManager(); ok {
		return m.CommandString(names)
	}
	return "install manually: " + strings.Join(names, " ")
}
