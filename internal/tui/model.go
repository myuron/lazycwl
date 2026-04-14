package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	ctx         context.Context
	cancel      context.CancelFunc
	client      *aws.Client
	currentView viewState
	mode        inputMode
	cursor      int
	groupCursor int
	offset      int // scroll offset for current active list
	groupOffset int // scroll offset for group list (preserved when viewing streams)

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
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		ctx:           ctx,
		cancel:        cancel,
		client:        client,
		currentView:   viewGroups,
		sinceDuration: time.Hour,
	}
}

// NewModelWithOptions creates a new TUI model with the given options.
func NewModelWithOptions(client *aws.Client, opts Options) Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := Model{
		ctx:           ctx,
		cancel:        cancel,
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
		m.offset = 0
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

func (m Model) fetchLogGroups() tea.Cmd {
	return func() tea.Msg {
		groups, err := m.client.ListLogGroups(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		return logGroupsMsg(groups)
	}
}

func (m Model) fetchLogStreams(groupName string) tea.Cmd {
	return func() tea.Msg {
		streams, err := m.client.ListLogStreams(m.ctx, groupName)
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
		events, err := m.client.GetLogEvents(m.ctx, groupName, streamName, now.Add(-since), now)
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
		events, err := m.client.GetMultiStreamLogEvents(m.ctx, groupName, streamNames, now.Add(-since), now)
		if err != nil {
			return errMsg{err}
		}
		return logEventsMsg(events)
	}
}

// visibleItems returns the number of list items visible in a pane.
// It accounts for the header line and status bar.
func (m Model) visibleItems() int {
	contentHeight := m.height - 4
	if m.mode != modeNormal {
		contentHeight--
	}
	// Subtract 1 for the header line ("Log Groups\n" / "Streams — ...\n")
	visible := contentHeight - 1
	if visible < 1 {
		return 1
	}
	return visible
}

// adjustOffset ensures the cursor is within the visible viewport.
func (m *Model) adjustOffset() {
	visible := m.visibleItems()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
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
