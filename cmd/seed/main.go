// Command seed generates large CloudWatch Logs test data into floci.
//
// Usage:
//
//	go run ./cmd/seed --groups 200 --streams 200 --events 10
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

var (
	numGroups  = flag.Int("groups", 200, "number of log groups")
	numStreams = flag.Int("streams", 200, "number of streams per group")
	numEvents  = flag.Int("events", 10, "number of events per stream")
	workers    = flag.Int("workers", 20, "number of concurrent workers")
)

var prefixes = []string{
	"/aws/lambda", "/aws/ecs", "/aws/apigateway", "/aws/rds", "/aws/batch",
	"/app/api", "/app/worker", "/app/frontend", "/infra/monitoring", "/infra/ci",
}

var services = []string{
	"user-service", "order-service", "payment-service", "notification-service",
	"search-service", "auth-service", "inventory-service", "analytics-service",
	"report-generator", "image-processor", "email-sender", "cache-warmer",
	"data-pipeline", "log-aggregator", "health-checker",
}

var envs = []string{"prod", "staging", "dev"}

var retentions = []int32{7, 14, 30, 60, 90, 365}

var messages = []string{
	"INFO Processing request: GET /api/v1/users",
	"INFO Processing request: POST /api/v1/orders",
	"INFO Query executed in 45ms (rows=120)",
	"INFO Cache hit for key: session:abc123",
	"INFO Health check passed",
	"INFO Batch job completed: 500/500 records processed",
	"INFO HTTP response: 200 OK (12ms)",
	"INFO Configuration reloaded",
	"INFO Connection pool stats: active=5 idle=10",
	"DEBUG Entering function: processOrder",
	"DEBUG SQL: SELECT * FROM users WHERE id = 42",
	"DEBUG Redis GET session:xyz -> hit",
	"DEBUG Goroutine count: 150",
	"DEBUG GC pause: 3ms",
	"WARN Slow query detected: 2300ms (threshold: 1000ms)",
	"WARN Connection pool exhausted, waiting for available connection",
	"WARN Retry attempt 2/3 for operation: sendEmail",
	"WARN Memory usage high: 85%",
	"WARN Rate limit approaching: 950/1000 requests",
	"ERROR Failed to connect to database: connection timeout after 30s",
	"ERROR Internal Server Error: null pointer dereference",
	"ERROR Payment declined for order-9001: insufficient funds",
	"ERROR Circuit breaker opened for search-service (failures: 5/5)",
	"ERROR Authentication failed: invalid token",
}

func groupName(idx int) string {
	p := prefixes[idx%len(prefixes)]
	s := services[idx%len(services)]
	e := envs[idx%len(envs)]
	suffix := idx / (len(prefixes) * len(services))
	if suffix > 0 {
		return fmt.Sprintf("%s/%s-%s-%d", p, s, e, suffix)
	}
	return fmt.Sprintf("%s/%s-%s", p, s, e)
}

func streamName(groupIdx, streamIdx int) string {
	p := prefixes[groupIdx%len(prefixes)]
	switch p {
	case "/aws/lambda":
		return fmt.Sprintf("2026/04/14/[$LATEST]%08x_s%d", rand.Int31(), streamIdx)
	case "/aws/ecs":
		return fmt.Sprintf("ecs/task-%06x/%d", rand.Int31n(0xffffff), streamIdx)
	default:
		return fmt.Sprintf("i-%08x-%d", rand.Int31(), streamIdx)
	}
}

func buildEvents(baseTS int64, count int) []types.InputLogEvent {
	events := make([]types.InputLogEvent, count)
	for i := range events {
		events[i] = types.InputLogEvent{
			Timestamp: awssdk.Int64(baseTS + int64(i)*100),
			Message:   awssdk.String(messages[rand.Intn(len(messages))]),
		}
	}
	return events
}

type groupWork struct {
	groupIdx int
	name     string
}

