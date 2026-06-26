package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "starting lazytik…"
	}

	cur, ok := m.current()
	if !ok {
		msg := m.status
		if m.loading {
			msg = "loading feed…"
		} else if msg == "" {
			msg = "no videos in feed"
		}
		body := placeholderStyle.Width(m.width).Render(msg)
		return body + "\n" + m.footer()
	}

	// Header: @author · ❤ likes · position.
	likes := ""
	if cur.Likes >= 0 {
		likes = "  " + likesStyle.Render("♥ "+humanCount(cur.Likes))
	}
	pos := footerStyle.Render(fmt.Sprintf("  %d/%d", m.index+1, len(m.feed)))
	header := headerStyle.Render("@"+cur.Author) + likes + pos

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

	caption := captionStyle.Width(m.width).Render(truncate(cur.Caption, m.width))

	parts := []string{header, pane, caption}
	if m.status != "" {
		parts = append(parts, statusStyle.Render(m.status))
	}
	parts = append(parts, m.footer())
	return strings.Join(parts, "\n")
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
