package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (m Model) View() string {
	if m.loading {
		return "Loading..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	if m.width == 0 {
		return m.viewSimple()
	}

	return m.viewTwoColumn()
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
			b.WriteString(fmt.Sprintf("%s %s (retention: %dd, size: %dB)\n", cursor, g.Name, g.RetentionDays, g.StoredBytes))
		}
	case viewStreams:
		b.WriteString(fmt.Sprintf("Log Streams — %s\n\n", m.selectedGroup))
		for i, s := range m.sortedStreams(m.filteredStreams()) {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			mark := " "
			if m.selected[s.Name] {
				mark = "*"
			}
			b.WriteString(fmt.Sprintf("%s%s %s (%s, %s)\n", cursor, mark, s.Name, s.LastEventTimestamp.Format("2006-01-02 15:04:05"), formatBytes(s.StoredBytes)))
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
		Width(leftWidth - 2).
		Height(contentHeight).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder())

	rightStyle := lipgloss.NewStyle().
		Width(rightWidth - 2).
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
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

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
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, groups[i].Name))
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
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, m.logGroups[i].Name))
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
		b.WriteString(fmt.Sprintf("Streams — %s\n", m.selectedGroup))
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
		b.WriteString(fmt.Sprintf("%s%s %s  %s  %s\n", cursor, mark, s.Name, lastEvent, formatBytes(s.StoredBytes)))
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
	return fmt.Sprintf(" Sort: %s | q: quit | /: search | s: sort", sortStr)
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

