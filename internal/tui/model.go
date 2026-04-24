package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myuron/lazycwl/internal/aws"
	"github.com/myuron/lazycwl/internal/editor"
	"github.com/myuron/lazycwl/internal/formatter"
)

type viewState int

const (
	viewGroups viewState = iota
	viewStreams
	viewTail
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
)

// logGroupsPageMsg is sent when the first page of log groups is fetched.
type logGroupsPageMsg struct {
	groups    []aws.LogGroup
	nextToken *string
}

// logStreamsPageMsg is sent when the first page of log streams is fetched.
type logStreamsPageMsg struct {
	streams   []aws.LogStream
	nextToken *string
}

// moreGroupsPageMsg is sent when additional pages of log groups are fetched.
type moreGroupsPageMsg struct {
	groups    []aws.LogGroup
	nextToken *string
}

// moreStreamsPageMsg is sent when additional pages of log streams are fetched.
type moreStreamsPageMsg struct {
	streams   []aws.LogStream
	nextToken *string
}

// logEventsMsg is sent when log events are fetched.
type logEventsMsg []aws.LogEvent

// editorFinishedMsg is sent when the editor process exits.
type editorFinishedMsg struct{ err error }

// tailEventMsg delivers new events from a live tail stream.
type tailEventMsg struct {
	events []aws.LogEvent
}

// tailErrMsg is sent when an error occurs in the live tail stream.
type tailErrMsg struct{ err error }

// tailStartedMsg is sent when the live tail session starts.
type tailStartedMsg struct{}

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

	searchQuery    string
	sortDescending bool
	selected       map[string]bool // multi-select stream names

	groupsNextToken  *string
	streamsNextToken *string

	loading     bool
	loadingMore bool
	err         error

	// Tail mode state
	tailEvents       []aws.LogEvent
	tailStreams      []string
	tailCancel       context.CancelFunc
	tailPaused       bool
	tailScrollOffset int // offset from the bottom (0 = at bottom, auto-scroll)
	tailEventsCh     <-chan aws.LogEvent
	tailErrCh        <-chan error // set when the live tail goroutine surfaces stream.Err()

	width  int
	height int
}

// Options configures the initial state of the TUI model.
type Options struct {
	InitialGroup string
}

// NewModel creates a new TUI model.
func NewModel(client *aws.Client) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		ctx:            ctx,
		cancel:         cancel,
		client:         client,
		currentView:    viewGroups,
		sortDescending: true,
	}
}

// NewModelWithOptions creates a new TUI model with the given options.
func NewModelWithOptions(client *aws.Client, opts Options) Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := Model{
		ctx:            ctx,
		cancel:         cancel,
		client:         client,
		currentView:    viewGroups,
		sortDescending: true,
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

	case logGroupsPageMsg:
		m.logGroups = msg.groups
		m.groupsNextToken = msg.nextToken
		m.loading = false
		return m, nil

	case logStreamsPageMsg:
		m.logStreams = msg.streams
		m.streamsNextToken = msg.nextToken
		m.loading = false
		m.cursor = 0
		m.offset = 0
		return m, nil

	case moreGroupsPageMsg:
		m.logGroups = append(m.logGroups, msg.groups...)
		m.groupsNextToken = msg.nextToken
		m.loadingMore = false
		return m, nil

	case moreStreamsPageMsg:
		m.logStreams = append(m.logStreams, msg.streams...)
		m.streamsNextToken = msg.nextToken
		m.loadingMore = false
		return m, nil

	case logEventsMsg:
		m.loading = false
		return m, m.openEditor([]aws.LogEvent(msg))

	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case tailEventMsg:
		if m.currentView != viewTail {
			return m, nil
		}
		added := len(msg.events)
		m.appendTailEvents(msg.events)
		// Pin the visible window when the user is either paused or scrolled
		// up to read older events. endIdx = totalEvents - tailScrollOffset,
		// so to keep endIdx pointing at the same absolute event we bump
		// scrollOffset by the number of newly added events (independent of
		// how many were trimmed from the front).
		if m.tailPaused || m.tailScrollOffset > 0 {
			m.tailScrollOffset += added
			maxOffset := len(m.tailEvents) - m.tailVisibleLines()
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.tailScrollOffset > maxOffset {
				m.tailScrollOffset = maxOffset
			}
		}
		return m, m.waitForTailEvent()

	case tailErrMsg:
		// A closed events channel after a user-initiated exit can deliver a
		// stale tailErrMsg; ignore anything that arrives once tail mode is
		// already gone.
		if m.currentView != viewTail {
			return m, nil
		}
		m.err = msg.err
		return m.exitTailMode()

	case tailStartedMsg:
		return m, m.waitForTailEvent()

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeSearch:
			return m.handleSearchKey(msg)
		default:
			return m.handleKey(msg)
		}
	}

	return m, nil
}

func (m Model) fetchLogGroups() tea.Cmd {
	return func() tea.Msg {
		groups, nextToken, err := m.client.ListLogGroupsPage(m.ctx, nil)
		if err != nil {
			return errMsg{err}
		}
		return logGroupsPageMsg{groups: groups, nextToken: nextToken}
	}
}

func (m Model) fetchMoreGroups() tea.Cmd {
	token := m.groupsNextToken
	return func() tea.Msg {
		groups, nextToken, err := m.client.ListLogGroupsPage(m.ctx, token)
		if err != nil {
			return errMsg{err}
		}
		return moreGroupsPageMsg{groups: groups, nextToken: nextToken}
	}
}

func (m Model) fetchLogStreams(groupName string) tea.Cmd {
	descending := m.sortDescending
	return func() tea.Msg {
		streams, nextToken, err := m.client.ListLogStreamsPage(m.ctx, groupName, nil, descending)
		if err != nil {
			return errMsg{err}
		}
		return logStreamsPageMsg{streams: streams, nextToken: nextToken}
	}
}

func (m Model) fetchMoreStreams() tea.Cmd {
	token := m.streamsNextToken
	groupName := m.selectedGroup
	descending := m.sortDescending
	return func() tea.Msg {
		streams, nextToken, err := m.client.ListLogStreamsPage(m.ctx, groupName, token, descending)
		if err != nil {
			return errMsg{err}
		}
		return moreStreamsPageMsg{streams: streams, nextToken: nextToken}
	}
}

func (m Model) fetchMultiLogEvents(groupName string, streamNames []string) tea.Cmd {
	return func() tea.Msg {
		events, err := m.client.GetMultiStreamLogEvents(m.ctx, groupName, streamNames)
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

// selectedGroupARN returns the ARN of the currently selected log group.
func (m Model) selectedGroupARN() string {
	for _, g := range m.logGroups {
		if g.Name == m.selectedGroup {
			return g.ARN
		}
	}
	return ""
}

func (m Model) hasMoreGroups() bool {
	return m.groupsNextToken != nil
}

func (m Model) hasMoreStreams() bool {
	return m.streamsNextToken != nil
}

// maybeFetchMore returns a command to fetch the next page if the cursor
// is at the last item and more pages are available.
func (m *Model) maybeFetchMore() tea.Cmd {
	if m.loadingMore {
		return nil
	}
	switch m.currentView {
	case viewGroups:
		if m.hasMoreGroups() && m.cursor >= len(m.logGroups)-1 {
			m.loadingMore = true
			return m.fetchMoreGroups()
		}
	case viewStreams:
		if m.hasMoreStreams() && m.cursor >= len(m.logStreams)-1 {
			m.loadingMore = true
			return m.fetchMoreStreams()
		}
	}
	return nil
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
