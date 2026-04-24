package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myuron/lazycwl/internal/aws"
)

func makeTailModel() Model {
	m := NewModel(nil)
	m.currentView = viewTail
	m.selectedGroup = "/aws/lambda/func-a"
	m.tailStreams = []string{"stream-001"}
	m.tailPaused = false
	m.tailScrollOffset = 0
	m.width = 80
	m.height = 24
	return m
}

func makeTailEvents(n int) []aws.LogEvent {
	events := make([]aws.LogEvent, n)
	base := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	for i := range events {
		events[i] = aws.LogEvent{
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Message:   "log message " + string(rune('A'+i%26)),
		}
	}
	return events
}

// --- Enter tail mode ---

func TestModel_EnterTailMode_RequiresStreams(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logStreams = []aws.LogStream{} // empty

	m, _ = update(m, keyMsg('f'))
	if m.currentView == viewTail {
		t.Error("should not enter tail mode with no streams")
	}
}

func TestModel_EnterTailMode_FallsBackToGroupName(t *testing.T) {
	// When ARN is not available, enterTailMode should use the group name as identifier.
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logGroups = []aws.LogGroup{{Name: "/aws/test"}} // no ARN
	m.logStreams = []aws.LogStream{{Name: "stream-001"}}

	m, cmd := update(m, keyMsg('f'))
	if m.currentView != viewTail {
		t.Error("should enter tail mode even without ARN (falls back to group name)")
	}
	if cmd == nil {
		t.Error("expected command to start tail stream")
	}
}

func TestModel_EnterTailMode_MultiSelect_SortedStreamNames(t *testing.T) {
	// Iterating a map has randomized order in Go; the multi-select collection
	// must be sorted so the header and AWS request are deterministic across
	// runs with the same selection.
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logGroups = []aws.LogGroup{{Name: "/aws/test"}}
	m.logStreams = []aws.LogStream{
		{Name: "stream-a"}, {Name: "stream-b"}, {Name: "stream-c"},
	}
	m.selected = map[string]bool{
		"stream-c": true, "stream-a": true, "stream-b": true,
	}

	m, _ = update(m, keyMsg('f'))
	if m.currentView != viewTail {
		t.Fatal("expected viewTail")
	}
	expected := []string{"stream-a", "stream-b", "stream-c"}
	if len(m.tailStreams) != len(expected) {
		t.Fatalf("expected %d streams, got %d", len(expected), len(m.tailStreams))
	}
	for i, want := range expected {
		if m.tailStreams[i] != want {
			t.Errorf("tailStreams[%d]: expected %q, got %q", i, want, m.tailStreams[i])
		}
	}
}

func TestModel_FKeyOnlyWorksInStreamsView(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{{Name: "g1"}}
	m.cursor = 0

	m, _ = update(m, keyMsg('f'))
	if m.currentView == viewTail {
		t.Error("f should not enter tail mode in groups view")
	}
}

// --- Tail event handling ---

func TestModel_TailEventMsg_AppendsEvents(t *testing.T) {
	m := makeTailModel()

	events := makeTailEvents(3)
	m, _ = update(m, tailEventMsg{events: events})

	if len(m.tailEvents) != 3 {
		t.Errorf("expected 3 tail events, got %d", len(m.tailEvents))
	}
}

func TestModel_TailEventMsg_TrimsAtMax(t *testing.T) {
	m := makeTailModel()

	// Fill with maxTailEvents + 10
	events := makeTailEvents(maxTailEvents + 10)
	m, _ = update(m, tailEventMsg{events: events})

	if len(m.tailEvents) != maxTailEvents {
		t.Errorf("expected %d tail events after trim, got %d", maxTailEvents, len(m.tailEvents))
	}
}

func TestModel_TailEventMsg_ScrolledUp_PinsToOriginalEvents(t *testing.T) {
	// When the user has scrolled up to read old events, appending new events
	// at the bottom should keep the same absolute events visible by bumping
	// scrollOffset by the number of newly added events (not by trimmed count).
	m := makeTailModel()
	m.tailEvents = makeTailEvents(maxTailEvents)
	m.tailScrollOffset = 5

	// Add 10 more events, which will trim 10 from front.
	// Net growth is 0, but added is 10. The visible window (endIdx = total-offset)
	// needs to move back by 10 in absolute index space to show the same events.
	m, _ = update(m, tailEventMsg{events: makeTailEvents(10)})

	if m.tailScrollOffset != 15 {
		t.Errorf("expected scrollOffset 15 (5 + 10 added), got %d", m.tailScrollOffset)
	}
}

