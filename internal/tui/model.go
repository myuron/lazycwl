package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/myuron/lazycwl/internal/aws"
	"github.com/myuron/lazycwl/internal/editor"
	"github.com/myuron/lazycwl/internal/formatter"
)

type viewState int

const (
	viewGroups  viewState = iota
	viewStreams
)

type inputMode int

const (
	modeNormal    inputMode = iota
	modeSearch
	modeTimeInput
)

// logGroupsMsg is sent when log groups are fetched.
type logGroupsMsg []aws.LogGroup

// logStreamsMsg is sent when log streams are fetched.
type logStreamsMsg []aws.LogStream

// logEventsMsg is sent when log events are fetched.
type logEventsMsg []aws.LogEvent

// editorFinishedMsg is sent when the editor process exits.
type editorFinishedMsg struct{ err error }

// errMsg is sent when an error occurs.
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Model is the root Bubble Tea model.
type Model struct {
	client      *aws.Client
	currentView viewState
	mode        inputMode
	cursor      int
	groupCursor int

	logGroups     []aws.LogGroup
	logStreams    []aws.LogStream
	selectedGroup string
	sinceDuration time.Duration

	searchQuery string
	timeInput   string
	sortByName  bool
	selected    map[string]bool // multi-select stream names

	groupsNextToken  *string
	streamsNextToken *string

	loading bool
	err     error

	width  int
	height int
}

// Options configures the initial state of the TUI model.
type Options struct {
	InitialGroup  string
	SinceDuration time.Duration
}

// NewModel creates a new TUI model.
func NewModel(client *aws.Client) Model {
	return Model{
		client:        client,
		currentView:   viewGroups,
		sinceDuration: time.Hour,
	}
}

