package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/naimroslan/lazytik/internal/scraper"
)

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FE2C55")) // TikTok red
	likesStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#25F4EE"))            // TikTok cyan
	captionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	paneStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
	placeholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Align(lipgloss.Center)
)

// spinnerFrames is a braille spinner shown while the feed loads.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "starting lazytik…"
	}

	cur, ok := m.current()
	if !ok {
		msg := m.status
		if m.loading {
			msg = spinnerFrames[m.spinnerFrame%len(spinnerFrames)] + " loading feed…"
		} else if msg == "" {
			msg = "no videos in feed"
		}
		body := placeholderStyle.Width(m.width).Render(msg)
		return body + "\n" + m.footer()
	}

	// @author · ♥ likes · position — shown at the top of the caption section.
	info := m.infoLine(cur)

	if m.useTwoColumn() {
		return m.viewTwoColumn(cur, info)
	}
	return m.viewSingleColumn(cur, info)
}

// infoLine renders "@author · ♥ likes · N/M" for the focused video.
func (m Model) infoLine(cur scraper.Video) string {
	likes := ""
	if cur.Likes >= 0 {
		likes = "  " + likesStyle.Render("♥ "+humanCount(cur.Likes))
	}
	pos := footerStyle.Render(fmt.Sprintf("  %d/%d", m.index+1, len(m.feed)))
	return headerStyle.Render("@"+cur.Author) + likes + pos
}

// viewTwoColumn renders the wide desktop layout: a roughly-square video pane on
// the left and a caption/comments column on the right, separated by the boxes'
// borders. The CAPTION section is pinned to the top of the right column.
func (m Model) viewTwoColumn(cur scraper.Video, info string) string {
	paneW, paneH := m.paneCells()
	left := paneStyle.Width(paneW).Height(paneH).Render(m.paneContent(paneW, paneH))

	// The two bordered boxes consume 4 border columns total; the rest is the
	// right column's content width.
	innerW := m.width - paneW - 4
	if innerW < 1 {
		innerW = 1
	}

	// Wrap the caption, then clamp to the rows left after the section labels so a
	// long caption can't push the right box taller than the video pane.
	capRows := paneH - 5
	caption := clampLines(captionStyle.Width(innerW).Render(cur.Caption), capRows)

	side := lipgloss.JoinVertical(
		lipgloss.Left,
		sectionDivider("CAPTION", innerW),
		info,
		caption,
		"",
		sectionDivider("COMMENTS", innerW),
		placeholderStyle.Width(innerW).Render("comments coming soon"),
	)
	right := paneStyle.Width(innerW).Height(paneH).Render(side)

	parts := []string{lipgloss.JoinHorizontal(lipgloss.Top, left, right)}
	if m.status != "" {
		parts = append(parts, statusStyle.Render(m.status))
	}
	parts = append(parts, m.footer())
	return strings.Join(parts, "\n")
}

// viewSingleColumn renders the narrow / kitty fallback: video pane stacked above
// the caption and comments sections, each introduced by a labelled separator.
func (m Model) viewSingleColumn(cur scraper.Video, info string) string {
	paneW, paneH := m.paneCells()
	var pane string
	if m.frame != "" && m.renderer.Name() == "kitty" {
		// The frame is a kitty graphics escape placed with C=1 (cursor unmoved),
		// so reserve exactly paneH rows for the image to overlay; without this,
		// Lipgloss would treat the escape as empty and collapse it to one line.
		pane = m.frame + strings.Repeat("\n", paneH-1)
	} else {
		pane = paneStyle.Width(paneW).Height(paneH).Render(m.paneContent(paneW, paneH))
	}

	parts := []string{
		pane,
		sectionDivider("CAPTION", m.width),
		info,
		captionStyle.Width(m.width).Render(truncate(cur.Caption, m.width)),
		sectionDivider("COMMENTS", m.width),
		placeholderStyle.Width(m.width).Render("comments coming soon"),
	}
	if m.status != "" {
		parts = append(parts, statusStyle.Render(m.status))
	}
	parts = append(parts, m.footer())
	return strings.Join(parts, "\n")
}

// sectionDivider draws a dim, labelled rule filling width w, e.g.
// "── CAPTION ─────────────".
func sectionDivider(label string, w int) string {
	head := "── " + label + " "
	n := w - lipgloss.Width(head)
	if n < 0 {
		n = 0
	}
	return footerStyle.Render(head + strings.Repeat("─", n))
}

// clampLines truncates rendered text to at most n lines, marking the cut with an
// ellipsis. n <= 0 yields the empty string.
func clampLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	lines = lines[:n]
	lines[n-1] += "…"
	return strings.Join(lines, "\n")
}

// paneContent is the inside of the video pane. In M0 there is no real video, so
// we show a centered placeholder; M2 fills this with rendered half-block frames.
func (m Model) paneContent(w, h int) string {
	if m.frame != "" {
		return m.frame
	}
	var msg string
	switch {
	case m.cfg.Fullscreen:
		msg = "press enter to play fullscreen"
	case m.status != "":
		msg = m.status
	default:
		msg = "buffering…"
	}
	return placeholderStyle.Width(w).Height(h).Render(msg)
}

func (m Model) footer() string {
	return footerStyle.Render("↑/↓ next·prev   enter play   space pause   q quit")
}

// truncate shortens s to fit width w, adding an ellipsis.
func truncate(s string, w int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if w <= 1 || len([]rune(s)) <= w {
		return s
	}
	r := []rune(s)
	return string(r[:w-1]) + "…"
}

// humanCount formats large counts like TikTok does: 12.3K, 4.5M.
func humanCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
