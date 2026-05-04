package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View implements tea.Model.
func (m Model) View() string {
	// Show full-screen error only when no data is loaded (initial load failure).
	// Otherwise, errors are shown in the status bar.
	if m.err != nil && len(m.logGroups) == 0 {
		return m.renderErrorView()
	}

	base := m.renderBaseView()
	if m.loading {
		return overlayPopup(base, m.renderLoadingPopup(), m.width, m.height)
	}
	return base
}

// renderErrorView renders a full-screen startup error wrapped to the
// terminal width so long AWS error messages don't run off the alt screen.
func (m Model) renderErrorView() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	body := lipgloss.NewStyle().
		Width(width).
		Render(fmt.Sprintf("Error: %v", m.err))
	return body + "\n\nPress q to quit."
}

// renderBaseView returns the underlying view without any loading overlay.
func (m Model) renderBaseView() string {
	if m.currentView == viewTail {
		return m.renderTailView()
	}
	if m.width == 0 {
		return m.viewSimple()
	}
	return m.viewTwoColumn()
}

func (m Model) renderLoadingPopup() string {
	msg := m.loadingMessage
	if msg == "" {
		msg = "Loading..."
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 2).
		Render(msg)
}

// overlayPopup places popup centered over base. Width and height are the
// terminal dimensions; the popup is anchored to the center cell. ANSI codes
// in base are preserved by ansi.Cut.
func overlayPopup(base, popup string, width, height int) string {
	if width == 0 || height == 0 {
		// No window size yet — fall back to showing the popup alone.
		return popup
	}

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")

	popupH := len(popupLines)
	popupW := 0
	for _, l := range popupLines {
		if w := ansi.StringWidth(l); w > popupW {
			popupW = w
		}
	}

	startRow := (height - popupH) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (width - popupW) / 2
	if startCol < 0 {
		startCol = 0
	}

	for i, popupLine := range popupLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		baseLines[row] = overlayLineAt(baseLines[row], popupLine, startCol, popupW, width)
	}
	return strings.Join(baseLines, "\n")
}

// overlayLineAt replaces the visual columns [col, col+popupW) of base with
// popup, preserving styled segments outside that range. termWidth caps the
// returned line to the terminal width.
func overlayLineAt(base, popup string, col, popupW, termWidth int) string {
	left := ansi.Cut(base, 0, col)
	leftW := ansi.StringWidth(left)
	if leftW < col {
		left += strings.Repeat(" ", col-leftW)
	}
	right := ansi.Cut(base, col+popupW, termWidth)
	return left + popup + right
}

func (m Model) viewSimple() string {
	var b strings.Builder

	switch m.currentView {
	case viewGroups:
		b.WriteString("Log Groups\n\n")
		for i, g := range m.filteredGroups() {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s (retention: %dd, size: %dB)\n", cursor, g.Name, g.RetentionDays, g.StoredBytes)
		}
	case viewStreams:
		fmt.Fprintf(&b, "Log Streams — %s\n\n", m.selectedGroup)
		for i, s := range m.sortedStreams(m.filteredStreams()) {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			mark := " "
			if m.selected[s.Name] {
				mark = "*"
			}
			fmt.Fprintf(&b, "%s%s %s (%s)\n", cursor, mark, s.Name, s.LastEventTimestamp.Format("2006-01-02 15:04:05"))
		}
	}

	b.WriteString(m.renderInputLine())
	b.WriteString("\nq: quit | j/k: move | l: enter | h: back | /: search | s: sort")

	return b.String()
}