func TestModel_TailEventMsg_ScrolledUp_NoTrim_PinsPosition(t *testing.T) {
	// Scrolled-up, buffer not full yet — adding events must not slide the view.
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)
	m.tailScrollOffset = 5

	m, _ = update(m, tailEventMsg{events: makeTailEvents(3)})

	if m.tailScrollOffset != 8 {
		t.Errorf("expected scrollOffset 8 (5 + 3 added), got %d", m.tailScrollOffset)
	}
}

func TestModel_TailEventMsg_PausedAtBottom_StaysInPlace(t *testing.T) {
	// Paused at the bottom: new events should be buffered but the view
	// must not auto-scroll. scrollOffset should bump so endIdx stays pinned.
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)
	m.tailPaused = true
	m.tailScrollOffset = 0

	m, _ = update(m, tailEventMsg{events: makeTailEvents(5)})

	if m.tailScrollOffset != 5 {
		t.Errorf("expected scrollOffset 5 (paused, 5 added), got %d", m.tailScrollOffset)
	}
}

func TestModel_TailEventMsg_LiveAtBottom_AutoScrolls(t *testing.T) {
	// Live mode (not paused) at the bottom: auto-scroll should keep offset at 0.
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)
	m.tailPaused = false
	m.tailScrollOffset = 0

	m, _ = update(m, tailEventMsg{events: makeTailEvents(5)})

	if m.tailScrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 (live auto-scroll), got %d", m.tailScrollOffset)
	}
}

func TestModel_TailEventMsg_ScrollOffsetClampedToMax(t *testing.T) {
	// If bumping scrollOffset would exceed maxOffset (because so many events
	// were trimmed that the old position is gone), clamp to maxOffset.
	m := makeTailModel()
	m.tailEvents = makeTailEvents(maxTailEvents)
	// User is near the top (high offset). visibleLines = 24-3 = 21, so max = 979.
	m.tailScrollOffset = 975

	// Add 100 events, trimming 100 from front. Old top events are gone.
	m, _ = update(m, tailEventMsg{events: makeTailEvents(100)})

	maxOffset := len(m.tailEvents) - m.tailVisibleLines()
	if m.tailScrollOffset != maxOffset {
		t.Errorf("expected scrollOffset clamped to maxOffset %d, got %d", maxOffset, m.tailScrollOffset)
	}
}

func TestModel_TailEventMsg_IgnoredOutsideTailView(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewGroups

	m, _ = update(m, tailEventMsg{events: makeTailEvents(3)})
	if len(m.tailEvents) != 0 {
		t.Error("tailEventMsg should be ignored when not in viewTail")
	}
}

// --- Tail key handling ---

func TestModel_TailMode_QExits(t *testing.T) {
	m := makeTailModel()

	m, _ = update(m, keyMsg('q'))
	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams after q in tail mode, got %d", m.currentView)
	}
}

func TestModel_TailMode_EscapeExits(t *testing.T) {
	m := makeTailModel()

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEscape})
	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams after escape in tail mode, got %d", m.currentView)
	}
}

func TestModel_TailMode_PTogglesPause(t *testing.T) {
	m := makeTailModel()

	m, _ = update(m, keyMsg('p'))
	if !m.tailPaused {
		t.Error("expected tailPaused=true after first p")
	}

	m, _ = update(m, keyMsg('p'))
	if m.tailPaused {
		t.Error("expected tailPaused=false after second p")
	}
	if m.tailScrollOffset != 0 {
		t.Error("expected scrollOffset reset to 0 on resume")
	}
}

func TestModel_TailMode_KScrollsUp(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)

	m, _ = update(m, keyMsg('k'))
	if m.tailScrollOffset != 1 {
		t.Errorf("expected scrollOffset 1 after k, got %d", m.tailScrollOffset)
	}
}

func TestModel_TailMode_JScrollsDown(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)
	m.tailScrollOffset = 5

	m, _ = update(m, keyMsg('j'))
	if m.tailScrollOffset != 4 {
		t.Errorf("expected scrollOffset 4 after j, got %d", m.tailScrollOffset)
	}
}

func TestModel_TailMode_JDoesNotGoBelowZero(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(10)
	m.tailScrollOffset = 0

	m, _ = update(m, keyMsg('j'))
	if m.tailScrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 (floor), got %d", m.tailScrollOffset)
	}
}

