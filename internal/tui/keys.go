package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	listLen := m.currentListLen()

	switch msg.Type {
	case tea.KeyDown:
		if m.cursor < listLen-1 {
			m.cursor++
		}
		m.adjustOffset()
		return m, m.maybeFetchMore()
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		m.adjustOffset()
		return m, nil
	case tea.KeyEnter, tea.KeyRight:
		return m.navigateForward()
	case tea.KeyBackspace, tea.KeyLeft:
		return m.navigateBack()
	case tea.KeySpace:
		if m.currentView == viewStreams {
			streams := m.sortedStreams(m.filteredStreams())
			if m.cursor < len(streams) {
				name := streams[m.cursor].Name
				if m.selected == nil {
					m.selected = make(map[string]bool)
				}
				if m.selected[name] {
					delete(m.selected, name)
				} else {
					m.selected[name] = true
				}
			}
		}
		return m, nil
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "j":
			if m.cursor < listLen-1 {
				m.cursor++
			}
			m.adjustOffset()
			return m, m.maybeFetchMore()
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
			m.adjustOffset()
			return m, nil
		case "g":
			m.cursor = 0
			m.offset = 0
			return m, nil
		case "G":
			if listLen > 0 {
				m.cursor = listLen - 1
			}
			m.adjustOffset()
			return m, m.maybeFetchMore()
		case "l":
			return m.navigateForward()
		case "h":
			return m.navigateBack()
		case "/":
			m.mode = modeSearch
			m.searchQuery = ""
			m.cursor = 0
			m.offset = 0
			return m, nil
		case "s":
			if m.currentView == viewStreams {
				m.sortByName = !m.sortByName
				m.cursor = 0
				m.offset = 0
			}
			return m, nil
		case "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.mode = modeNormal
		m.searchQuery = ""
		m.cursor = 0
		m.offset = 0
		return m, nil
	case tea.KeyEnter:
		m.mode = modeNormal
		return m, nil
	case tea.KeyBackspace:
		if r := []rune(m.searchQuery); len(r) > 0 {
			m.searchQuery = string(r[:len(r)-1])
			m.cursor = 0
			m.offset = 0
		}
		return m, nil
	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		m.cursor = 0
		m.offset = 0
		return m, nil
	}
	return m, nil
}

