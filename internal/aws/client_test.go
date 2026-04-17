package aws

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// mockLogsAPI implements LogsClient for testing.
type mockLogsAPI struct {
	describeLogGroupsFn  func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	describeLogStreamsFn func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	getLogEventsFn       func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
}

func (m *mockLogsAPI) DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return m.describeLogGroupsFn(ctx, params, optFns...)
}

func (m *mockLogsAPI) DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	return m.describeLogStreamsFn(ctx, params, optFns...)
}

func (m *mockLogsAPI) GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	return m.getLogEventsFn(ctx, params, optFns...)
}

func TestClient_ListLogGroups(t *testing.T) {
	mock := &mockLogsAPI{
		describeLogGroupsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []types.LogGroup{
					{
						LogGroupName: aws.String("/aws/lambda/func-a"),
						RetentionInDays: aws.Int32(30),
						StoredBytes:     aws.Int64(1024),
					},
					{
						LogGroupName: aws.String("/aws/ecs/service-b"),
						RetentionInDays: aws.Int32(7),
						StoredBytes:     aws.Int64(2048),
					},
				},
			}, nil
		},
	}

	client := &Client{api: mock}
	groups, err := client.ListLogGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	if groups[0].Name != "/aws/lambda/func-a" {
		t.Errorf("expected group name /aws/lambda/func-a, got %s", groups[0].Name)
	}
	if groups[0].RetentionDays != 30 {
		t.Errorf("expected retention 30, got %d", groups[0].RetentionDays)
	}
	if groups[0].StoredBytes != 1024 {
		t.Errorf("expected stored bytes 1024, got %d", groups[0].StoredBytes)
	}
}

func TestClient_ListLogStreams(t *testing.T) {
	now := time.Now()
	nowMs := now.UnixMilli()

	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			if aws.ToString(params.LogGroupName) != "/aws/lambda/func-a" {
				t.Errorf("expected log group /aws/lambda/func-a, got %s", aws.ToString(params.LogGroupName))
			}
			if params.OrderBy != types.OrderByLastEventTime {
				t.Errorf("expected OrderBy=LastEventTime, got %v", params.OrderBy)
			}
			if !aws.ToBool(params.Descending) {
				t.Error("expected Descending=true")
			}
			if aws.ToInt32(params.Limit) != 50 {
				t.Errorf("expected Limit=50, got %d", aws.ToInt32(params.Limit))
			}
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{
						LogStreamName:        aws.String("stream-001"),
						LastEventTimestamp:    aws.Int64(nowMs),
						StoredBytes:          aws.Int64(4096),
					},
					{
						LogStreamName:        aws.String("stream-002"),
						LastEventTimestamp:    aws.Int64(nowMs - 60000),
						StoredBytes:          aws.Int64(8192),
					},
				},
			}, nil
		},
	}

	client := &Client{api: mock}
	streams, err := client.ListLogStreams(context.Background(), "/aws/lambda/func-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}

	if streams[0].Name != "stream-001" {
		t.Errorf("expected stream name stream-001, got %s", streams[0].Name)
	}
	if streams[0].StoredBytes != 4096 {
		t.Errorf("expected stored bytes 4096, got %d", streams[0].StoredBytes)
	}
	if streams[1].StoredBytes != 8192 {
		t.Errorf("expected stored bytes 8192, got %d", streams[1].StoredBytes)
	}
}

func TestClient_ListLogStreamsPage(t *testing.T) {
	now := time.Now()
	nowMs := now.UnixMilli()

	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			if aws.ToString(params.LogGroupName) != "/aws/lambda/func-a" {
				t.Errorf("expected log group /aws/lambda/func-a, got %s", aws.ToString(params.LogGroupName))
			}
			if params.OrderBy != types.OrderByLastEventTime {
				t.Errorf("expected OrderBy=LastEventTime, got %v", params.OrderBy)
			}
			if !aws.ToBool(params.Descending) {
				t.Error("expected Descending=true")
			}
			if aws.ToInt32(params.Limit) != 50 {
				t.Errorf("expected Limit=50, got %d", aws.ToInt32(params.Limit))
			}
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{
						LogStreamName:     aws.String("stream-001"),
						LastEventTimestamp: aws.Int64(nowMs),
						StoredBytes:       aws.Int64(4096),
					},
				},
				NextToken: aws.String("next-page"),
			}, nil
		},
	}

	client := &Client{api: mock}
	streams, nextToken, err := client.ListLogStreamsPage(context.Background(), "/aws/lambda/func-a", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].Name != "stream-001" {
		t.Errorf("expected stream name stream-001, got %s", streams[0].Name)
	}
	if nextToken == nil || *nextToken != "next-page" {
		t.Errorf("expected nextToken=next-page, got %v", nextToken)
	}
}

