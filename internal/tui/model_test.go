package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myuron/lazycwl/internal/aws"
)

// update is a test helper that calls Update and returns the concrete Model.
func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

func keyMsg(r rune) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestModel_InitialState(t *testing.T) {
	m := NewModel(nil)

	if m.currentView != viewGroups {
		t.Errorf("expected initial view to be viewGroups, got %d", m.currentView)
	}
	if m.cursor != 0 {
		t.Errorf("expected initial cursor to be 0, got %d", m.cursor)
	}
}

func TestModel_ReceiveLogGroups(t *testing.T) {
	m := NewModel(nil)

	groups := []aws.LogGroup{
		{Name: "/aws/lambda/func-a", RetentionDays: 30, StoredBytes: 1024},
		{Name: "/aws/ecs/service-b", RetentionDays: 7, StoredBytes: 2048},
	}

	model, _ := update(m, logGroupsPageMsg{groups: groups})

	if len(model.logGroups) != 2 {
		t.Fatalf("expected 2 log groups, got %d", len(model.logGroups))
	}
	if model.logGroups[0].Name != "/aws/lambda/func-a" {
		t.Errorf("expected first group /aws/lambda/func-a, got %s", model.logGroups[0].Name)
	}
	if model.loading {
		t.Error("expected loading to be false after receiving groups")
	}
}

func TestModel_CursorMovement_J(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "group-1"},
		{Name: "group-2"},
		{Name: "group-3"},
	}

	m, _ = update(m, keyMsg('j'))
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.cursor)
	}

	m, _ = update(m, keyMsg('j'))
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 after j, got %d", m.cursor)
	}

	// j at bottom stays at bottom
	m, _ = update(m, keyMsg('j'))
	if m.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.cursor)
	}
}

func TestModel_CursorMovement_K(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "group-1"},
		{Name: "group-2"},
	}
	m.cursor = 1

	m, _ = update(m, keyMsg('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after k, got %d", m.cursor)
	}

	m, _ = update(m, keyMsg('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestModel_CursorMovement_G(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "group-1"},
		{Name: "group-2"},
		{Name: "group-3"},
	}
	m.cursor = 1

	m, _ = update(m, keyMsg('G'))
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 after G, got %d", m.cursor)
	}

	m, _ = update(m, keyMsg('g'))
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after g, got %d", m.cursor)
	}
}

func TestModel_CursorMovement_ArrowKeys(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "group-1"},
		{Name: "group-2"},
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.cursor)
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after up, got %d", m.cursor)
	}
}

func TestModel_NavigateToStreams(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a"},
		{Name: "/aws/ecs/service-b"},
	}
	m.cursor = 0

	m, _ = update(m, keyMsg('l'))
	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams after l, got %d", m.currentView)
	}
	if m.selectedGroup != "/aws/lambda/func-a" {
		t.Errorf("expected selected group /aws/lambda/func-a, got %s", m.selectedGroup)
	}
	if !m.loading {
		t.Error("expected loading to be true while fetching streams")
	}
}

func TestModel_NavigateBackToGroups(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a"},
	}
	m.currentView = viewStreams
	m.selectedGroup = "/aws/lambda/func-a"
	m.cursor = 2
	m.groupCursor = 0

	m, _ = update(m, keyMsg('h'))
	if m.currentView != viewGroups {
		t.Errorf("expected viewGroups after h, got %d", m.currentView)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor restored to 0, got %d", m.cursor)
	}
}

func TestModel_ReceiveLogStreams(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams

	streams := []aws.LogStream{
		{Name: "stream-001", LastEventTimestamp: time.Now()},
		{Name: "stream-002", LastEventTimestamp: time.Now().Add(-time.Minute)},
	}

	m, _ = update(m, logStreamsPageMsg{streams: streams})

	if len(m.logStreams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(m.logStreams))
	}
	if m.loading {
		t.Error("expected loading to be false after receiving streams")
	}
}

func TestModel_Quit(t *testing.T) {
	m := NewModel(nil)

	_, cmd := update(m, keyMsg('q'))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestModel_ViewRendersGroups(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a", RetentionDays: 30, StoredBytes: 1024},
	}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_EmptyListCursorMovement(t *testing.T) {
	m := NewModel(nil)

	m, _ = update(m, keyMsg('j'))
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 on empty list, got %d", m.cursor)
	}
}

