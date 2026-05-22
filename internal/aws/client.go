package aws

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// maxConcurrent caps the number of in-flight GetLogEvents API calls across all
// streams and time chunks. Tuned to stay well under the CloudWatch Logs
// 25 TPS account-level limit while still saturating typical fetches.
const maxConcurrent = 8

// timeChunks is the number of equal time slices each stream's [first, last]
// range is split into for parallel GetLogEvents fetches.
const timeChunks = 8

// minChunkDurationMs is the smallest time slice we will create. If a stream's
// range is shorter than timeChunks * minChunkDurationMs, fewer chunks are
// used (chunking a 10-second stream into 8 slices is pointless overhead).
const minChunkDurationMs = 1000

// defaultPageSize is the default number of items to fetch per API page.
const defaultPageSize = 50

// LogsClient is the interface for CloudWatch Logs API operations.
type LogsClient interface {
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
	StartLiveTail(ctx context.Context, params *cloudwatchlogs.StartLiveTailInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartLiveTailOutput, error)
}

// LogGroup represents a CloudWatch Logs log group.
type LogGroup struct {
	Name          string
	ARN           string
	RetentionDays int32
	StoredBytes   int64
}

// LogStream represents a CloudWatch Logs log stream.
type LogStream struct {
	Name                string
	FirstEventTimestamp time.Time
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
		groups = append(groups, toLogGroup(g))
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
		groups = append(groups, toLogGroup(g))
	}
	return groups, nil
}

func toLogGroup(g types.LogGroup) LogGroup {
	var retention int32
	if g.RetentionInDays != nil {
		retention = *g.RetentionInDays
	}
	var stored int64
	if g.StoredBytes != nil {
		stored = *g.StoredBytes
	}
	// Prefer Arn (without trailing :*), fall back to LogGroupArn (strip :*).
	arn := awssdk.ToString(g.Arn)
	if arn == "" {
		arn = strings.TrimSuffix(awssdk.ToString(g.LogGroupArn), ":*")
	}
	return LogGroup{
		Name:          awssdk.ToString(g.LogGroupName),
		ARN:           arn,
		RetentionDays: retention,
		StoredBytes:   stored,
	}
}

// StartLiveTailSession starts a live tail streaming session for the given
// log group ARN and optional stream names. The caller is responsible for
// reading from the returned event stream and closing it.
func (c *Client) StartLiveTailSession(ctx context.Context, logGroupARN string, streamNames []string) (*cloudwatchlogs.StartLiveTailOutput, error) {
	input := &cloudwatchlogs.StartLiveTailInput{
		LogGroupIdentifiers: []string{logGroupARN},
	}
	if len(streamNames) > 0 {
		input.LogStreamNames = streamNames
	}
	out, err := c.api.StartLiveTail(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("starting live tail: %w", err)
	}
	return out, nil
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
		streams = append(streams, toLogStream(s))
	}
	return streams, out.NextToken, nil
}

func toLogStream(s types.LogStream) LogStream {
	var first, last time.Time
	if s.FirstEventTimestamp != nil {
		first = time.UnixMilli(*s.FirstEventTimestamp)
	}
	if s.LastEventTimestamp != nil {
		last = time.UnixMilli(*s.LastEventTimestamp)
	}
	return LogStream{
		Name:                awssdk.ToString(s.LogStreamName),
		FirstEventTimestamp: first,
		LastEventTimestamp:  last,
	}
}

// DescribeLogStream returns metadata (including event time range) for a single
// log stream. The lookup uses LogStreamNamePrefix and filters for an exact
// name match, since CloudWatch Logs has no single-stream describe API.
func (c *Client) DescribeLogStream(ctx context.Context, logGroupName, logStreamName string) (LogStream, error) {
	out, err := c.api.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        awssdk.String(logGroupName),
		LogStreamNamePrefix: awssdk.String(logStreamName),
		Limit:               awssdk.Int32(defaultPageSize),
	})
	if err != nil {
		return LogStream{}, fmt.Errorf("describing log stream %q: %w", logStreamName, err)
	}
	for _, s := range out.LogStreams {
		if awssdk.ToString(s.LogStreamName) == logStreamName {
			return toLogStream(s), nil
		}
	}
	return LogStream{}, fmt.Errorf("log stream %q not found in group %q", logStreamName, logGroupName)
}

// ListLogStreams returns log streams for a given log group (first page, descending).
func (c *Client) ListLogStreams(ctx context.Context, logGroupName string) ([]LogStream, error) {
	streams, _, err := c.ListLogStreamsPage(ctx, logGroupName, nil, true)
	return streams, err
}

// GetLogEvents returns all log events for a given log group and stream,
// handling NextForwardToken pagination automatically.
func (c *Client) GetLogEvents(ctx context.Context, logGroupName, logStreamName string) ([]LogEvent, error) {
	return c.paginateGetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  awssdk.String(logGroupName),
		LogStreamName: awssdk.String(logStreamName),
		StartFromHead: awssdk.Bool(true),
	})
}

// GetLogEventsByTimeRange paginates events for a single stream restricted to
// [startTimeMs, endTimeMs). StartTime is inclusive, EndTime is exclusive —
// callers can split a stream's range into adjacent windows without overlap
// or gaps and fetch the windows in parallel.
func (c *Client) GetLogEventsByTimeRange(ctx context.Context, logGroupName, logStreamName string, startTimeMs, endTimeMs int64) ([]LogEvent, error) {
	return c.paginateGetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  awssdk.String(logGroupName),
		LogStreamName: awssdk.String(logStreamName),
		StartFromHead: awssdk.Bool(true),
		StartTime:     awssdk.Int64(startTimeMs),
		EndTime:       awssdk.Int64(endTimeMs),
	})
}

