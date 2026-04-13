package formatter

import (
	"strings"
	"testing"
	"time"

	"github.com/myuron/lazycwl/internal/aws"
)

func TestFormat_SingleEvent(t *testing.T) {
	ts := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	events := []aws.LogEvent{
		{Timestamp: ts, Message: "START RequestId: abc-123"},
	}

	result := Format(events)
	expected := "[2024-01-15T09:30:00.000Z] START RequestId: abc-123\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormat_MultipleEvents(t *testing.T) {
	ts1 := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	ts2 := time.Date(2024, 1, 15, 9, 30, 1, 0, time.UTC)
	events := []aws.LogEvent{
		{Timestamp: ts1, Message: "START RequestId: abc-123"},
		{Timestamp: ts2, Message: "END RequestId: abc-123"},
	}

	result := Format(events)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "[2024-01-15T09:30:00.000Z]") {
		t.Errorf("unexpected first line: %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "[2024-01-15T09:30:01.000Z]") {
		t.Errorf("unexpected second line: %s", lines[1])
	}
}

func TestFormat_EmptyEvents(t *testing.T) {
	result := Format(nil)
	if result != "" {
		t.Errorf("expected empty string for nil events, got %q", result)
	}
}

func TestFormat_SortsByTimestamp(t *testing.T) {
	ts1 := time.Date(2024, 1, 15, 9, 30, 1, 0, time.UTC)
	ts2 := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	events := []aws.LogEvent{
		{Timestamp: ts1, Message: "second"},
		{Timestamp: ts2, Message: "first"},
	}

	result := Format(events)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if !strings.Contains(lines[0], "first") {
		t.Errorf("expected first event to be sorted first, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "second") {
		t.Errorf("expected second event to be sorted second, got: %s", lines[1])
	}
}