func TestModel_NavigateForwardFromStreams_FetchesLogs(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/lambda/func-a"
	m.logStreams = []aws.LogStream{
		{Name: "stream-001", LastEventTimestamp: time.Now()},
	}
	m.cursor = 0

	m, cmd := update(m, keyMsg('l'))
	if !m.loading {
		t.Error("expected loading to be true while fetching log events")
	}
	// client is nil so cmd should still be returned (will error at runtime)
	if cmd == nil {
		t.Error("expected a command to fetch log events")
	}
}

func TestModel_ReceiveLogEvents_OpensEditor(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/lambda/func-a"

	events := []aws.LogEvent{
		{Timestamp: time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC), Message: "test log"},
	}

	m, cmd := update(m, logEventsMsg(events))
	if m.loading {
		t.Error("expected loading to be false after receiving events")
	}
	// Should return an editor command
	if cmd == nil {
		t.Error("expected a command to open editor")
	}
}

func TestModel_EditorFinished_RestoresView(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/lambda/func-a"
	m.logStreams = []aws.LogStream{
		{Name: "stream-001"},
	}
	m.cursor = 0

	m, _ = update(m, editorFinishedMsg{})
	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams after editor finished, got %d", m.currentView)
	}
}

func TestModel_ViewTwoColumns_GroupsView(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 24
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a", RetentionDays: 30, StoredBytes: 1024},
		{Name: "/aws/ecs/service-b", RetentionDays: 7, StoredBytes: 2048},
	}
	m.cursor = 0

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
	if !strings.Contains(view, "Log Groups") {
		t.Error("expected 'Log Groups' header in left pane")
	}
	if !strings.Contains(view, "Streams") {
		t.Error("expected 'Streams' header in right pane")
	}
}

func TestModel_ViewTwoColumns_StreamsView(t *testing.T) {
	ts := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	m := NewModel(nil)
	m.width = 120
	m.height = 24
	m.currentView = viewStreams
	m.selectedGroup = "/aws/lambda/func-a"
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a"},
		{Name: "/aws/ecs/service-b"},
	}
	m.logStreams = []aws.LogStream{
		{Name: "stream-001", LastEventTimestamp: ts},
		{Name: "stream-002", LastEventTimestamp: ts.Add(-time.Hour)},
	}
	m.cursor = 0
	m.groupCursor = 0

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
	// Right pane should show Last Event timestamps
	if !strings.Contains(view, "09:30:00") {
		t.Error("expected Last Event time '09:30:00' in streams pane")
	}
	if !strings.Contains(view, "08:30:00") {
		t.Error("expected Last Event time '08:30:00' in streams pane")
	}
}

func TestModel_ViewTwoColumns_StreamsShowLastEvent(t *testing.T) {
	ts := time.Date(2024, 3, 20, 14, 5, 30, 0, time.UTC)
	m := NewModel(nil)
	m.width = 120
	m.height = 24
	m.currentView = viewStreams
	m.selectedGroup = "/test/group"
	m.logStreams = []aws.LogStream{
		{Name: "my-stream", LastEventTimestamp: ts},
	}
	m.cursor = 0

	view := m.View()
	if !strings.Contains(view, "my-stream") {
		t.Error("expected stream name in view")
	}
	if !strings.Contains(view, "2024-03-20 14:05:30") {
		t.Error("expected Last Event timestamp '2024-03-20 14:05:30' next to stream name")
	}
}


// --- Phase 5: Search / Filter / Sort / Time range ---

func TestModel_SearchMode_Enter(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a"},
		{Name: "/aws/ecs/service-b"},
	}

	// / enters search mode
	m, _ = update(m, keyMsg('/'))
	if m.mode != modeSearch {
		t.Errorf("expected modeSearch, got %d", m.mode)
	}
}