func TestModel_TailMode_GJumpsToTop(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)
	m.tailScrollOffset = 0

	m, _ = update(m, keyMsg('g'))
	// Should be at max offset
	expected := len(m.tailEvents) - m.tailVisibleLines()
	if expected < 0 {
		expected = 0
	}
	if m.tailScrollOffset != expected {
		t.Errorf("expected scrollOffset %d after g, got %d", expected, m.tailScrollOffset)
	}
}

func TestModel_TailMode_GCapitalJumpsToBottom(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)
	m.tailScrollOffset = 20

	m, _ = update(m, keyMsg('G'))
	if m.tailScrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after G, got %d", m.tailScrollOffset)
	}
}

func TestModel_TailMode_GCapital_ResumesFromPaused(t *testing.T) {
	// F16 spec: "G 押下で最下部に戻り自動スクロールを再開".
	// Auto-scroll is a live-mode behaviour, so G must also clear paused state.
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)
	m.tailPaused = true
	m.tailScrollOffset = 20

	m, _ = update(m, keyMsg('G'))
	if m.tailScrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after G, got %d", m.tailScrollOffset)
	}
	if m.tailPaused {
		t.Error("expected tailPaused=false after G (auto-scroll resumes)")
	}
}

func TestModel_TailMode_ArrowKeys(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(50)

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.tailScrollOffset != 1 {
		t.Errorf("expected scrollOffset 1 after up arrow, got %d", m.tailScrollOffset)
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.tailScrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after down arrow, got %d", m.tailScrollOffset)
	}
}

// --- Exit tail mode ---

func TestModel_ExitTailMode_ClearsState(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(10)

	updated, _ := m.exitTailMode()
	m = updated.(Model)

	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams after exit, got %d", m.currentView)
	}
	if m.tailEvents != nil {
		t.Error("expected tailEvents to be nil after exit")
	}
	if m.tailStreams != nil {
		t.Error("expected tailStreams to be nil after exit")
	}
}

// --- Render tail view ---

func TestModel_RenderTailView_ShowsHeader(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(3)

	view := m.renderTailView()
	if !strings.Contains(view, "Tail:") {
		t.Error("expected 'Tail:' in header")
	}
	if !strings.Contains(view, "/aws/lambda/func-a") {
		t.Error("expected log group name in header")
	}
	if !strings.Contains(view, "stream-001") {
		t.Error("expected stream name in header")
	}
}

func TestModel_RenderTailView_ShowsEvents(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = []aws.LogEvent{
		{Timestamp: time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC), Message: "START RequestId"},
	}

	view := m.renderTailView()
	if !strings.Contains(view, "2024-01-15T09:30:00.000Z") {
		t.Error("expected ISO8601 timestamp in view")
	}
	if !strings.Contains(view, "START RequestId") {
		t.Error("expected log message in view")
	}
}

func TestModel_RenderTailView_ShowsLiveStatus(t *testing.T) {
	m := makeTailModel()
	m.tailPaused = false

	view := m.renderTailView()
	if !strings.Contains(view, "Live") {
		t.Error("expected 'Live' in status bar")
	}
	if !strings.Contains(view, "p: pause") {
		t.Error("expected 'p: pause' hint")
	}
}

func TestModel_RenderTailView_ShowsPausedStatus(t *testing.T) {
	m := makeTailModel()
	m.tailPaused = true

	view := m.renderTailView()
	if !strings.Contains(view, "Paused") {
		t.Error("expected 'Paused' in status bar")
	}
	if !strings.Contains(view, "p: resume") {
		t.Error("expected 'p: resume' hint")
	}
}

func TestModel_RenderTailView_ShowsEventCount(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(42)

	view := m.renderTailView()
	if !strings.Contains(view, "42 events") {
		t.Error("expected '42 events' in status bar")
	}
}

// --- appendTailEvents ---

func TestAppendTailEvents_Basic(t *testing.T) {
	m := makeTailModel()
	trimmed := m.appendTailEvents(makeTailEvents(5))
	if trimmed != 0 {
		t.Errorf("expected 0 trimmed, got %d", trimmed)
	}
	if len(m.tailEvents) != 5 {
		t.Errorf("expected 5 events, got %d", len(m.tailEvents))
	}
}

func TestAppendTailEvents_Trims(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(maxTailEvents - 5)
	trimmed := m.appendTailEvents(makeTailEvents(10))
	if trimmed != 5 {
		t.Errorf("expected 5 trimmed, got %d", trimmed)
	}
	if len(m.tailEvents) != maxTailEvents {
		t.Errorf("expected %d events, got %d", maxTailEvents, len(m.tailEvents))
	}
}

