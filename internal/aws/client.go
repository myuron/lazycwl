package aws

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// maxConcurrentStreams limits the number of concurrent GetLogEvents API calls.
const maxConcurrentStreams = 5

// defaultPageSize is the default number of items to fetch per API page.
const defaultPageSize = 50

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
	StoredBytes        int64
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
	return os.Getenv("AWS_ENDPOINT_URL")
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
// descending controls the sort order by last event time.
func (c *Client) ListLogStreamsPage(ctx context.Context, logGroupName string, nextToken *string, descending bool) ([]LogStream, *string, error) {
	out, err := c.api.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: awssdk.String(logGroupName),
		NextToken:    nextToken,
		OrderBy:      types.OrderByLastEventTime,
		Descending:   awssdk.Bool(descending),
		Limit:        awssdk.Int32(defaultPageSize),
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
		var stored int64
		if s.StoredBytes != nil {
			stored = *s.StoredBytes
		}
		streams = append(streams, LogStream{
			Name:              awssdk.ToString(s.LogStreamName),
			LastEventTimestamp: lastEvent,
			StoredBytes:       stored,
		})
	}
	return streams, out.NextToken, nil
}

// ListLogStreams returns log streams for a given log group (first page, descending).
func (c *Client) ListLogStreams(ctx context.Context, logGroupName string) ([]LogStream, error) {
	streams, _, err := c.ListLogStreamsPage(ctx, logGroupName, nil, true)
	return streams, err
}

// GetLogEvents returns all log events for a given log group and stream,
// handling pagination automatically.
func (c *Client) GetLogEvents(ctx context.Context, logGroupName, logStreamName string) ([]LogEvent, error) {
	var allEvents []LogEvent
	var prevToken string

	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  awssdk.String(logGroupName),
		LogStreamName: awssdk.String(logStreamName),
		StartFromHead: awssdk.Bool(true),
	}

	for {
		out, err := c.api.GetLogEvents(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("getting log events: %w", err)
		}

		for _, e := range out.Events {
			var ts time.Time
			if e.Timestamp != nil {
				ts = time.UnixMilli(*e.Timestamp)
			}
			allEvents = append(allEvents, LogEvent{
				Timestamp: ts,
				Message:   awssdk.ToString(e.Message),
			})
		}

		// GetLogEvents signals end-of-stream by returning the same
		// NextForwardToken as the previous call, or an empty page with
		// no new token.
		nextToken := awssdk.ToString(out.NextForwardToken)
		if nextToken == "" || nextToken == prevToken || len(out.Events) == 0 {
			break
		}
		prevToken = nextToken
		input.NextToken = out.NextForwardToken
	}

	return allEvents, nil
}

// GetMultiStreamLogEvents fetches log events from multiple streams concurrently,
// limiting parallelism to maxConcurrentStreams.
func (c *Client) GetMultiStreamLogEvents(ctx context.Context, logGroupName string, streamNames []string) ([]LogEvent, error) {
	results := make([][]LogEvent, len(streamNames))
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrentStreams)

	for i, name := range streamNames {
		wg.Add(1)
		go func(idx int, streamName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			events, err := c.GetLogEvents(ctx, logGroupName, streamName)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			results[idx] = events
		}(i, name)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	var allEvents []LogEvent
	for _, events := range results {
		allEvents = append(allEvents, events...)
	}
	return allEvents, nil
}
