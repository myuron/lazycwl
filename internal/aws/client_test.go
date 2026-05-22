package aws

import (
	"context"
	"fmt"
	"sync"
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
	startLiveTailFn      func(ctx context.Context, params *cloudwatchlogs.StartLiveTailInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartLiveTailOutput, error)
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

func (m *mockLogsAPI) StartLiveTail(ctx context.Context, params *cloudwatchlogs.StartLiveTailInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartLiveTailOutput, error) {
	if m.startLiveTailFn != nil {
		return m.startLiveTailFn(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("startLiveTailFn not set")
}

func TestClient_ListLogGroups(t *testing.T) {
	mock := &mockLogsAPI{
		describeLogGroupsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []types.LogGroup{
					{
						LogGroupName:    aws.String("/aws/lambda/func-a"),
						Arn:             aws.String("arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/func-a"),
						RetentionInDays: aws.Int32(30),
						StoredBytes:     aws.Int64(1024),
					},
					{
						LogGroupName:    aws.String("/aws/ecs/service-b"),
						Arn:             aws.String("arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/ecs/service-b"),
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
	if groups[0].ARN != "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/func-a" {
		t.Errorf("expected ARN, got %s", groups[0].ARN)
	}
	if groups[0].RetentionDays != 30 {
		t.Errorf("expected retention 30, got %d", groups[0].RetentionDays)
	}
	if groups[0].StoredBytes != 1024 {
		t.Errorf("expected stored bytes 1024, got %d", groups[0].StoredBytes)
	}
}

func TestClient_StartLiveTailSession(t *testing.T) {
	mock := &mockLogsAPI{
		startLiveTailFn: func(ctx context.Context, params *cloudwatchlogs.StartLiveTailInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartLiveTailOutput, error) {
			if len(params.LogGroupIdentifiers) != 1 {
				t.Errorf("expected 1 log group identifier, got %d", len(params.LogGroupIdentifiers))
			}
			if params.LogGroupIdentifiers[0] != "arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/func-a" {
				t.Errorf("unexpected log group ARN: %s", params.LogGroupIdentifiers[0])
			}
			if len(params.LogStreamNames) != 2 {
				t.Errorf("expected 2 stream names, got %d", len(params.LogStreamNames))
			}
			return &cloudwatchlogs.StartLiveTailOutput{}, nil
		},
	}

	client := &Client{api: mock}
	_, err := client.StartLiveTailSession(
		context.Background(),
		"arn:aws:logs:ap-northeast-1:123456789012:log-group:/aws/lambda/func-a",
		[]string{"stream-001", "stream-002"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_StartLiveTailSession_NoStreams(t *testing.T) {
	mock := &mockLogsAPI{
		startLiveTailFn: func(ctx context.Context, params *cloudwatchlogs.StartLiveTailInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartLiveTailOutput, error) {
			if len(params.LogStreamNames) != 0 {
				t.Errorf("expected no stream names, got %d", len(params.LogStreamNames))
			}
			return &cloudwatchlogs.StartLiveTailOutput{}, nil
		},
	}

	client := &Client{api: mock}
	_, err := client.StartLiveTailSession(context.Background(), "arn:test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
						LogStreamName:      aws.String("stream-001"),
						LastEventTimestamp: aws.Int64(nowMs),
					},
					{
						LogStreamName:      aws.String("stream-002"),
						LastEventTimestamp: aws.Int64(nowMs - 60000),
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
						LogStreamName:      aws.String("stream-001"),
						LastEventTimestamp: aws.Int64(nowMs),
					},
				},
				NextToken: aws.String("next-page"),
			}, nil
		},
	}

	client := &Client{api: mock}
	streams, nextToken, err := client.ListLogStreamsPage(context.Background(), "/aws/lambda/func-a", nil, true)
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

func TestClient_ListLogStreamsPage_Ascending(t *testing.T) {
	now := time.Now()
	nowMs := now.UnixMilli()

	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			if params.OrderBy != types.OrderByLastEventTime {
				t.Errorf("expected OrderBy=LastEventTime, got %v", params.OrderBy)
			}
			if aws.ToBool(params.Descending) {
				t.Error("expected Descending=false for ascending order")
			}
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{
						LogStreamName:      aws.String("stream-old"),
						LastEventTimestamp: aws.Int64(nowMs - 60000),
					},
					{
						LogStreamName:      aws.String("stream-new"),
						LastEventTimestamp: aws.Int64(nowMs),
					},
				},
			}, nil
		},
	}

	client := &Client{api: mock}
	streams, _, err := client.ListLogStreamsPage(context.Background(), "/aws/lambda/func-a", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
	if streams[0].Name != "stream-old" {
		t.Errorf("expected stream-old first (ascending), got %s", streams[0].Name)
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
	// Verify that concurrent GetLogEvents goroutines are bounded by the
	// global semaphore. DescribeLogStreams returns no event-time range, so
	// each stream falls back to a single sequential GetLogEvents call —
	// keeping this test focused on the multi-stream concurrency cap.
	var peakConcurrent atomic.Int32
	var current atomic.Int32

	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{LogStreamName: params.LogStreamNamePrefix},
				},
			}, nil
		},
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			cur := current.Add(1)
			for {
				old := peakConcurrent.Load()
				if cur <= old || peakConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
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
	streams := make([]string, 20)
	for i := range streams {
		streams[i] = fmt.Sprintf("stream-%d", i)
	}

	ctx := context.Background()
	_, err := client.GetMultiStreamLogEvents(ctx, "group", streams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peak := peakConcurrent.Load()
	if peak > maxConcurrent {
		t.Errorf("peak concurrency %d exceeded limit %d", peak, maxConcurrent)
	}
	if peak == 0 {
		t.Error("expected at least 1 concurrent call")
	}
}

func TestClient_DescribeLogStream(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			if aws.ToString(params.LogGroupName) != "/aws/lambda/func-a" {
				t.Errorf("unexpected log group: %s", aws.ToString(params.LogGroupName))
			}
			if aws.ToString(params.LogStreamNamePrefix) != "stream-001" {
				t.Errorf("expected LogStreamNamePrefix=stream-001, got %s", aws.ToString(params.LogStreamNamePrefix))
			}
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{
						LogStreamName:       aws.String("stream-001-other"),
						FirstEventTimestamp: aws.Int64(nowMs - 200000),
						LastEventTimestamp:  aws.Int64(nowMs - 100000),
					},
					{
						LogStreamName:       aws.String("stream-001"),
						FirstEventTimestamp: aws.Int64(nowMs - 60000),
						LastEventTimestamp:  aws.Int64(nowMs),
					},
				},
			}, nil
		},
	}

	client := &Client{api: mock}
	stream, err := client.DescribeLogStream(context.Background(), "/aws/lambda/func-a", "stream-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream.Name != "stream-001" {
		t.Errorf("expected exact-match Name=stream-001, got %s", stream.Name)
	}
	if got := stream.FirstEventTimestamp.UnixMilli(); got != nowMs-60000 {
		t.Errorf("expected FirstEventTimestamp=%d, got %d", nowMs-60000, got)
	}
	if got := stream.LastEventTimestamp.UnixMilli(); got != nowMs {
		t.Errorf("expected LastEventTimestamp=%d, got %d", nowMs, got)
	}
}