func TestModel_SearchMode_FilterGroups(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a"},
		{Name: "/aws/ecs/service-b"},
		{Name: "/aws/lambda/func-c"},
	}

	// Enter search mode and type "lambda"
	m, _ = update(m, keyMsg('/'))
	for _, r := range "lambda" {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	filtered := m.filteredGroups()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered groups, got %d", len(filtered))
	}
	if filtered[0].Name != "/aws/lambda/func-a" {
		t.Errorf("expected /aws/lambda/func-a, got %s", filtered[0].Name)
	}
}

func TestModel_SearchMode_FilterStreams(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.logStreams = []aws.LogStream{
		{Name: "stream-001"},
		{Name: "stream-002"},
		{Name: "other-stream"},
	}

	m, _ = update(m, keyMsg('/'))
	for _, r := range "stream-0" {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	filtered := m.filteredStreams()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered streams, got %d", len(filtered))
	}
}

func TestModel_SearchMode_EscapeClearsFilter(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a"},
		{Name: "/aws/ecs/service-b"},
	}

	m, _ = update(m, keyMsg('/'))
	for _, r := range "lambda" {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEscape})
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after escape, got %d", m.mode)
	}
	if m.searchQuery != "" {
		t.Errorf("expected empty search query after escape, got %q", m.searchQuery)
	}
}

func TestModel_SearchMode_BackspaceDeletesChar(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{{Name: "test"}}

	m, _ = update(m, keyMsg('/'))
	for _, r := range "abc" {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.searchQuery != "ab" {
		t.Errorf("expected search query 'ab' after backspace, got %q", m.searchQuery)
	}
}

func TestModel_SortedStreams_ByTimeDesc(t *testing.T) {
	now := time.Now()
	m := NewModel(nil)
	m.currentView = viewStreams
	m.logStreams = []aws.LogStream{
		{Name: "old-stream", LastEventTimestamp: now.Add(-time.Hour)},
		{Name: "new-stream", LastEventTimestamp: now},
	}

	sorted := m.sortedStreams(m.logStreams)
	if sorted[0].Name != "new-stream" {
		t.Errorf("expected new-stream first (time desc), got %s", sorted[0].Name)
	}
}

func TestModel_SortedStreams_ByTimeAsc(t *testing.T) {
	now := time.Now()
	m := NewModel(nil)
	m.currentView = viewStreams
	m.sortDescending = false
	m.logStreams = []aws.LogStream{
		{Name: "new-stream", LastEventTimestamp: now},
		{Name: "old-stream", LastEventTimestamp: now.Add(-time.Hour)},
	}

	sorted := m.sortedStreams(m.logStreams)
	if sorted[0].Name != "old-stream" {
		t.Errorf("expected old-stream first (time asc), got %s", sorted[0].Name)
	}
}

func TestModel_SortToggle(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "test-group"
	m.logStreams = []aws.LogStream{{Name: "stream-1"}}

	// Default is descending
	if !m.sortDescending {
		t.Error("expected default sortDescending=true")
	}

	// s toggles to ascending
	m, cmd := update(m, keyMsg('s'))
	if m.sortDescending {
		t.Error("expected sortDescending=false after pressing s")
	}
	if cmd == nil {
		t.Error("expected cmd to re-fetch streams after sort toggle")
	}

	// s toggles back to descending
	m, cmd = update(m, keyMsg('s'))
	if !m.sortDescending {
		t.Error("expected sortDescending=true after pressing s again")
	}
	if cmd == nil {
		t.Error("expected cmd to re-fetch streams after sort toggle")
	}
}

// --- Phase 6: Multi-select, Preview, Pagination ---

func TestModel_MultiSelect_SpaceToggles(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.logStreams = []aws.LogStream{
		{Name: "stream-001"},
		{Name: "stream-002"},
		{Name: "stream-003"},
	}
	m.cursor = 0

	// Space selects current stream (use KeySpace, matching real terminal input)
	m, _ = update(m, tea.KeyMsg{Type: tea.KeySpace})
	if !m.selected["stream-001"] {
		t.Error("expected stream-001 to be selected")
	}

	// Move down and select another
	m, _ = update(m, keyMsg('j'))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeySpace})
	if !m.selected["stream-002"] {
		t.Error("expected stream-002 to be selected")
	}

	// Space again deselects
	m, _ = update(m, tea.KeyMsg{Type: tea.KeySpace})
	if m.selected["stream-002"] {
		t.Error("expected stream-002 to be deselected")
	}
}

