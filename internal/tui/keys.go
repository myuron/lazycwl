package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
		case "t":
			m.mode = modeTimeInput
			m.timeInput = ""
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

func (m Model) handleTimeInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.mode = modeNormal
		m.timeInput = ""
		return m, nil
	case tea.KeyEnter:
		if d, err := parseDuration(m.timeInput); err == nil {
			m.sinceDuration = d
		}
		m.mode = modeNormal
		m.timeInput = ""
		return m, nil
	case tea.KeyBackspace:
		if r := []rune(m.timeInput); len(r) > 0 {
			m.timeInput = string(r[:len(r)-1])
		}
		return m, nil
	case tea.KeyRunes:
		m.timeInput += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

// parseDuration parses durations like "30m", "2h", "7d".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %w", err)
	}
	if num <= 0 {
		return 0, fmt.Errorf("duration must be positive: %d", num)
	}

	switch unit {
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %c", unit)
	}
}
