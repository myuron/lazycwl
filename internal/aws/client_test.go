package aws

import (
	"context"
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
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []types.LogStream{
					{
						LogStreamName:        aws.String("stream-001"),
						LastEventTimestamp:    aws.Int64(nowMs),
					},
					{
						LogStreamName:        aws.String("stream-002"),
						LastEventTimestamp:    aws.Int64(nowMs - 60000),
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

func TestClient_GetLogEvents(t *testing.T) {
	mock := &mockLogsAPI{
		getLogEventsFn: func(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
			if aws.ToString(params.LogGroupName) != "/aws/lambda/func-a" {
				t.Errorf("expected log group /aws/lambda/func-a, got %s", aws.ToString(params.LogGroupName))
			}
			if aws.ToString(params.LogStreamName) != "stream-001" {
				t.Errorf("expected log stream stream-001, got %s", aws.ToString(params.LogStreamName))
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
	startTime := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	events, err := client.GetLogEvents(context.Background(), "/aws/lambda/func-a", "stream-001", startTime, endTime)
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