func TestModel_MultiSelect_NavigateForward(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/lambda/func-a"
	m.logStreams = []aws.LogStream{
		{Name: "stream-001"},
		{Name: "stream-002"},
		{Name: "stream-003"},
	}
	m.selected = map[string]bool{
		"stream-001": true,
		"stream-003": true,
	}
	m.cursor = 0

	// l with multi-select should fetch events for selected streams
	m, cmd := update(m, keyMsg('l'))
	if !m.loading {
		t.Error("expected loading to be true")
	}
	if cmd == nil {
		t.Error("expected command to be returned")
	}
}

func TestModel_MultiSelect_ClearedOnBack(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.logGroups = []aws.LogGroup{{Name: "g1"}}
	m.logStreams = []aws.LogStream{{Name: "s1"}}
	m.selected = map[string]bool{"s1": true}

	m, _ = update(m, keyMsg('h'))
	if len(m.selected) != 0 {
		t.Error("expected selected to be cleared on navigate back")
	}
}

func TestModel_SpaceOnlyWorksInStreamsView(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{{Name: "g1"}}
	m.cursor = 0

	m, _ = update(m, tea.KeyMsg{Type: tea.KeySpace})
	if m.selected != nil && len(m.selected) > 0 {
		t.Error("space should not select in groups view")
	}
}

func TestModel_Pagination_NextToken(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "group-1"},
	}
	m.groupsNextToken = stringPtr("token-123")

	if !m.hasMoreGroups() {
		t.Error("expected hasMoreGroups to be true when token exists")
	}

	m.groupsNextToken = nil
	if m.hasMoreGroups() {
		t.Error("expected hasMoreGroups to be false when no token")
	}
}

func stringPtr(s string) *string { return &s }

// --- Phase 7: CLI options ---

func TestNewModelWithOptions_InitialGroup(t *testing.T) {
	m := NewModelWithOptions(nil, Options{
		InitialGroup: "/aws/lambda/func-a",
	})

	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams when InitialGroup set, got %d", m.currentView)
	}
	if m.selectedGroup != "/aws/lambda/func-a" {
		t.Errorf("expected selected group /aws/lambda/func-a, got %s", m.selectedGroup)
	}
}

func TestNewModelWithOptions_NoGroup(t *testing.T) {
	m := NewModelWithOptions(nil, Options{})
	if m.currentView != viewGroups {
		t.Errorf("expected viewGroups when no InitialGroup, got %d", m.currentView)
	}
}

func TestModel_SearchThenNavigate_GroupCursorPointsToCorrectGroup(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/func-a"},
		{Name: "/aws/ecs/service-b"},
		{Name: "/aws/lambda/func-c"},
	}

	// Enter search mode and type "ecs" to filter to only service-b
	m, _ = update(m, keyMsg('/'))
	for _, r := range "ecs" {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Confirm search with Enter
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	// cursor=0 points to the first (and only) filtered item: service-b
	// Navigate forward to streams
	m, _ = update(m, keyMsg('l'))

	// groupCursor should point to index 1 in the unfiltered logGroups
	// (the index of /aws/ecs/service-b)
	if m.groupCursor != 1 {
		t.Errorf("expected groupCursor=1 (index of /aws/ecs/service-b in unfiltered list), got %d", m.groupCursor)
	}
}

// --- Scroll / Viewport ---

func makeGroups(n int) []aws.LogGroup {
	groups := make([]aws.LogGroup, n)
	for i := range groups {
		groups[i] = aws.LogGroup{Name: fmt.Sprintf("/aws/lambda/func-%03d", i)}
	}
	return groups
}

func makeStreams(n int) []aws.LogStream {
	streams := make([]aws.LogStream, n)
	for i := range streams {
		streams[i] = aws.LogStream{
			Name:               fmt.Sprintf("stream-%03d", i),
			LastEventTimestamp: time.Now().Add(-time.Duration(i) * time.Minute),
		}
	}
	return streams
}