func (m Model) viewTwoColumn() string {
	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth
	contentHeight := m.height - 4
	if m.mode != modeNormal {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	leftStyle := lipgloss.NewStyle().
		Width(leftWidth-2).
		Height(contentHeight).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder())

	rightStyle := lipgloss.NewStyle().
		Width(rightWidth-2).
		Height(contentHeight).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder())

	activeBorder := lipgloss.Color("62")
	inactiveBorder := lipgloss.Color("240")

	var leftCol, rightCol string

	switch m.currentView {
	case viewGroups:
		leftStyle = leftStyle.BorderForeground(activeBorder)
		rightStyle = rightStyle.BorderForeground(inactiveBorder)
		leftCol = m.renderGroupList(contentHeight)
		rightCol = m.renderStreamList(contentHeight)
	case viewStreams:
		leftStyle = leftStyle.BorderForeground(inactiveBorder)
		rightStyle = rightStyle.BorderForeground(activeBorder)
		leftCol = m.renderGroupListInactive(contentHeight)
		rightCol = m.renderStreamList(contentHeight)
	}

	left := leftStyle.Render(strings.TrimSuffix(leftCol, "\n"))
	right := rightStyle.Render(strings.TrimSuffix(rightCol, "\n"))

	// Build output line-by-line to avoid lipgloss JoinHorizontal/JoinVertical
	// adding unexpected padding that can cause overflow on some terminals.
	// Cap each pane to a fixed height (top border + contentHeight + bottom
	// border) because lipgloss.Height does not truncate wrapped content.
	expectedPaneLines := contentHeight + 2
	leftLines := capPaneLines(strings.Split(left, "\n"), expectedPaneLines)
	rightLines := capPaneLines(strings.Split(right, "\n"), expectedPaneLines)

	maxPaneLines := len(leftLines)
	if len(rightLines) > maxPaneLines {
		maxPaneLines = len(rightLines)
	}

	var b strings.Builder
	for i := 0; i < maxPaneLines; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		b.WriteString(l)
		b.WriteString(r)
		b.WriteString("\n")
	}

	if inputLine := m.renderInputLine(); inputLine != "" {
		b.WriteString(inputLine)
		b.WriteString("\n")
	}
	b.WriteString(m.renderStatusBar())

	// Hard-cap to m.height - 1 lines to prevent overflow on any terminal
	result := b.String()
	lines := strings.Split(result, "\n")
	maxLines := m.height - 1
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderGroupList(maxHeight int) string {
	var b strings.Builder
	b.WriteString("Log Groups\n")
	groups := m.filteredGroups()
	visible := maxHeight - 1
	end := m.offset + visible
	if end > len(groups) {
		end = len(groups)
	}
	lines := 1 // header
	for i := m.offset; i < end; i++ {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		fmt.Fprintf(&b, "%s %s\n", cursor, groups[i].Name)
		lines++
	}
	for lines < maxHeight {
		b.WriteString("\n")
		lines++
	}
	return b.String()
}

func (m Model) renderGroupListInactive(maxHeight int) string {
	var b strings.Builder
	b.WriteString("Log Groups\n")
	visible := maxHeight - 1
	// Use groupOffset to scroll the inactive list to show the selected group
	offset := m.groupOffset
	// Ensure the groupCursor is visible
	if m.groupCursor < offset {
		offset = m.groupCursor
	}
	if m.groupCursor >= offset+visible {
		offset = m.groupCursor - visible + 1
	}
	end := offset + visible
	if end > len(m.logGroups) {
		end = len(m.logGroups)
	}
	lines := 1 // header
	for i := offset; i < end; i++ {
		cursor := " "
		if i == m.groupCursor {
			cursor = ">"
		}
		fmt.Fprintf(&b, "%s %s\n", cursor, m.logGroups[i].Name)
		lines++
	}
	for lines < maxHeight {
		b.WriteString("\n")
		lines++
	}
	return b.String()
}

func (m Model) renderStreamList(maxHeight int) string {
	var b strings.Builder
	if m.selectedGroup != "" {
		fmt.Fprintf(&b, "Streams — %s\n", m.selectedGroup)
	} else {
		b.WriteString("Streams\n")
	}
	streams := m.sortedStreams(m.filteredStreams())
	visible := maxHeight - 1
	offset := 0
	if m.currentView == viewStreams {
		offset = m.offset
	}
	end := offset + visible
	if end > len(streams) {
		end = len(streams)
	}
	lines := 1 // header
	for i := offset; i < end; i++ {
		s := streams[i]
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		mark := " "
		if m.selected[s.Name] {
			mark = "*"
		}
		lastEvent := s.LastEventTimestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(&b, "%s%s %s  %s\n", cursor, mark, s.Name, lastEvent)
		lines++
	}
	for lines < maxHeight {
		b.WriteString("\n")
		lines++
	}
	return b.String()
}

func (m Model) renderInputLine() string {
	switch m.mode {
	case modeSearch:
		return fmt.Sprintf("/%s", m.searchQuery)
	}
	return ""
}

func (m Model) renderStatusBar() string {
	sortStr := "time ↓"
	if !m.sortDescending {
		sortStr = "time ↑"
	}
	var bar string
	if m.currentView == viewStreams {
		bar = fmt.Sprintf(" Sort: %s | q: quit | /: search | s: sort | f: follow", sortStr)
	} else {
		bar = fmt.Sprintf(" Sort: %s | q: quit | /: search | s: sort", sortStr)
	}
	if m.err != nil {
		bar = fmt.Sprintf(" Error: %v | %s", m.err, bar[1:])
	}
	return bar
}

// capPaneLines ensures a bordered pane has exactly n lines by keeping the
// first line (top border), the middle content lines up to the limit, and the
// last line (bottom border). This prevents lipgloss text wrapping from
// expanding the pane beyond the expected height.
func capPaneLines(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	// top border + (n-2) content lines + bottom border
	capped := make([]string, 0, n)
	capped = append(capped, lines[0])
	capped = append(capped, lines[1:n-1]...)
	capped = append(capped, lines[len(lines)-1])
	return capped
}