func TestClient_GetLogEvents(t *testing.T) {
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			if aws.ToString(params.LogGroupName) != "/aws/lambda/func-a" {
				t.Errorf("expected log group /aws/lambda/func-a, got %s", aws.ToString(params.LogGroupName))
			}
			if aws.ToString(params.LogStreamName) != "stream-001" {
				t.Errorf("expected log stream stream-001, got %s", aws.ToString(params.LogStreamName))
			}
			if params.StartTime != nil {
				t.Errorf("expected StartTime to be nil, got %d", *params.StartTime)
			}
			if params.EndTime != nil {
				t.Errorf("expected EndTime to be nil, got %d", *params.EndTime)
			}
			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []types.OutputLogEvent{
					{
						Timestamp: aws.Int64(1705312200000),
						Message:   aws.String("START RequestId: abc-123"),
					},
					{
						Timestamp: aws.Int64(1705312201000),
						Message:   aws.String("END RequestId: abc-123"),
					},
				},
			}, nil
		},
	}

	client := &Client{api: mock}
	events, err := client.GetLogEvents(context.Background(), "/aws/lambda/func-a", "stream-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Message != "START RequestId: abc-123" {
		t.Errorf("expected message 'START RequestId: abc-123', got %s", events[0].Message)
	}
}

func TestClient_GetLogEvents_Pagination(t *testing.T) {
	// Simulate 3 pages of log events.
	// GetLogEvents returns NextForwardToken; pagination stops when the token
	// is the same as the previous one.
	callCount := 0
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			callCount++
			switch callCount {
			case 1:
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: []types.OutputLogEvent{
						{Timestamp: aws.Int64(1000), Message: aws.String("event-1")},
						{Timestamp: aws.Int64(2000), Message: aws.String("event-2")},
					},
					NextForwardToken: aws.String("token-page2"),
				}, nil
			case 2:
				if aws.ToString(params.NextToken) != "token-page2" {
					t.Errorf("expected NextToken=token-page2, got %s", aws.ToString(params.NextToken))
				}
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: []types.OutputLogEvent{
						{Timestamp: aws.Int64(3000), Message: aws.String("event-3")},
					},
					NextForwardToken: aws.String("token-page3"),
				}, nil
			case 3:
				// Empty page with same token signals end of pagination.
				return &cloudwatchlogs.GetLogEventsOutput{
					Events:           []types.OutputLogEvent{},
					NextForwardToken: aws.String("token-page3"),
				}, nil
			default:
				t.Fatal("unexpected extra API call")
				return nil, nil
			}
		},
	}

	client := &Client{api: mock}
	events, err := client.GetLogEvents(context.Background(), "group", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events across pages, got %d", len(events))
	}
	if events[2].Message != "event-3" {
		t.Errorf("expected last event 'event-3', got %s", events[2].Message)
	}
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

func TestClient_GetLogEvents_PaginationStopsOnSameToken(t *testing.T) {
	// GetLogEvents pagination: the first call returns a token. The second call
	// uses that token and gets the same token back, signaling end-of-stream.
	// Total: 2 API calls, events from both are collected.
	callCount := 0
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			callCount++
			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []types.OutputLogEvent{
					{Timestamp: aws.Int64(int64(callCount) * 1000), Message: aws.String(fmt.Sprintf("event-%d", callCount))},
				},
				NextForwardToken: aws.String("same-token"),
			}, nil
		},
	}

	client := &Client{api: mock}
	events, err := client.GetLogEvents(context.Background(), "group", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events (first call + second call with same token), got %d", len(events))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (stop on same token), got %d", callCount)
	}
}

func TestClient_GetLogEvents_PaginationStopsOnEmptyPage(t *testing.T) {
	// If a page returns 0 events, pagination should stop even if the token changes.
	callCount := 0
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			callCount++
			if callCount == 1 {
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: []types.OutputLogEvent{
						{Timestamp: aws.Int64(1000), Message: aws.String("event-1")},
					},
					NextForwardToken: aws.String("token-2"),
				}, nil
			}
			// Second page: empty events
			return &cloudwatchlogs.GetLogEventsOutput{
				Events:           []types.OutputLogEvent{},
				NextForwardToken: aws.String("token-3"),
			}, nil
		},
	}

	client := &Client{api: mock}
	events, err := client.GetLogEvents(context.Background(), "group", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event (stop on empty page), got %d", len(events))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestClient_GetLogEvents_ContextCancellation(t *testing.T) {
	// Pagination should respect context cancellation.
	callCount := 0
	ctx, cancel := context.WithCancel(context.Background())
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			callCount++
			if callCount == 2 {
				cancel()
				return nil, ctx.Err()
			}
			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []types.OutputLogEvent{
					{Timestamp: aws.Int64(1000), Message: aws.String("event")},
				},
				NextForwardToken: aws.String(fmt.Sprintf("token-%d", callCount)),
			}, nil
		},
	}

	client := &Client{api: mock}
	_, err := client.GetLogEvents(ctx, "group", "stream")
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
}

func TestFetchMultiLogEvents_ConcurrencyLimit(t *testing.T) {
	// Verify that concurrent goroutines are limited.
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			cur := current.Add(1)
			// Track the peak concurrency
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			current.Add(-1)
			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []types.OutputLogEvent{
					{Timestamp: aws.Int64(1000), Message: aws.String("event")},
				},
				NextForwardToken: aws.String("same"),
			}, nil
		},
	}

	client := &Client{api: mock}
	// Request 20 streams to ensure the limit is tested
	streams := make([]string, 20)
	for i := range streams {
		streams[i] = fmt.Sprintf("stream-%d", i)
	}

	ctx := context.Background()
	_, err := client.GetMultiStreamLogEvents(ctx, "group", streams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peak := maxConcurrent.Load()
	if peak > maxConcurrentStreams {
		t.Errorf("peak concurrency %d exceeded limit %d", peak, maxConcurrentStreams)
	}
	if peak == 0 {
		t.Error("expected at least 1 concurrent call")
	}
}