func TestModel_ScrollOffset_InitiallyZero(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)

	if m.offset != 0 {
		t.Errorf("expected initial offset 0, got %d", m.offset)
	}
}

func TestModel_ScrollOffset_CursorDownScrollsViewport(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)
	m.width = 100
	m.height = 12 // contentHeight = 12-3 = 9, visible items = 9-1(header) = 8

	// Move cursor to bottom of visible area
	for i := 0; i < 8; i++ {
		m, _ = update(m, keyMsg('j'))
	}

	// Cursor should be at 8, offset should have scrolled to keep cursor visible
	if m.cursor != 8 {
		t.Errorf("expected cursor 8, got %d", m.cursor)
	}
	if m.offset < 1 {
		t.Errorf("expected offset >= 1 when cursor passes visible area, got %d", m.offset)
	}
}

func TestModel_ScrollOffset_CursorUpScrollsBack(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)
	m.width = 100
	m.height = 12
	m.cursor = 20
	m.offset = 15

	// Move cursor up past the offset
	for i := 0; i < 6; i++ {
		m, _ = update(m, keyMsg('k'))
	}

	// Cursor at 14, offset should have scrolled back
	if m.cursor != 14 {
		t.Errorf("expected cursor 14, got %d", m.cursor)
	}
	if m.offset > m.cursor {
		t.Errorf("expected offset <= cursor (%d), got %d", m.cursor, m.offset)
	}
}

func TestModel_ScrollOffset_GJumpsToTopResetsOffset(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)
	m.width = 100
	m.height = 12
	m.cursor = 30
	m.offset = 25

	m, _ = update(m, keyMsg('g'))
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after g, got %d", m.cursor)
	}
	if m.offset != 0 {
		t.Errorf("expected offset 0 after g, got %d", m.offset)
	}
}

func TestModel_ScrollOffset_GJumpsToBottomAdjustsOffset(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)
	m.width = 100
	m.height = 12 // visible items = 8

	m, _ = update(m, keyMsg('G'))
	if m.cursor != 49 {
		t.Errorf("expected cursor 49 after G, got %d", m.cursor)
	}
	// offset should place cursor within visible range
	if m.offset < 42 {
		t.Errorf("expected offset >= 42 to keep cursor 49 visible, got %d", m.offset)
	}
}

func TestModel_ScrollOffset_RenderShowsItemsFromOffset(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)
	m.width = 100
	m.height = 12
	m.cursor = 20
	m.offset = 18

	view := m.View()
	// Items at offset 18, 19, 20 should be visible
	if !strings.Contains(view, "func-018") {
		t.Error("expected func-018 to be visible at offset 18")
	}
	if !strings.Contains(view, "func-020") {
		t.Error("expected func-020 (cursor) to be visible")
	}
	// Item before offset should NOT be visible
	if strings.Contains(view, "func-000") {
		t.Error("expected func-000 to NOT be visible when offset is 18")
	}
}

func TestModel_ScrollOffset_StreamViewScrolls(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logStreams = makeStreams(50)
	m.width = 120
	m.height = 12

	// Move cursor down past visible area
	for i := 0; i < 10; i++ {
		m, _ = update(m, keyMsg('j'))
	}

	if m.cursor != 10 {
		t.Errorf("expected cursor 10, got %d", m.cursor)
	}
	if m.offset < 3 {
		t.Errorf("expected offset >= 3 for streams scroll, got %d", m.offset)
	}
}

func TestModel_ScrollOffset_ResetOnViewChange(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)
	m.width = 100
	m.height = 12
	m.cursor = 20
	m.offset = 15

	// Navigate forward to streams
	m, _ = update(m, keyMsg('l'))
	if m.offset != 0 {
		t.Errorf("expected offset reset to 0 on view change, got %d", m.offset)
	}
}

func TestModel_ScrollOffset_ResetOnNavigateBack(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.logGroups = makeGroups(5)
	m.logStreams = makeStreams(50)
	m.selectedGroup = "/aws/test"
	m.groupCursor = 2
	m.cursor = 20
	m.offset = 15

	m, _ = update(m, keyMsg('h'))
	// offset should be adjusted for the restored groupCursor
	if m.offset > m.cursor {
		t.Errorf("expected offset <= cursor after navigate back, got offset=%d cursor=%d", m.offset, m.cursor)
	}
}

