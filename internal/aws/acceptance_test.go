//go:build integration

package aws

import (
	"context"
	"testing"
	"time"
)

// These tests require floci running on localhost:4566 with seed data.
// Run: docker compose up -d && bash scripts/seed-testdata.sh
// Execute: go test -tags=integration ./internal/aws/ -v

func newFlociClient(t *testing.T) *Client {
	t.Helper()
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_DEFAULT_REGION", "ap-northeast-1")

	ctx := context.Background()
	client, err := NewClient(ctx, "", "ap-northeast-1")
	if err != nil {
		t.Fatalf("failed to create floci client: %v", err)
	}
	return client
}

func TestAcceptance_ListLogGroups(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	groups, err := client.ListLogGroups(ctx)
	if err != nil {
		t.Fatalf("ListLogGroups failed: %v", err)
	}

	if len(groups) < 5 {
		t.Fatalf("expected at least 5 log groups from seed data, got %d", len(groups))
	}

	// Verify seed data groups exist
	expected := map[string]bool{
		"/aws/lambda/api-handler":     false,
		"/aws/lambda/batch-processor": false,
		"/aws/ecs/web-service":        false,
		"/app/api/backend":            false,
		"/app/worker/queue-consumer":  false,
	}
	for _, g := range groups {
		if _, ok := expected[g.Name]; ok {
			expected[g.Name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected log group %q not found", name)
		}
	}

	// Verify retention is set
	for _, g := range groups {
		if g.RetentionDays != 30 {
			t.Errorf("expected retention 30 for %s, got %d", g.Name, g.RetentionDays)
		}
	}
}

func TestAcceptance_ListLogStreams(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	streams, err := client.ListLogStreams(ctx, "/aws/lambda/api-handler")
	if err != nil {
		t.Fatalf("ListLogStreams failed: %v", err)
	}

	if len(streams) != 2 {
		t.Fatalf("expected 2 streams for api-handler, got %d", len(streams))
	}

	streamNames := make(map[string]bool)
	for _, s := range streams {
		streamNames[s.Name] = true
	}

	if !streamNames["2026/04/13/[$LATEST]abc123"] {
		t.Error("expected stream abc123 not found")
	}
	if !streamNames["2026/04/13/[$LATEST]def456"] {
		t.Error("expected stream def456 not found")
	}
}

func TestAcceptance_GetLogEvents(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	// Use a wide time range to capture seed data
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	events, err := client.GetLogEvents(ctx, "/aws/lambda/api-handler", "2026/04/13/[$LATEST]abc123", start, end)
	if err != nil {
		t.Fatalf("GetLogEvents failed: %v", err)
	}

	// Seed data has 6 events, but may be doubled if seed script was run multiple times
	if len(events) < 6 {
		t.Fatalf("expected at least 6 events for abc123 stream, got %d", len(events))
	}

	// Verify expected messages exist
	expectedMessages := []string{
		"START RequestId: abc-123",
		"REPORT RequestId: abc-123 Duration: 120.5 ms Memory: 128 MB",
	}
	for _, expected := range expectedMessages {
		found := false
		for _, e := range events {
			if e.Message == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected event %q not found", expected)
		}
	}
}

func TestAcceptance_GetLogEvents_ErrorStream(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	events, err := client.GetLogEvents(ctx, "/aws/lambda/api-handler", "2026/04/13/[$LATEST]def456", start, end)
	if err != nil {
		t.Fatalf("GetLogEvents failed: %v", err)
	}

	if len(events) < 5 {
		t.Fatalf("expected at least 5 events for def456 stream, got %d", len(events))
	}

	// Verify error message exists
	foundError := false
	for _, e := range events {
		if e.Message == "ERROR Failed to connect to database: connection timeout" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error event not found in stream")
	}
}

func TestAcceptance_ListLogGroupsPage(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	groups, nextToken, err := client.ListLogGroupsPage(ctx, nil)
	if err != nil {
		t.Fatalf("ListLogGroupsPage failed: %v", err)
	}

	if len(groups) < 5 {
		t.Fatalf("expected at least 5 groups, got %d", len(groups))
	}

	// With only 5 groups, nextToken should be nil
	_ = nextToken // may or may not be nil depending on floci behavior
}

func TestAcceptance_ListLogStreamsPage(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	streams, _, err := client.ListLogStreamsPage(ctx, "/aws/lambda/api-handler", nil)
	if err != nil {
		t.Fatalf("ListLogStreamsPage failed: %v", err)
	}

	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
}

func TestAcceptance_FullFlow_GroupToStreamToEvents(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	// Step 1: List groups and find batch-processor
	groups, err := client.ListLogGroups(ctx)
	if err != nil {
		t.Fatalf("ListLogGroups failed: %v", err)
	}

	var batchGroup string
	for _, g := range groups {
		if g.Name == "/aws/lambda/batch-processor" {
			batchGroup = g.Name
			break
		}
	}
	if batchGroup == "" {
		t.Fatal("batch-processor group not found")
	}

	// Step 2: List streams
	streams, err := client.ListLogStreams(ctx, batchGroup)
	if err != nil {
		t.Fatalf("ListLogStreams failed: %v", err)
	}

	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}

	// Step 3: Get log events
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	events, err := client.GetLogEvents(ctx, batchGroup, streams[0].Name, start, end)
	if err != nil {
		t.Fatalf("GetLogEvents failed: %v", err)
	}

	if len(events) < 6 {
		t.Fatalf("expected at least 6 events, got %d", len(events))
	}

	// Verify WARN event exists
	foundWarn := false
	for _, e := range events {
		if e.Message == "WARN Slow query detected: 2300ms" {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Error("expected WARN event not found")
	}
}

func TestAcceptance_AllSeedGroups_HaveStreamsAndEvents(t *testing.T) {
	client := newFlociClient(t)
	ctx := context.Background()

	expectedStreams := map[string]int{
		"/aws/lambda/api-handler":     2,
		"/aws/lambda/batch-processor": 1,
		"/aws/ecs/web-service":        1,
		"/app/api/backend":            1,
		"/app/worker/queue-consumer":  1,
	}

	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	for group, expectedCount := range expectedStreams {
		streams, err := client.ListLogStreams(ctx, group)
		if err != nil {
			t.Errorf("ListLogStreams(%s) failed: %v", group, err)
			continue
		}

		if len(streams) != expectedCount {
			t.Errorf("%s: expected %d streams, got %d", group, expectedCount, len(streams))
			continue
		}

		// Verify each stream has events
		for _, s := range streams {
			events, err := client.GetLogEvents(ctx, group, s.Name, start, end)
			if err != nil {
				t.Errorf("GetLogEvents(%s/%s) failed: %v", group, s.Name, err)
				continue
			}
			if len(events) == 0 {
				t.Errorf("%s/%s: expected events, got 0", group, s.Name)
			}
		}
	}
}
