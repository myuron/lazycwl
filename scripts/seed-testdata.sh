#!/usr/bin/env bash
# Seed CloudWatch Logs test data into floci (localhost:4566)
set -euo pipefail

export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=ap-northeast-1

echo "Creating log groups..."

groups=(
  "/aws/lambda/api-handler"
  "/aws/lambda/batch-processor"
  "/aws/ecs/web-service"
  "/app/api/backend"
  "/app/worker/queue-consumer"
)

for group in "${groups[@]}"; do
  aws logs create-log-group --log-group-name "$group" 2>/dev/null || true
  aws logs put-retention-policy --log-group-name "$group" --retention-in-days 30
  echo "  Created: $group"
done

echo ""
echo "Creating log streams and events..."

now_ms=$(date +%s)000

create_stream_with_events() {
  local group=$1
  local stream=$2
  shift 2
  local messages=("$@")

  aws logs create-log-stream \
    --log-group-name "$group" \
    --log-stream-name "$stream" 2>/dev/null || true

  local events="["
  local ts=$now_ms
  local first=true
  for msg in "${messages[@]}"; do
    if [ "$first" = true ]; then
      first=false
    else
      events+=","
    fi
    events+="{\"timestamp\":$ts,\"message\":\"$msg\"}"
    ts=$((ts + 1000))
  done
  events+="]"

  aws logs put-log-events \
    --log-group-name "$group" \
    --log-stream-name "$stream" \
    --log-events "$events" > /dev/null

  echo "  $group / $stream (${#messages[@]} events)"
}

create_stream_with_events "/aws/lambda/api-handler" "2026/04/13/[\$LATEST]abc123" \
  "START RequestId: abc-123" \
  "INFO Loading configuration..." \
  "INFO Processing request: GET /api/users" \
  "INFO Query executed in 45ms" \
  "END RequestId: abc-123" \
  "REPORT RequestId: abc-123 Duration: 120.5 ms Memory: 128 MB"

create_stream_with_events "/aws/lambda/api-handler" "2026/04/13/[\$LATEST]def456" \
  "START RequestId: def-456" \
  "INFO Processing request: POST /api/orders" \
  "ERROR Failed to connect to database: connection timeout" \
  "END RequestId: def-456" \
  "REPORT RequestId: def-456 Duration: 30000.0 ms Memory: 128 MB"

create_stream_with_events "/aws/lambda/batch-processor" "2026/04/13/[\$LATEST]ghi789" \
  "START RequestId: ghi-789" \
  "INFO Batch job started: process-daily-reports" \
  "INFO Processing 1500 records..." \
  "WARN Slow query detected: 2300ms" \
  "INFO Batch completed: 1500/1500 records" \
  "END RequestId: ghi-789"

create_stream_with_events "/aws/ecs/web-service" "web-service/web/task-001" \
  "Starting application on port 8080" \
  "Connected to database: postgres://db:5432/app" \
  "Health check endpoint ready" \
  "GET /health 200 2ms" \
  "POST /api/webhook 500 Internal Server Error" \
  "GET /health 200 1ms"

create_stream_with_events "/app/api/backend" "i-0abc123def456" \
  "[2026-04-13T10:00:00Z] Server started on :3000" \
  "[2026-04-13T10:00:01Z] Redis connection established" \
  "[2026-04-13T10:05:30Z] Request timeout: /api/search (client_id=user-42)" \
  "[2026-04-13T10:05:31Z] Circuit breaker opened for search-service" \
  "[2026-04-13T10:10:31Z] Circuit breaker closed for search-service"

create_stream_with_events "/app/worker/queue-consumer" "worker-1" \
  "Listening on queue: orders-processing" \
  "Received message: order-9001" \
  "Processing order-9001: payment validation" \
  "Order order-9001 completed successfully" \
  "Received message: order-9002" \
  "Processing order-9002: payment validation" \
  "ERROR Payment declined for order-9002"

echo ""
echo "Done! Test data seeded successfully."
echo "Run lazycwl with: AWS_ENDPOINT_URL=http://localhost:4566 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_DEFAULT_REGION=ap-northeast-1 ./lazycwl"