func TestClient_DescribeLogStream_NotFound(t *testing.T) {
	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{LogStreams: nil}, nil
		},
	}
	client := &Client{api: mock}
	if _, err := client.DescribeLogStream(context.Background(), "g", "missing"); err == nil {
		t.Fatal("expected error when stream is not found")
	}
}

func TestClient_GetLogEventsByTimeRange(t *testing.T) {
	calls := 0
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			calls++
			if got := aws.ToInt64(params.StartTime); got != 1000 {
				t.Errorf("expected StartTime=1000, got %d", got)
			}
			if got := aws.ToInt64(params.EndTime); got != 5000 {
				t.Errorf("expected EndTime=5000, got %d", got)
			}
			if !aws.ToBool(params.StartFromHead) {
				t.Error("expected StartFromHead=true")
			}
			// Empty events on the second call terminates pagination.
			if calls == 1 {
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: []types.OutputLogEvent{
						{Timestamp: aws.Int64(1000), Message: aws.String("a")},
						{Timestamp: aws.Int64(4000), Message: aws.String("b")},
					},
					NextForwardToken: aws.String("next"),
				}, nil
			}
			return &cloudwatchlogs.GetLogEventsOutput{
				Events:           []types.OutputLogEvent{},
				NextForwardToken: aws.String("next"),
			}, nil
		},
	}
	client := &Client{api: mock}
	events, err := client.GetLogEventsByTimeRange(context.Background(), "g", "s", 1000, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestClient_GetLogEventsByTimeRange_Pagination(t *testing.T) {
	// Within a single time window, pagination still uses NextForwardToken.
	calls := 0
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			calls++
			if aws.ToInt64(params.StartTime) != 100 || aws.ToInt64(params.EndTime) != 200 {
				t.Errorf("unexpected time window: start=%d end=%d", aws.ToInt64(params.StartTime), aws.ToInt64(params.EndTime))
			}
			switch calls {
			case 1:
				return &cloudwatchlogs.GetLogEventsOutput{
					Events:           []types.OutputLogEvent{{Timestamp: aws.Int64(100), Message: aws.String("e1")}},
					NextForwardToken: aws.String("t2"),
				}, nil
			case 2:
				if aws.ToString(params.NextToken) != "t2" {
					t.Errorf("expected NextToken=t2, got %s", aws.ToString(params.NextToken))
				}
				return &cloudwatchlogs.GetLogEventsOutput{
					Events:           []types.OutputLogEvent{{Timestamp: aws.Int64(150), Message: aws.String("e2")}},
					NextForwardToken: aws.String("t2"),
				}, nil
			}
			t.Fatal("too many calls")
			return nil, nil
		},
	}
	client := &Client{api: mock}
	events, err := client.GetLogEventsByTimeRange(context.Background(), "g", "s", 100, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 || calls != 2 {
		t.Errorf("expected 2 events from 2 calls, got %d events / %d calls", len(events), calls)
	}
}

