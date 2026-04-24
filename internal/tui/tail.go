package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	localaws "github.com/myuron/lazycwl/internal/aws"
)

// maxTailEvents is the maximum number of events kept in the tail buffer.
const maxTailEvents = 1000

// enterTailMode starts a live tail session for the selected stream(s).
func (m Model) enterTailMode() (tea.Model, tea.Cmd) {
	streams := m.sortedStreams(m.filteredStreams())
	if len(streams) == 0 {
		return m, nil
	}

	var streamNames []string
	if len(m.selected) > 0 {
		for name := range m.selected {
			streamNames = append(streamNames, name)
		}
	} else {
		if m.cursor < len(streams) {
			streamNames = []string{streams[m.cursor].Name}
		}
	}

	// Use ARN if available, fall back to group name for local environments.
	groupIdentifier := m.selectedGroupARN()
	if groupIdentifier == "" {
		groupIdentifier = m.selectedGroup
	}

	m.currentView = viewTail
	m.tailStreams = streamNames
	m.tailEvents = nil
	m.tailPaused = false
	m.tailScrollOffset = 0
	m.err = nil

	tailCtx, tailCancel := context.WithCancel(m.ctx)
	m.tailCancel = tailCancel

	eventsCh := make(chan localaws.LogEvent, 100)
	m.tailEventsCh = eventsCh

	return m, m.startTailStream(tailCtx, groupIdentifier, streamNames, eventsCh)
}

// startTailStream starts the live tail API call and pumps events into the channel.
func (m Model) startTailStream(ctx context.Context, groupARN string, streamNames []string, eventsCh chan<- localaws.LogEvent) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		out, err := client.StartLiveTailSession(ctx, groupARN, streamNames)
		if err != nil {
			close(eventsCh)
			return tailErrMsg{err}
		}

		stream := out.GetStream()

		// Pump events from the AWS EventStream into our channel in a goroutine.
		go func() {
			defer close(eventsCh)
			defer func() { _ = stream.Close() }()

			for event := range stream.Events() {
				switch v := event.(type) {
				case *types.StartLiveTailResponseStreamMemberSessionUpdate:
					for _, e := range v.Value.SessionResults {
						var ts time.Time
						if e.Timestamp != nil {
							ts = time.UnixMilli(*e.Timestamp)
						}
						le := localaws.LogEvent{
							Timestamp: ts,
							Message:   aws.ToString(e.Message),
						}
						select {
						case eventsCh <- le:
						case <-ctx.Done():
							return
						}
					}
				case *types.StartLiveTailResponseStreamMemberSessionStart:
					// Session started, no action needed
				}
			}
		}()

		return tailStartedMsg{}
	}
}

// waitForTailEvent returns a tea.Cmd that waits for the next event from the channel.
func (m Model) waitForTailEvent() tea.Cmd {
	ch := m.tailEventsCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return tailErrMsg{fmt.Errorf("tail stream closed")}
		}
		// Drain any buffered events to batch them
		events := []localaws.LogEvent{event}
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					return tailEventMsg{events: events}
				}
				events = append(events, e)
				if len(events) >= 500 {
					return tailEventMsg{events: events}
				}
			default:
				return tailEventMsg{events: events}
			}
		}
	}
}

// appendTailEvents adds events to the tail buffer and trims if necessary.
// Returns the number of events trimmed from the front.
func (m *Model) appendTailEvents(events []localaws.LogEvent) int {
	m.tailEvents = append(m.tailEvents, events...)
	trimmed := 0
	if len(m.tailEvents) > maxTailEvents {
		trimmed = len(m.tailEvents) - maxTailEvents
		m.tailEvents = m.tailEvents[trimmed:]
	}
	return trimmed
}

// exitTailMode stops the live tail and returns to streams view.
func (m Model) exitTailMode() (tea.Model, tea.Cmd) {
	if m.tailCancel != nil {
		m.tailCancel()
		m.tailCancel = nil
	}
	m.currentView = viewStreams
	m.tailEvents = nil
	m.tailStreams = nil
	m.tailEventsCh = nil
	m.tailPaused = false
	m.tailScrollOffset = 0
	return m, nil
}

// handleTailKey handles key events in tail mode.
func (m Model) handleTailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		return m.exitTailMode()
	case tea.KeyDown:
		m.tailScrollDown()
		return m, nil
	case tea.KeyUp:
		m.tailScrollUp()
		return m, nil
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "q":
			return m.exitTailMode()
		case "j":
			m.tailScrollDown()
			return m, nil
		case "k":
			m.tailScrollUp()
			return m, nil
		case "g":
			m.tailScrollToTop()
			return m, nil
		case "G":
			m.tailScrollOffset = 0
			return m, nil
		case "p":
			m.tailPaused = !m.tailPaused
			if !m.tailPaused {
				// Resume: jump to bottom
				m.tailScrollOffset = 0
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) tailScrollUp() {
	maxOffset := len(m.tailEvents) - m.tailVisibleLines()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.tailScrollOffset < maxOffset {
		m.tailScrollOffset++
	}
}

func (m *Model) tailScrollDown() {
	if m.tailScrollOffset > 0 {
		m.tailScrollOffset--
	}
}

func (m *Model) tailScrollToTop() {
	maxOffset := len(m.tailEvents) - m.tailVisibleLines()
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.tailScrollOffset = maxOffset
}

// tailVisibleLines returns the number of log lines visible in the tail view.
func (m Model) tailVisibleLines() int {
	// header(1) + separator(1) + footer(1) = 3 lines overhead
	visible := m.height - 3
	if visible < 1 {
		return 1
	}
	return visible
}

// renderTailView renders the full-screen tail view.
func (m Model) renderTailView() string {
	var b strings.Builder

	// Header
	streamList := strings.Join(m.tailStreams, ", ")
	header := fmt.Sprintf(" Tail: %s (%s)", m.selectedGroup, streamList)
	b.WriteString(header)
	b.WriteString("\n")

	// Separator
	sep := strings.Repeat("─", m.width)
	b.WriteString(sep)
	b.WriteString("\n")

	// Available lines for events
	availableLines := m.tailVisibleLines()

	// Calculate which events to show based on scroll offset
	totalEvents := len(m.tailEvents)
	endIdx := totalEvents - m.tailScrollOffset
	if endIdx < 0 {
		endIdx = 0
	}
	startIdx := endIdx - availableLines
	if startIdx < 0 {
		startIdx = 0
	}

	linesWritten := 0
	for i := startIdx; i < endIdx; i++ {
		e := m.tailEvents[i]
		fmt.Fprintf(&b, " [%s] %s\n",
			e.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z"),
			e.Message)
		linesWritten++
	}

	// Pad remaining lines
	for linesWritten < availableLines {
		b.WriteString("\n")
		linesWritten++
	}

	// Footer
	status := "Live"
	hint := "p: pause  q: exit"
	if m.tailPaused {
		status = "Paused"
		hint = "j/k: scroll  p: resume  q: exit"
	}
	fmt.Fprintf(&b, " %s | %d events | %s", status, totalEvents, hint)

	return b.String()
}