func main() {
	flag.Parse()
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loading AWS config: %v\n", err)
		os.Exit(1)
	}

	var cwOpts []func(*cloudwatchlogs.Options)
	if ep := os.Getenv("AWS_ENDPOINT_URL"); ep != "" {
		cwOpts = append(cwOpts, func(o *cloudwatchlogs.Options) {
			o.BaseEndpoint = &ep
		})
	}
	client := cloudwatchlogs.NewFromConfig(cfg, cwOpts...)

	totalStreams := *numGroups * *numStreams
	totalEvents := totalStreams * *numEvents
	fmt.Printf("=== Seed: %d groups × %d streams × %d events = %d total events ===\n",
		*numGroups, *numStreams, *numEvents, totalEvents)

	// Create all groups first (sequential, fast)
	fmt.Print("Creating groups...")
	for g := 0; g < *numGroups; g++ {
		name := groupName(g)
		_, _ = client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: awssdk.String(name),
		})
		ret := retentions[g%len(retentions)]
		_, _ = client.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
			LogGroupName:    awssdk.String(name),
			RetentionInDays: awssdk.Int32(ret),
		})
	}
	fmt.Printf(" done (%d groups)\n", *numGroups)

	// Create streams + events with worker pool
	var completed atomic.Int64
	work := make(chan groupWork, *numGroups)

	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for gw := range work {
				// Place all events within the last 30 minutes so they fall within lazycwl's default time range (1h).
				// Spread all group×stream combinations evenly across the 30-minute window.
				now := time.Now().UnixMilli()
				thirtyMinMS := int64(30 * 60 * 1000)
				totalCombinations := int64(*numGroups) * int64(*numStreams)
				// Each stream gets a small slice of the 30min window
				intervalPerStream := thirtyMinMS / totalCombinations
				if intervalPerStream < 1 {
					intervalPerStream = 1
				}
				for s := 0; s < *numStreams; s++ {
					sName := streamName(gw.groupIdx, s)
					_, csErr := client.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
						LogGroupName:  awssdk.String(gw.name),
						LogStreamName: awssdk.String(sName),
					})
					if csErr != nil {
						fmt.Fprintf(os.Stderr, "\nCreateLogStream error: group=%s stream=%s: %v\n", gw.name, sName, csErr)
						continue
					}
					if *numEvents > 0 {
						comboIdx := int64(gw.groupIdx)*int64(*numStreams) + int64(s)
						sBaseTS := now - thirtyMinMS + comboIdx*intervalPerStream
						events := buildEvents(sBaseTS, *numEvents)
						// put-log-events max batch is 10000
						for i := 0; i < len(events); i += 10000 {
							end := i + 10000
							if end > len(events) {
								end = len(events)
							}
							resp, putErr := client.PutLogEvents(ctx, &cloudwatchlogs.PutLogEventsInput{
								LogGroupName:  awssdk.String(gw.name),
								LogStreamName: awssdk.String(sName),
								LogEvents:     events[i:end],
							})
							if putErr != nil {
								fmt.Fprintf(os.Stderr, "\nPutLogEvents error: group=%s stream=%s: %v\n", gw.name, sName, putErr)
							} else if resp.RejectedLogEventsInfo != nil {
								fmt.Fprintf(os.Stderr, "\nRejected events: group=%s stream=%s: expired=%d, tooNew=%d, tooOld=%d\n",
									gw.name, sName,
									awssdk.ToInt32(resp.RejectedLogEventsInfo.ExpiredLogEventEndIndex),
									awssdk.ToInt32(resp.RejectedLogEventsInfo.TooNewLogEventStartIndex),
									awssdk.ToInt32(resp.RejectedLogEventsInfo.TooOldLogEventEndIndex))
							}
						}
					}
				}
				done := completed.Add(1)
				fmt.Printf("\r  Progress: %d/%d groups", done, *numGroups)
			}
		}()
	}

	for g := 0; g < *numGroups; g++ {
		work <- groupWork{groupIdx: g, name: groupName(g)}
	}
	close(work)
	wg.Wait()

	fmt.Printf("\n\n=== Done! %d groups, %d streams, %d events ===\n",
		*numGroups, totalStreams, totalEvents)
}