func TestModel_TailErrMsg_IgnoredAfterExit(t *testing.T) {
	// After the user exits tail mode (q/Esc), the pump goroutine closes the
	// events channel, which makes waitForTailEvent return a synthetic
	// "tail stream closed" tailErrMsg. This must not surface as an error
	// because the user's exit was intentional.
	m := makeTailModel()
	m.logGroups = []aws.LogGroup{{Name: "/aws/lambda/func-a"}}
	m.logStreams = []aws.LogStream{{Name: "stream-001"}}

	// Simulate user exit: currentView is no longer viewTail.
	updated, _ := m.exitTailMode()
	m = updated.(Model)
	if m.currentView != viewStreams {
		t.Fatalf("setup: expected viewStreams after exit, got %d", m.currentView)
	}

	// The delayed tailErrMsg arrives after exit.
	m, _ = update(m, tailErrMsg{err: fmt.Errorf("tail stream closed")})

	if m.err != nil {
		t.Errorf("expected err nil (stale msg ignored), got %v", m.err)
	}
}

func TestModel_TailErrMsg_ReturnsToStreamsWithError(t *testing.T) {
	m := makeTailModel()
	m.logGroups = []aws.LogGroup{{Name: "/aws/lambda/func-a"}}

	m, _ = update(m, tailErrMsg{err: fmt.Errorf("UnsupportedOperation")})

	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams after tailErrMsg, got %d", m.currentView)
	}
	if m.err == nil {
		t.Error("expected error to be preserved after tailErrMsg")
	}

	// Error should be shown in status bar, not as full-screen error
	view := m.View()
	if !strings.Contains(view, "UnsupportedOperation") {
		t.Error("expected error message in status bar")
	}
	if !strings.Contains(view, "Log Groups") || !strings.Contains(view, "Streams") {
		t.Error("expected normal TUI layout, not full-screen error")
	}
}

func TestModel_TailErrMsg_ErrorClearsOnKeyPress(t *testing.T) {
	m := makeTailModel()
	m.logGroups = []aws.LogGroup{{Name: "/aws/lambda/func-a"}}
	m.logStreams = []aws.LogStream{{Name: "stream-001"}}

	// Simulate tail error
	m, _ = update(m, tailErrMsg{err: fmt.Errorf("test error")})
	if m.err == nil {
		t.Fatal("expected error to be set")
	}

	// Any key press should clear the error
	m, _ = update(m, keyMsg('j'))
	if m.err != nil {
		t.Error("expected error to be cleared after key press")
	}
}

func TestModel_WaitForTailEvent_SurfacesStreamErr(t *testing.T) {
	// When the AWS EventStream surfaces a real error via stream.Err()
	// (SessionTimeoutException, ExpiredTokenException, etc.), it should be
	// delivered through tailErrCh and reported via tailErrMsg — not swallowed
	// into the generic "tail stream closed" message.
	m := makeTailModel()
	eventsCh := make(chan aws.LogEvent)
	errCh := make(chan error, 1)
	m.tailEventsCh = eventsCh
	m.tailErrCh = errCh

	errCh <- fmt.Errorf("SessionTimeoutException")
	close(eventsCh)

	cmd := m.waitForTailEvent()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	msg := cmd()
	errMsg, ok := msg.(tailErrMsg)
	if !ok {
		t.Fatalf("expected tailErrMsg, got %T", msg)
	}
	if !strings.Contains(errMsg.err.Error(), "SessionTimeoutException") {
		t.Errorf("expected SessionTimeoutException in error, got %v", errMsg.err)
	}
}

func TestModel_WaitForTailEvent_FallbackToStreamClosed(t *testing.T) {
	// When eventsCh closes with no error on errCh, fall back to the generic
	// "tail stream closed" message (clean shutdown path).
	m := makeTailModel()
	eventsCh := make(chan aws.LogEvent)
	errCh := make(chan error, 1)
	m.tailEventsCh = eventsCh
	m.tailErrCh = errCh

	close(eventsCh)

	cmd := m.waitForTailEvent()
	msg := cmd()
	errMsg, ok := msg.(tailErrMsg)
	if !ok {
		t.Fatalf("expected tailErrMsg, got %T", msg)
	}
	if !strings.Contains(errMsg.err.Error(), "closed") {
		t.Errorf("expected 'closed' in error, got %v", errMsg.err)
	}
}

func TestModel_ViewDispatcher_TailMode(t *testing.T) {
	m := makeTailModel()
	m.tailEvents = makeTailEvents(3)

	view := m.View()
	if !strings.Contains(view, "Tail:") {
		t.Error("expected View() to render tail view when in viewTail")
	}
}