func TestClient_GetMultiStreamLogEvents_TimeChunksInParallel(t *testing.T) {
	// When DescribeLogStreams returns a wide time range, GetMultiStreamLogEvents
	// should split the range into multiple chunks and fetch them in parallel,
	// reducing wall-clock time vs. sequential pagination.
	var getCalls atomic.Int32
	var peakConcurrent atomic.Int32
	var current atomic.Int32

	var startsMu sync.Mutex
	seenWindows := make(map[[2]int64]bool)

	const firstMs = int64(1_000_000)
	const lastMs = int64(9_000_000)

	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{
						LogStreamName:       params.LogStreamNamePrefix,
						FirstEventTimestamp: aws.Int64(firstMs),
						LastEventTimestamp:  aws.Int64(lastMs),
					},
				},
			}, nil
		},
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			getCalls.Add(1)
			cur := current.Add(1)
			for {
				old := peakConcurrent.Load()
				if cur <= old || peakConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			current.Add(-1)

			startsMu.Lock()
			seenWindows[[2]int64{aws.ToInt64(params.StartTime), aws.ToInt64(params.EndTime)}] = true
			startsMu.Unlock()

			return &cloudwatchlogs.GetLogEventsOutput{
				Events: []types.OutputLogEvent{
					{Timestamp: params.StartTime, Message: aws.String("e")},
				},
				NextForwardToken: aws.String("end"),
			}, nil
		},
	}

	client := &Client{api: mock}
	events, err := client.GetMultiStreamLogEvents(context.Background(), "g", []string{"stream-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if getCalls.Load() < 2 {
		t.Errorf("expected multiple chunked GetLogEvents calls, got %d", getCalls.Load())
	}
	if peakConcurrent.Load() < 2 {
		t.Errorf("expected concurrent execution across chunks, got peak=%d", peakConcurrent.Load())
	}
	if len(events) < 2 {
		t.Errorf("expected at least 2 events from chunked fetch, got %d", len(events))
	}

	// Time windows must be non-overlapping and cover [firstMs, lastMs+1).
	startsMu.Lock()
	defer startsMu.Unlock()
	type window struct{ start, end int64 }
	windows := make([]window, 0, len(seenWindows))
	for k := range seenWindows {
		windows = append(windows, window{k[0], k[1]})
	}
	// Sort by start ascending.
	for i := 0; i < len(windows); i++ {
		for j := i + 1; j < len(windows); j++ {
			if windows[j].start < windows[i].start {
				windows[i], windows[j] = windows[j], windows[i]
			}
		}
	}
	if windows[0].start != firstMs {
		t.Errorf("first window start=%d, expected %d", windows[0].start, firstMs)
	}
	if windows[len(windows)-1].end != lastMs+1 {
		t.Errorf("last window end=%d, expected %d", windows[len(windows)-1].end, lastMs+1)
	}
	for i := 1; i < len(windows); i++ {
		if windows[i].start != windows[i-1].end {
			t.Errorf("window gap or overlap: prev=[%d,%d) next=[%d,%d)",
				windows[i-1].start, windows[i-1].end, windows[i].start, windows[i].end)
		}
	}
}

func TestClient_GetMultiStreamLogEvents_FallbackWhenNoTimeRange(t *testing.T) {
	// DescribeLogStreams returns a stream with no FirstEventTimestamp; the
	// client must fall back to a plain sequential GetLogEvents (no time
	// chunking) for that stream and still return its events.
	var startTimesSeen atomic.Int32
	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{LogStreamName: params.LogStreamNamePrefix},
				},
			}, nil
		},
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			if params.StartTime != nil || params.EndTime != nil {
				startTimesSeen.Add(1)
			}
			return &cloudwatchlogs.GetLogEventsOutput{
				Events:           []types.OutputLogEvent{{Timestamp: aws.Int64(1), Message: aws.String("e")}},
				NextForwardToken: aws.String("same"),
			}, nil
		},
	}
	client := &Client{api: mock}
	events, err := client.GetMultiStreamLogEvents(context.Background(), "g", []string{"s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least one event from fallback path")
	}
	if startTimesSeen.Load() != 0 {
		t.Errorf("expected sequential fallback (no StartTime/EndTime), got %d windowed calls", startTimesSeen.Load())
	}
}

func TestClient_ListLogStreamsPage_PopulatesFirstEventTimestamp(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	mock := &mockLogsAPI{
		describeLogStreamsFn: func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{
						LogStreamName:       aws.String("s"),
						FirstEventTimestamp: aws.Int64(nowMs - 60000),
						LastEventTimestamp:  aws.Int64(nowMs),
					},
				},
			}, nil
		},
	}
	client := &Client{api: mock}
	streams, _, err := client.ListLogStreamsPage(context.Background(), "g", nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if got := streams[0].FirstEventTimestamp.UnixMilli(); got != nowMs-60000 {
		t.Errorf("expected FirstEventTimestamp=%d, got %d", nowMs-60000, got)
	}
}