func TestModel_ScrollOffset_InactiveGroupListScrolls(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(50)
	m.currentView = viewStreams
	m.selectedGroup = m.logGroups[45].Name
	m.groupCursor = 45
	m.logStreams = makeStreams(3)
	m.width = 100
	m.height = 12

	view := m.View()
	// The inactive group list should show the selected group
	if !strings.Contains(view, "func-045") {
		t.Error("expected func-045 to be visible in inactive group list")
	}
}

// --- Pane height consistency (#3) ---

// countRenderedLines returns the number of lines in a rendered View output.
func countRenderedLines(view string) int {
	return len(strings.Split(view, "\n"))
}

func TestModel_PaneHeight_BothPanesSameHeight(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 24
	m.logGroups = makeGroups(5) // few groups
	// no streams loaded

	view := m.View()
	lines := strings.Split(view, "\n")

	// Total rendered lines should be m.height-1 to leave room for cursor line
	if len(lines) > m.height-1 {
		t.Errorf("rendered %d lines but max is %d (terminal height %d - 1)", len(lines), m.height-1, m.height)
	}
}

func TestModel_PaneHeight_FewGroupsManyStreams(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 24
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logGroups = makeGroups(3)  // few groups
	m.logStreams = makeStreams(50) // many streams
	m.groupCursor = 0

	view := m.View()
	lines := strings.Split(view, "\n")

	if len(lines) > m.height-1 {
		t.Errorf("rendered %d lines but max is %d (terminal height %d - 1)", len(lines), m.height-1, m.height)
	}
}

func TestModel_PaneHeight_ManyGroupsFewStreams(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 24
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logGroups = makeGroups(50) // many groups
	m.logStreams = makeStreams(2) // few streams
	m.groupCursor = 0

	view := m.View()
	lines := strings.Split(view, "\n")

	if len(lines) > m.height-1 {
		t.Errorf("rendered %d lines but max is %d (terminal height %d - 1)", len(lines), m.height-1, m.height)
	}
}

func TestModel_PaneWidth_TotalFitsTerminal(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 24
	m.logGroups = makeGroups(5)

	view := m.View()
	lines := strings.Split(view, "\n")

	for i, line := range lines {
		// Use RuneCount for visual width approximation (box-drawing chars are single-width)
		w := runeWidth(line)
		if w > m.width {
			t.Errorf("line %d: visual width %d exceeds terminal width %d", i, w, m.width)
		}
	}
}

func TestModel_PaneWidth_TotalFitsTerminal_OddWidth(t *testing.T) {
	m := NewModel(nil)
	m.width = 101 // odd width to test rounding
	m.height = 24
	m.logGroups = makeGroups(5)

	view := m.View()
	lines := strings.Split(view, "\n")

	for i, line := range lines {
		w := runeWidth(line)
		if w > m.width {
			t.Errorf("line %d: visual width %d exceeds terminal width %d", i, w, m.width)
		}
	}
}

// runeWidth counts the visual width of a string.
// East Asian wide characters would need special handling, but
// box-drawing characters are single-width.
func runeWidth(s string) int {
	return len([]rune(s))
}

func TestModel_RenderContentExactLines(t *testing.T) {
	// Verify that renderGroupList and renderStreamList produce exactly maxHeight lines
	// to prevent pane size mismatch when lipgloss Height enforcement varies across terminals.
	tests := []struct {
		name       string
		groups     int
		streams    int
		maxHeight  int
	}{
		{"few groups", 3, 0, 20},
		{"many groups", 50, 0, 20},
		{"exact groups", 20, 0, 20},
		{"few streams", 0, 3, 20},
		{"many streams", 0, 50, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(nil)
			m.width = 100
			m.height = tt.maxHeight + 3
			m.logGroups = makeGroups(tt.groups)
			m.logStreams = makeStreams(tt.streams)
			m.selectedGroup = "/aws/test"

			groupContent := m.renderGroupList(tt.maxHeight)
			groupLines := strings.Count(groupContent, "\n")
			if groupLines != tt.maxHeight {
				t.Errorf("renderGroupList: got %d lines, want %d", groupLines, tt.maxHeight)
			}

			inactiveContent := m.renderGroupListInactive(tt.maxHeight)
			inactiveLines := strings.Count(inactiveContent, "\n")
			if inactiveLines != tt.maxHeight {
				t.Errorf("renderGroupListInactive: got %d lines, want %d", inactiveLines, tt.maxHeight)
			}

			streamContent := m.renderStreamList(tt.maxHeight)
			streamLines := strings.Count(streamContent, "\n")
			if streamLines != tt.maxHeight {
				t.Errorf("renderStreamList: got %d lines, want %d", streamLines, tt.maxHeight)
			}
		})
	}
}

