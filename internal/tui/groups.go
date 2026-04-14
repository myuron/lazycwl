package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myuron/lazycwl/internal/aws"
)

func (m Model) currentListLen() int {
	switch m.currentView {
	case viewGroups:
		return len(m.filteredGroups())
	case viewStreams:
		return len(m.filteredStreams())
	}
	return 0
}

func (m Model) filteredGroups() []aws.LogGroup {
	if m.searchQuery == "" {
		return m.logGroups
	}
	q := strings.ToLower(m.searchQuery)
	var result []aws.LogGroup
	for _, g := range m.logGroups {
		if strings.Contains(strings.ToLower(g.Name), q) {
			result = append(result, g)
		}
	}
	return result
}

func (m Model) sortedStreams(streams []aws.LogStream) []aws.LogStream {
	sorted := make([]aws.LogStream, len(streams))
	copy(sorted, streams)
	if m.sortByName {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Name < sorted[j].Name
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].LastEventTimestamp.After(sorted[j].LastEventTimestamp)
		})
	}
	return sorted
}

func (m Model) navigateForward() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case viewGroups:
		groups := m.filteredGroups()
		if len(groups) == 0 || m.cursor >= len(groups) {
			return m, nil
		}
		m.selectedGroup = groups[m.cursor].Name
		// Find the index in the unfiltered list for the inactive group column
		m.groupCursor = 0
		for i, g := range m.logGroups {
			if g.Name == m.selectedGroup {
				m.groupCursor = i
				break
			}
		}
		m.currentView = viewStreams
		m.cursor = 0
		m.searchQuery = ""
		m.loading = true
		return m, m.fetchLogStreams(m.selectedGroup)
	case viewStreams:
		streams := m.sortedStreams(m.filteredStreams())
		if len(streams) == 0 || m.cursor >= len(streams) {
			return m, nil
		}
		m.loading = true
		// If multi-select is active, use selected streams; otherwise use cursor
		var streamNames []string
		if len(m.selected) > 0 {
			for name := range m.selected {
				streamNames = append(streamNames, name)
			}
		} else {
			streamNames = []string{streams[m.cursor].Name}
		}
		return m, m.fetchMultiLogEvents(m.selectedGroup, streamNames)
	}
	return m, nil
}

func (m Model) navigateBack() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case viewStreams:
		m.currentView = viewGroups
		m.cursor = m.groupCursor
		m.logStreams = nil
		m.searchQuery = ""
		m.sortByName = false
		m.selected = nil
		return m, nil
	}
	return m, nil
}
