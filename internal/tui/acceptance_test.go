//go:build integration

package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myuron/lazycwl/internal/aws"
)

func newFlociModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_DEFAULT_REGION", "ap-northeast-1")

	ctx := context.Background()
	client, err := aws.NewClient(ctx, "", "ap-northeast-1")
	if err != nil {
		t.Fatalf("failed to create floci client: %v", err)
	}
	return NewModel(client)
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

// execCmd runs a tea.Cmd synchronously and feeds the result back into Update.
// It skips tea.ExecProcess commands (editor launch) to avoid spawning processes in tests.
func execCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	// Skip editor process execution in tests
	if _, ok := msg.(tea.ExecMsg); ok {
		return m
	}
	// Handle tea.BatchMsg (multiple commands batched together)
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = execCmd(t, m, c)
		}
		return m
	}
	m, newCmd := updateModel(m, msg)
	if newCmd != nil {
		return execCmd(t, m, newCmd)
	}
	return m
}

func TestAcceptance_TUI_InitLoadsGroups(t *testing.T) {
	m := newFlociModel(t)

	cmd := m.Init()
	m = execCmd(t, m, cmd)

	if len(m.logGroups) < 5 {
		t.Fatalf("expected at least 5 log groups, got %d", len(m.logGroups))
	}
	if m.loading {
		t.Error("expected loading to be false after init")
	}
}

func TestAcceptance_TUI_NavigateToStreams(t *testing.T) {
	m := newFlociModel(t)

	// Init: load groups
	cmd := m.Init()
	m = execCmd(t, m, cmd)

	// Find /aws/lambda/api-handler and navigate to it
	targetIdx := -1
	for i, g := range m.logGroups {
		if g.Name == "/aws/lambda/api-handler" {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		t.Fatal("api-handler group not found")
	}

	// Move cursor to target
	for i := 0; i < targetIdx; i++ {
		m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// Navigate forward
	m, cmd2 := updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = execCmd(t, m, cmd2)

	if m.currentView != viewStreams {
		t.Errorf("expected viewStreams, got %d", m.currentView)
	}
	if m.selectedGroup != "/aws/lambda/api-handler" {
		t.Errorf("expected selected group /aws/lambda/api-handler, got %s", m.selectedGroup)
	}
	if len(m.logStreams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(m.logStreams))
	}
}

func TestAcceptance_TUI_NavigateBackPreservesCursor(t *testing.T) {
	m := newFlociModel(t)

	cmd := m.Init()
	m = execCmd(t, m, cmd)

	// Move to index 2
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	savedCursor := m.cursor

	// Navigate forward
	m, cmd2 := updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = execCmd(t, m, cmd2)

	// Navigate back
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

	if m.currentView != viewGroups {
		t.Errorf("expected viewGroups, got %d", m.currentView)
	}
	if m.cursor != savedCursor {
		t.Errorf("expected cursor %d, got %d", savedCursor, m.cursor)
	}
}

func TestAcceptance_TUI_SearchFilterGroups(t *testing.T) {
	m := newFlociModel(t)

	cmd := m.Init()
	m = execCmd(t, m, cmd)

	// Enter search mode and type "lambda"
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "lambda" {
		m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	filtered := m.filteredGroups()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 lambda groups, got %d", len(filtered))
	}
	for _, g := range filtered {
		if !strings.Contains(g.Name, "lambda") {
			t.Errorf("filtered group %q does not contain 'lambda'", g.Name)
		}
	}
}

func TestAcceptance_TUI_SearchFilterStreams(t *testing.T) {
	m := newFlociModel(t)

	cmd := m.Init()
	m = execCmd(t, m, cmd)

	// Navigate to api-handler
	for i, g := range m.logGroups {
		if g.Name == "/aws/lambda/api-handler" {
			m.cursor = i
			break
		}
	}

	m, cmd2 := updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = execCmd(t, m, cmd2)

	// Search for "abc"
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "abc" {
		m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	filtered := m.filteredStreams()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 stream matching 'abc', got %d", len(filtered))
	}
	if !strings.Contains(filtered[0].Name, "abc") {
		t.Errorf("filtered stream %q does not contain 'abc'", filtered[0].Name)
	}
}

func TestAcceptance_TUI_ViewRendersWithFlociData(t *testing.T) {
	m := newFlociModel(t)
	m.width = 120
	m.height = 40

	cmd := m.Init()
	m = execCmd(t, m, cmd)

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if !strings.Contains(view, "Log Groups") {
		t.Error("expected 'Log Groups' in view")
	}
	if !strings.Contains(view, "/aws/lambda/api-handler") {
		t.Error("expected api-handler in view")
	}
}

func TestAcceptance_TUI_FullNavigation_GroupToStreamToLogFetch(t *testing.T) {
	m := newFlociModel(t)

	cmd := m.Init()
	m = execCmd(t, m, cmd)

	// Find and navigate to a group with known single stream
	for i, g := range m.logGroups {
		if g.Name == "/aws/lambda/batch-processor" {
			m.cursor = i
			break
		}
	}

	// Navigate to streams
	m, cmd2 := updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = execCmd(t, m, cmd2)

	if m.currentView != viewStreams {
		t.Fatalf("expected viewStreams, got %d", m.currentView)
	}
	if len(m.logStreams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(m.logStreams))
	}

	// Navigate forward to fetch log events — this triggers openEditor,
	// but since we're in test we just verify the cmd is returned and loading is set.
	m, cmd3 := updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if !m.loading {
		t.Error("expected loading to be true while fetching events")
	}
	if cmd3 == nil {
		t.Error("expected command to fetch log events")
	}

	// Execute the fetch command — it will return logEventsMsg
	msg := cmd3()
	if eventsMsg, ok := msg.(logEventsMsg); ok {
		if len(eventsMsg) < 6 {
			t.Errorf("expected at least 6 events, got %d", len(eventsMsg))
		}
	} else {
		t.Errorf("expected logEventsMsg, got %T", msg)
	}
}

func TestAcceptance_TUI_WithOptions_InitialGroup(t *testing.T) {
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_DEFAULT_REGION", "ap-northeast-1")

	ctx := context.Background()
	client, err := aws.NewClient(ctx, "", "ap-northeast-1")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	m := NewModelWithOptions(client, Options{
		InitialGroup: "/aws/ecs/web-service",
	})

	// Should start in streams view
	if m.currentView != viewStreams {
		t.Fatalf("expected viewStreams with InitialGroup, got %d", m.currentView)
	}

	// Init fetches both groups and streams
	cmd := m.Init()
	m = execCmd(t, m, cmd)

	if len(m.logGroups) < 5 {
		t.Errorf("expected groups to be loaded, got %d", len(m.logGroups))
	}
	if len(m.logStreams) != 1 {
		t.Errorf("expected 1 stream for web-service, got %d", len(m.logStreams))
	}
}