func TestModel_PaneHeight_ExactMatch(t *testing.T) {
	// Both panes must have exactly the same rendered height.
	m := NewModel(nil)
	m.width = 100
	m.height = 24
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logGroups = makeGroups(3)
	m.logStreams = makeStreams(50)

	view := m.View()
	lines := strings.Split(view, "\n")

	// Total should be m.height - 1 lines to prevent top cutoff on some terminals
	if len(lines) != m.height-1 {
		t.Errorf("total lines: got %d, want exactly %d", len(lines), m.height-1)
	}
}

// --- Pagination: fetch all pages ---

func TestModel_FetchLogGroups_StoresNextToken(t *testing.T) {
	m := NewModel(nil)
	token := "next-page-token"

	m, _ = update(m, logGroupsPageMsg{
		groups:    []aws.LogGroup{{Name: "group-1"}},
		nextToken: &token,
	})

	if len(m.logGroups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(m.logGroups))
	}
	if m.groupsNextToken == nil || *m.groupsNextToken != token {
		t.Errorf("expected groupsNextToken=%q, got %v", token, m.groupsNextToken)
	}
}

func TestModel_FetchLogGroups_NilTokenWhenNoMore(t *testing.T) {
	m := NewModel(nil)

	m, _ = update(m, logGroupsPageMsg{
		groups:    []aws.LogGroup{{Name: "group-1"}},
		nextToken: nil,
	})

	if m.groupsNextToken != nil {
		t.Errorf("expected nil groupsNextToken, got %v", m.groupsNextToken)
	}
}

func TestModel_FetchLogStreams_StoresNextToken(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	token := "stream-token"

	m, _ = update(m, logStreamsPageMsg{
		streams:   []aws.LogStream{{Name: "s1"}},
		nextToken: &token,
	})

	if len(m.logStreams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(m.logStreams))
	}
	if m.streamsNextToken == nil || *m.streamsNextToken != token {
		t.Errorf("expected streamsNextToken=%q, got %v", token, m.streamsNextToken)
	}
}

func TestModel_MoreGroupsPage_AppendsToList(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = []aws.LogGroup{{Name: "group-1"}}
	token := "page3"

	m, _ = update(m, moreGroupsPageMsg{
		groups:    []aws.LogGroup{{Name: "group-2"}, {Name: "group-3"}},
		nextToken: &token,
	})

	if len(m.logGroups) != 3 {
		t.Fatalf("expected 3 groups after append, got %d", len(m.logGroups))
	}
	if m.groupsNextToken == nil || *m.groupsNextToken != token {
		t.Errorf("expected groupsNextToken=%q, got %v", token, m.groupsNextToken)
	}
}

func TestModel_MoreStreamsPage_AppendsToList(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.logStreams = []aws.LogStream{{Name: "s1"}}

	m, _ = update(m, moreStreamsPageMsg{
		streams:   []aws.LogStream{{Name: "s2"}, {Name: "s3"}},
		nextToken: nil,
	})

	if len(m.logStreams) != 3 {
		t.Fatalf("expected 3 streams after append, got %d", len(m.logStreams))
	}
	if m.streamsNextToken != nil {
		t.Errorf("expected nil streamsNextToken, got %v", m.streamsNextToken)
	}
}