// NewModelWithOptions creates a new TUI model with the given options.
func NewModelWithOptions(client *aws.Client, opts Options) Model {
	m := Model{
		client:        client,
		currentView:   viewGroups,
		sinceDuration: opts.SinceDuration,
	}
	if m.sinceDuration == 0 {
		m.sinceDuration = time.Hour
	}
	if opts.InitialGroup != "" {
		m.selectedGroup = opts.InitialGroup
		m.currentView = viewStreams
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.client == nil {
		return nil
	}
	m.loading = true
	if m.currentView == viewStreams && m.selectedGroup != "" {
		return tea.Batch(m.fetchLogGroups(), m.fetchLogStreams(m.selectedGroup))
	}
	return m.fetchLogGroups()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case logGroupsMsg:
		m.logGroups = []aws.LogGroup(msg)
		m.loading = false
		return m, nil

	case logStreamsMsg:
		m.logStreams = []aws.LogStream(msg)
		m.loading = false
		m.cursor = 0
		return m, nil

	case logEventsMsg:
		m.loading = false
		return m, m.openEditor([]aws.LogEvent(msg))

	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeSearch:
			return m.handleSearchKey(msg)
		case modeTimeInput:
			return m.handleTimeInputKey(msg)
		default:
			return m.handleKey(msg)
		}
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	listLen := m.currentListLen()

	switch msg.Type {
	case tea.KeyDown:
		if m.cursor < listLen-1 {
			m.cursor++
		}
		return m, nil
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case tea.KeyEnter, tea.KeyRight:
		return m.navigateForward()
	case tea.KeyBackspace, tea.KeyLeft:
		return m.navigateBack()
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "j":
			if m.cursor < listLen-1 {
				m.cursor++
			}
			return m, nil
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "g":
			m.cursor = 0
			return m, nil
		case "G":
			if listLen > 0 {
				m.cursor = listLen - 1
			}
			return m, nil
		case "l":
			return m.navigateForward()
		case "h":
			return m.navigateBack()
		case "/":
			m.mode = modeSearch
			m.searchQuery = ""
			m.cursor = 0
			return m, nil
		case "t":
			m.mode = modeTimeInput
			m.timeInput = ""
			return m, nil
		case " ":
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
		case "s":
			if m.currentView == viewStreams {
				m.sortByName = !m.sortByName
				m.cursor = 0
			}
			return m, nil
		case "q":
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
		return m, nil
	case tea.KeyEnter:
		m.mode = modeNormal
		return m, nil
	case tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.cursor = 0
		}
		return m, nil
	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		m.cursor = 0
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
		if len(m.timeInput) > 0 {
			m.timeInput = m.timeInput[:len(m.timeInput)-1]
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

func (m Model) filteredStreams() []aws.LogStream {
	if m.searchQuery == "" {
		return m.logStreams
	}
	q := strings.ToLower(m.searchQuery)
	var result []aws.LogStream
	for _, s := range m.logStreams {
		if strings.Contains(strings.ToLower(s.Name), q) {
			result = append(result, s)
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

func (m Model) fetchLogGroups() tea.Cmd {
	return func() tea.Msg {
		groups, err := m.client.ListLogGroups(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return logGroupsMsg(groups)
	}
}

func (m Model) fetchLogStreams(groupName string) tea.Cmd {
	return func() tea.Msg {
		streams, err := m.client.ListLogStreams(context.Background(), groupName)
		if err != nil {
			return errMsg{err}
		}
		return logStreamsMsg(streams)
	}
}

func (m Model) fetchLogEvents(groupName, streamName string) tea.Cmd {
	since := m.sinceDuration
	return func() tea.Msg {
		now := time.Now()
		events, err := m.client.GetLogEvents(context.Background(), groupName, streamName, now.Add(-since), now)
		if err != nil {
			return errMsg{err}
		}
		return logEventsMsg(events)
	}
}

func (m Model) fetchMultiLogEvents(groupName string, streamNames []string) tea.Cmd {
	since := m.sinceDuration
	return func() tea.Msg {
		now := time.Now()
		var allEvents []aws.LogEvent
		for _, sn := range streamNames {
			events, err := m.client.GetLogEvents(context.Background(), groupName, sn, now.Add(-since), now)
			if err != nil {
				return errMsg{err}
			}
			allEvents = append(allEvents, events...)
		}
		return logEventsMsg(allEvents)
	}
}

func (m Model) hasMoreGroups() bool {
	return m.groupsNextToken != nil
}

func (m Model) hasMoreStreams() bool {
	return m.streamsNextToken != nil
}

func (m Model) openEditor(events []aws.LogEvent) tea.Cmd {
	content := formatter.Format(events)
	path, cleanup, err := editor.WriteTempFile(content)
	if err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	cmd := editor.Cmd(path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		cleanup()
		return editorFinishedMsg{err: err}
	})
}

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

	return m.viewThreeColumn()
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
			b.WriteString(fmt.Sprintf("%s%s %s (%s)\n", cursor, mark, s.Name, s.LastEventTimestamp.Format("2006-01-02 15:04:05")))
		}
	}

	b.WriteString(m.renderInputLine())
	b.WriteString("\nq: quit | j/k: move | l: enter | h: back | /: search | t: time | s: sort")

	return b.String()
}

func (m Model) viewThreeColumn() string {
	colWidth := m.width / 3
	contentHeight := m.height - 3
	if m.mode != modeNormal {
		contentHeight--
	}

	borderStyle := lipgloss.NewStyle().
		Width(colWidth - 2).
		Height(contentHeight).
		Padding(0, 1)

	activeStyle := borderStyle.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	inactiveStyle := borderStyle.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	var leftCol, midCol, rightCol string

	switch m.currentView {
	case viewGroups:
		leftCol = ""
		midCol = m.renderGroupList(contentHeight)
		rightCol = m.renderGroupPreview()
	case viewStreams:
		leftCol = m.renderGroupListInactive(contentHeight)
		midCol = m.renderStreamList(contentHeight)
		rightCol = m.renderStreamPreview()
	}

	left := inactiveStyle.Render(leftCol)
	mid := activeStyle.Render(midCol)
	right := inactiveStyle.Render(rightCol)

	main := lipgloss.JoinHorizontal(lipgloss.Top, left, mid, right)

	inputLine := m.renderInputLine()
	statusBar := m.renderStatusBar()

	parts := []string{main}
	if inputLine != "" {
		parts = append(parts, inputLine)
	}
	parts = append(parts, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderGroupList(maxHeight int) string {
	var b strings.Builder
	b.WriteString("Log Groups\n")
	for i, g := range m.filteredGroups() {
		if i >= maxHeight-1 {
			break
		}
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, g.Name))
	}
	return b.String()
}

func (m Model) renderGroupListInactive(maxHeight int) string {
	var b strings.Builder
	b.WriteString("Log Groups\n")
	for i, g := range m.logGroups {
		if i >= maxHeight-1 {
			break
		}
		cursor := " "
		if i == m.groupCursor {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, g.Name))
	}
	return b.String()
}

func (m Model) renderStreamList(maxHeight int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Streams — %s\n", m.selectedGroup))
	for i, s := range m.sortedStreams(m.filteredStreams()) {
		if i >= maxHeight-1 {
			break
		}
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		mark := " "
		if m.selected[s.Name] {
			mark = "*"
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, mark, s.Name))
	}
	return b.String()
}

func (m Model) renderGroupPreview() string {
	groups := m.filteredGroups()
	if len(groups) == 0 || m.cursor >= len(groups) {
		return ""
	}
	g := groups[m.cursor]
	return fmt.Sprintf("Name: %s\nRetention: %dd\nSize: %dB", g.Name, g.RetentionDays, g.StoredBytes)
}

func (m Model) renderStreamPreview() string {
	streams := m.sortedStreams(m.filteredStreams())
	if len(streams) == 0 || m.cursor >= len(streams) {
		return ""
	}
	s := streams[m.cursor]
	return fmt.Sprintf("Stream: %s\nLast event: %s", s.Name, s.LastEventTimestamp.Format("2006-01-02 15:04:05"))
}

func (m Model) renderInputLine() string {
	switch m.mode {
	case modeSearch:
		return fmt.Sprintf("/%s", m.searchQuery)
	case modeTimeInput:
		return fmt.Sprintf("Since: %s", m.timeInput)
	}
	return ""
}

func (m Model) renderStatusBar() string {
	sinceStr := formatDuration(m.sinceDuration)
	sortStr := "time ↓"
	if m.sortByName {
		sortStr = "name ↑"
	}
	return fmt.Sprintf(" Sort: %s | Since: %s | q: quit | /: search | t: time | s: sort", sortStr, sinceStr)
}

func formatDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d / (24 * time.Hour))
		return fmt.Sprintf("%dd", days)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