func (c *Client) paginateGetLogEvents(ctx context.Context, input *cloudwatchlogs.GetLogEventsInput) ([]LogEvent, error) {
	var allEvents []LogEvent
	var prevToken string

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
		// NextForwardToken as the previous call, or an empty page.
		nextToken := awssdk.ToString(out.NextForwardToken)
		if nextToken == "" || nextToken == prevToken || len(out.Events) == 0 {
			break
		}
		prevToken = nextToken
		input.NextToken = out.NextForwardToken
	}

	return allEvents, nil
}

// GetMultiStreamLogEvents fetches log events from one or more streams. For
// each stream it first describes the stream to learn its event time range,
// then splits that range into time chunks and issues parallel GetLogEvents
// calls — turning a long serial pagination chain into mostly-parallel work.
// All API calls share one semaphore (maxConcurrent) to stay under
// CloudWatch Logs throttling limits.
func (c *Client) GetMultiStreamLogEvents(ctx context.Context, logGroupName string, streamNames []string) ([]LogEvent, error) {
	sem := make(chan struct{}, maxConcurrent)
	results := make([][]LogEvent, len(streamNames))
	var mu sync.Mutex
	var firstErr error
	recordErr := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for i, name := range streamNames {
		wg.Add(1)
		go func(idx int, streamName string) {
			defer wg.Done()
			events, err := c.fetchStreamEvents(ctx, logGroupName, streamName, sem)
			if err != nil {
				recordErr(err)
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

// fetchStreamEvents pulls every event for one stream, preferring the
// time-chunked parallel path when the stream's range is known. When the
// describe call fails or the range is missing/degenerate, it falls back to a
// single sequential GetLogEvents that still benefits from the global
// semaphore.
func (c *Client) fetchStreamEvents(ctx context.Context, logGroupName, logStreamName string, sem chan struct{}) ([]LogEvent, error) {
	stream, err := c.DescribeLogStream(ctx, logGroupName, logStreamName)
	if err != nil || stream.FirstEventTimestamp.IsZero() || stream.LastEventTimestamp.IsZero() {
		return c.fetchAllSequential(ctx, logGroupName, logStreamName, sem)
	}
	first := stream.FirstEventTimestamp.UnixMilli()
	last := stream.LastEventTimestamp.UnixMilli()
	if last < first {
		return c.fetchAllSequential(ctx, logGroupName, logStreamName, sem)
	}
	return c.fetchChunked(ctx, logGroupName, logStreamName, first, last, sem)
}

func (c *Client) fetchAllSequential(ctx context.Context, logGroupName, logStreamName string, sem chan struct{}) ([]LogEvent, error) {
	if err := acquire(ctx, sem); err != nil {
		return nil, err
	}
	defer func() { <-sem }()
	return c.GetLogEvents(ctx, logGroupName, logStreamName)
}

// fetchChunked splits [firstMs, lastMs] into equal time windows and fetches
// them in parallel. Each chunk does its own NextForwardToken pagination
// internally; the global semaphore bounds total API parallelism.
func (c *Client) fetchChunked(ctx context.Context, logGroupName, logStreamName string, firstMs, lastMs int64, sem chan struct{}) ([]LogEvent, error) {
	chunks := planTimeChunks(firstMs, lastMs)
	if len(chunks) == 1 {
		if err := acquire(ctx, sem); err != nil {
			return nil, err
		}
		defer func() { <-sem }()
		return c.GetLogEventsByTimeRange(ctx, logGroupName, logStreamName, chunks[0][0], chunks[0][1])
	}

	results := make([][]LogEvent, len(chunks))
	var mu sync.Mutex
	var firstErr error
	recordErr := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for i, window := range chunks {
		wg.Add(1)
		go func(idx int, start, end int64) {
			defer wg.Done()
			if err := acquire(ctx, sem); err != nil {
				recordErr(err)
				return
			}
			defer func() { <-sem }()
			events, err := c.GetLogEventsByTimeRange(ctx, logGroupName, logStreamName, start, end)
			if err != nil {
				recordErr(err)
				return
			}
			results[idx] = events
		}(i, window[0], window[1])
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Chunks are time-ordered and non-overlapping, and events within each
	// chunk arrive oldest-first (StartFromHead=true), so plain concatenation
	// yields a timestamp-sorted result.
	var all []LogEvent
	for _, r := range results {
		all = append(all, r...)
	}
	return all, nil
}

// planTimeChunks returns adjacent [start, end) windows covering [firstMs, lastMs].
// The final window's end is lastMs+1 because GetLogEvents.EndTime is exclusive.
// Streams shorter than timeChunks * minChunkDurationMs use fewer chunks so we
// don't pay describe + scatter overhead for already-tiny streams.
func planTimeChunks(firstMs, lastMs int64) [][2]int64 {
	totalMs := lastMs - firstMs + 1
	chunks := timeChunks
	if minSpan := int64(chunks) * minChunkDurationMs; totalMs < minSpan {
		chunks = int(totalMs / minChunkDurationMs)
		if chunks < 1 {
			chunks = 1
		}
	}
	windows := make([][2]int64, 0, chunks)
	span := totalMs / int64(chunks)
	for i := 0; i < chunks; i++ {
		start := firstMs + int64(i)*span
		end := start + span
		if i == chunks-1 {
			end = lastMs + 1
		}
		windows = append(windows, [2]int64{start, end})
	}
	return windows
}

func acquire(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