func TestModel_CursorAtBottom_TriggersMoreGroupsFetch(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(5)
	token := "more"
	m.groupsNextToken = &token
	m.cursor = 3
	m.width = 100
	m.height = 24

	// Move cursor to last item
	m, cmd := update(m, keyMsg('G'))
	if m.cursor != 4 {
		t.Errorf("expected cursor 4, got %d", m.cursor)
	}
	// Should trigger a fetch for more groups
	if cmd == nil {
		t.Error("expected command to fetch more groups when cursor at bottom with more pages")
	}
}

func TestModel_CursorAtBottom_NoFetchWhenNoToken(t *testing.T) {
	m := NewModel(nil)
	m.logGroups = makeGroups(5)
	m.groupsNextToken = nil
	m.width = 100
	m.height = 24

	m, cmd := update(m, keyMsg('G'))
	if cmd != nil {
		t.Error("expected no command when no more pages")
	}
}

func TestModel_CursorAtBottom_TriggersMoreStreamsFetch(t *testing.T) {
	m := NewModel(nil)
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logStreams = makeStreams(5)
	token := "more-streams"
	m.streamsNextToken = &token
	m.cursor = 3
	m.width = 100
	m.height = 24

	m, cmd := update(m, keyMsg('G'))
	if m.cursor != 4 {
		t.Errorf("expected cursor 4, got %d", m.cursor)
	}
	if cmd == nil {
		t.Error("expected command to fetch more streams when cursor at bottom with more pages")
	}
}

func TestModel_PaneHeight_FewItemsBothSides(t *testing.T) {
	// When both panes have few items, they must still have the same height.
	// This verifies padding survives TrimSuffix (not TrimRight which strips all).
	m := NewModel(nil)
	m.width = 100
	m.height = 24
	m.currentView = viewStreams
	m.selectedGroup = "/aws/test"
	m.logGroups = makeGroups(3)
	m.logStreams = makeStreams(2)

	view := m.View()
	lines := strings.Split(view, "\n")

	// Find pane border lines to measure pane heights
	topBorderFound := false
	bottomBorderCount := 0
	for _, line := range lines {
		if strings.Contains(line, "╭") {
			topBorderFound = true
		}
		if strings.Contains(line, "╰") {
			bottomBorderCount++
		}
	}
	if !topBorderFound {
		t.Error("top border not found")
	}
	// Both left and right pane bottom borders should be on the same line
	// (JoinHorizontal places them side by side), so exactly 1 line with ╰
	if bottomBorderCount != 1 {
		t.Errorf("expected 1 bottom border line, got %d (panes have different heights)", bottomBorderCount)
	}
}

func TestModel_PaneHeight_LongGroupNamesWrap(t *testing.T) {
	// When log group names exceed pane width, lipgloss wraps them into extra
	// lines.  The pane must still be capped to the expected height so the
	// bottom border is not pushed off-screen.
	m := NewModel(nil)
	m.width = 60 // narrow — left pane is 20 chars
	m.height = 14
	m.logGroups = []aws.LogGroup{
		{Name: "/aws/lambda/very-long-function-name-that-exceeds-pane-width-0"},
		{Name: "/aws/lambda/very-long-function-name-that-exceeds-pane-width-1"},
		{Name: "/aws/lambda/very-long-function-name-that-exceeds-pane-width-2"},
		{Name: "/aws/lambda/very-long-function-name-that-exceeds-pane-width-3"},
		{Name: "/aws/lambda/very-long-function-name-that-exceeds-pane-width-4"},
	}
	m.cursor = 0

	view := m.View()
	lines := strings.Split(view, "\n")

	// Both pane bottom borders must appear on the same line (exactly 1 line
	// containing ╰), proving the left pane didn't overflow due to wrapping.
	bottomBorderCount := 0
	for _, line := range lines {
		if strings.Contains(line, "╰") {
			bottomBorderCount++
		}
	}
	if bottomBorderCount != 1 {
		t.Errorf("expected 1 bottom border line, got %d (left pane overflowed due to text wrapping)", bottomBorderCount)
	}

	// The bottom border line must contain both left and right pane corners
	for _, line := range lines {
		if strings.Contains(line, "╰") {
			count := strings.Count(line, "╰")
			if count != 2 {
				t.Errorf("expected 2 ╰ on bottom border line (left+right pane), got %d", count)
			}
			break
		}
	}
}
