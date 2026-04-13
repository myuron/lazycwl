package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// LogsClient is the interface for CloudWatch Logs API operations.
type LogsClient interface {
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
}

// LogGroup represents a CloudWatch Logs log group.
type LogGroup struct {
	Name          string
	RetentionDays int32
	StoredBytes   int64
}

// LogStream represents a CloudWatch Logs log stream.
type LogStream struct {
	Name               string
	LastEventTimestamp  time.Time
}

// LogEvent represents a single log event.
type LogEvent struct {
	Timestamp time.Time
	Message   string
}

// Client wraps the CloudWatch Logs API.
type Client struct {
	api LogsClient
}

// NewClient creates a new Client with the given AWS profile and region.
// It respects the AWS_ENDPOINT_URL environment variable for custom endpoints (e.g. floci).
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	var cwOpts []func(*cloudwatchlogs.Options)
	if endpoint := endpointURL(); endpoint != "" {
		cwOpts = append(cwOpts, func(o *cloudwatchlogs.Options) {
			o.BaseEndpoint = &endpoint
		})
	}

	return &Client{api: cloudwatchlogs.NewFromConfig(cfg, cwOpts...)}, nil
}

func endpointURL() string {
	if v := os.Getenv("AWS_ENDPOINT_URL"); v != "" {
		return v
	}
	return ""
}

// ListLogGroupsPage returns one page of log groups with the given token.
func (c *Client) ListLogGroupsPage(ctx context.Context, nextToken *string) ([]LogGroup, *string, error) {
	out, err := c.api.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		NextToken: nextToken,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("describing log groups: %w", err)
	}

	groups := make([]LogGroup, 0, len(out.LogGroups))
	for _, g := range out.LogGroups {
		var retention int32
		if g.RetentionInDays != nil {
			retention = *g.RetentionInDays
		}
		var stored int64
		if g.StoredBytes != nil {
			stored = *g.StoredBytes
		}
		groups = append(groups, LogGroup{
			Name:          awssdk.ToString(g.LogGroupName),
			RetentionDays: retention,
			StoredBytes:   stored,
		})
	}
	return groups, out.NextToken, nil
}

// ListLogGroups returns all log groups in the account (first page).
func (c *Client) ListLogGroups(ctx context.Context) ([]LogGroup, error) {
	out, err := c.api.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("describing log groups: %w", err)
	}

	groups := make([]LogGroup, 0, len(out.LogGroups))
	for _, g := range out.LogGroups {
		var retention int32
		if g.RetentionInDays != nil {
			retention = *g.RetentionInDays
		}
		var stored int64
		if g.StoredBytes != nil {
			stored = *g.StoredBytes
		}
		groups = append(groups, LogGroup{
			Name:          awssdk.ToString(g.LogGroupName),
			RetentionDays: retention,
			StoredBytes:   stored,
		})
	}
	return groups, nil
}

// ListLogStreamsPage returns one page of log streams with the given token.
func (c *Client) ListLogStreamsPage(ctx context.Context, logGroupName string, nextToken *string) ([]LogStream, *string, error) {
	out, err := c.api.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: awssdk.String(logGroupName),
		NextToken:    nextToken,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("describing log streams: %w", err)
	}

	streams := make([]LogStream, 0, len(out.LogStreams))
	for _, s := range out.LogStreams {
		var lastEvent time.Time
		if s.LastEventTimestamp != nil {
			lastEvent = time.UnixMilli(*s.LastEventTimestamp)
		}
		streams = append(streams, LogStream{
			Name:              awssdk.ToString(s.LogStreamName),
			LastEventTimestamp: lastEvent,
		})
	}
	return streams, out.NextToken, nil
}

// ListLogStreams returns all log streams for a given log group (first page).
func (c *Client) ListLogStreams(ctx context.Context, logGroupName string) ([]LogStream, error) {
	out, err := c.api.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: awssdk.String(logGroupName),
	})
	if err != nil {
		return nil, fmt.Errorf("describing log streams: %w", err)
	}

	streams := make([]LogStream, 0, len(out.LogStreams))
	for _, s := range out.LogStreams {
		var lastEvent time.Time
		if s.LastEventTimestamp != nil {
			lastEvent = time.UnixMilli(*s.LastEventTimestamp)
		}
		streams = append(streams, LogStream{
			Name:              awssdk.ToString(s.LogStreamName),
			LastEventTimestamp: lastEvent,
		})
	}
	return streams, nil
}

// GetLogEvents returns log events for a given log group and stream within the specified time range.
func (c *Client) GetLogEvents(ctx context.Context, logGroupName, logStreamName string, startTime, endTime time.Time) ([]LogEvent, error) {
	out, err := c.api.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  awssdk.String(logGroupName),
		LogStreamName: awssdk.String(logStreamName),
		StartTime:     awssdk.Int64(startTime.UnixMilli()),
		EndTime:       awssdk.Int64(endTime.UnixMilli()),
		StartFromHead: awssdk.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("getting log events: %w", err)
	}

	events := make([]LogEvent, 0, len(out.Events))
	for _, e := range out.Events {
		var ts time.Time
		if e.Timestamp != nil {
			ts = time.UnixMilli(*e.Timestamp)
		}
		events = append(events, LogEvent{
			Timestamp: ts,
			Message:   awssdk.ToString(e.Message),
		})
	}
	return events, nil
}
