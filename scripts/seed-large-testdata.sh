#!/usr/bin/env bash
# Seed large CloudWatch Logs test data into floci (localhost:4566)
# for testing pagination, scrolling, and performance.
#
# Usage:
#   bash scripts/seed-large-testdata.sh [--groups N] [--streams N] [--events N]
#
# Defaults: 50 groups, 20 streams per group, 100 events per stream
set -euo pipefail

export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=ap-northeast-1

# --- Configuration ---
NUM_GROUPS=50
NUM_STREAMS=20
EVENTS_PER_STREAM=100

while [[ $# -gt 0 ]]; do
  case "$1" in
    --groups)  NUM_GROUPS="$2";        shift 2 ;;
    --streams) NUM_STREAMS="$2";       shift 2 ;;
    --events)  EVENTS_PER_STREAM="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

echo "=== Large Test Data Seeder ==="
echo "Groups: $NUM_GROUPS, Streams/group: $NUM_STREAMS, Events/stream: $EVENTS_PER_STREAM"
echo "Total events: $((NUM_GROUPS * NUM_STREAMS * EVENTS_PER_STREAM))"
echo ""

# --- Log group name templates ---
prefixes=(
  "/aws/lambda"
  "/aws/ecs"
  "/aws/apigateway"
  "/aws/rds"
  "/aws/batch"
  "/app/api"
  "/app/worker"
  "/app/frontend"
  "/infra/monitoring"
  "/infra/ci"
)

services=(
  "user-service"
  "order-service"
  "payment-service"
  "notification-service"
  "search-service"
  "auth-service"
  "inventory-service"
  "analytics-service"
  "report-generator"
  "image-processor"
  "email-sender"
  "cache-warmer"
  "data-pipeline"
  "log-aggregator"
  "health-checker"
)

environments=("prod" "staging" "dev")

# --- Log message templates ---
log_levels=("INFO" "WARN" "ERROR" "DEBUG")

info_messages=(
  "Processing request: GET /api/v1/users"
  "Processing request: POST /api/v1/orders"
  "Processing request: PUT /api/v1/items/{id}"
  "Processing request: DELETE /api/v1/sessions/{id}"
  "Query executed in %dms (rows=%d)"
  "Cache hit for key: session:%s"
  "Cache miss for key: user:%s"
  "Successfully connected to database"
  "Health check passed"
  "Batch job completed: %d/%d records processed"
  "Message consumed from queue: %s"
  "HTTP response: %d %s (%dms)"
  "Scaling up to %d instances"
  "Configuration reloaded"
  "Connection pool stats: active=%d idle=%d"
  "Request completed successfully (trace_id=%s)"
  "Deploying version %s"
  "Feature flag evaluated: %s = %s"
  "Background job started: %s"
  "Metrics flushed: %d datapoints"
)

warn_messages=(
  "Slow query detected: %dms (threshold: 1000ms)"
  "Connection pool exhausted, waiting for available connection"
  "Retry attempt %d/3 for operation: %s"
  "Rate limit approaching: %d/%d requests"
  "Memory usage high: %d%%"
  "Disk usage warning: %d%% used"
  "Certificate expires in %d days"
  "Deprecated API version used by client: %s"
  "Request timeout approaching: %dms / 30000ms"
  "Queue depth exceeding threshold: %d messages"
)

error_messages=(
  "Failed to connect to database: connection timeout after 30s"
  "Internal Server Error: null pointer dereference"
  "Payment declined for order-%d: insufficient funds"
  "Circuit breaker opened for %s (failures: %d/5)"
  "Authentication failed: invalid token"
  "Failed to send notification: SMTP connection refused"
  "Out of memory: requested %dMB, available %dMB"
  "Unhandled exception: index out of range [%d]"
  "Failed to deserialize message: invalid JSON at offset %d"
  "Connection reset by peer: %s:%d"
)

debug_messages=(
  "Entering function: processOrder"
  "SQL: SELECT * FROM users WHERE id = %d"
  "Redis GET session:%s -> hit"
  "HTTP headers: Content-Type=application/json"
  "Goroutine count: %d"
  "GC pause: %dms"
  "Request body size: %d bytes"
  "Response serialized in %dms"
)

# --- Helper functions ---
rand_int() {
  local min=$1 max=$2
  echo $(( RANDOM % (max - min + 1) + min ))
}

rand_element() {
  local arr=("$@")
  echo "${arr[RANDOM % ${#arr[@]}]}"
}

rand_hex() {
  local len=$1
  cat /dev/urandom | LC_ALL=C tr -dc 'a-f0-9' | head -c "$len"
}

generate_log_message() {
  local level
  local r=$(( RANDOM % 100 ))
  if   (( r < 60 )); then level="INFO"
  elif (( r < 80 )); then level="DEBUG"
  elif (( r < 93 )); then level="WARN"
  else                     level="ERROR"
  fi

  local msg
  case "$level" in
    INFO)
      msg=$(rand_element "${info_messages[@]}")
      msg=$(echo "$msg" | sed \
        -e "s/%d/$(rand_int 1 9999)/g" \
        -e "s/%s/$(rand_hex 8)/g")
      ;;
    WARN)
      msg=$(rand_element "${warn_messages[@]}")
      msg=$(echo "$msg" | sed \
        -e "s/%d/$(rand_int 1 9999)/g" \
        -e "s/%s/$(rand_hex 8)/g")
      ;;
    ERROR)
      msg=$(rand_element "${error_messages[@]}")
      msg=$(echo "$msg" | sed \
        -e "s/%d/$(rand_int 1 9999)/g" \
        -e "s/%s/$(rand_hex 8)/g")
      ;;
    DEBUG)
      msg=$(rand_element "${debug_messages[@]}")
      msg=$(echo "$msg" | sed \
        -e "s/%d/$(rand_int 1 9999)/g" \
        -e "s/%s/$(rand_hex 8)/g")
      ;;
  esac

  echo "$level $msg"
}

generate_group_name() {
  local idx=$1
  local prefix_idx=$(( idx % ${#prefixes[@]} ))
  local service_idx=$(( idx % ${#services[@]} ))
  local env_idx=$(( idx % ${#environments[@]} ))
  local prefix="${prefixes[$prefix_idx]}"
  local service="${services[$service_idx]}"
  local env="${environments[$env_idx]}"

  # Add numeric suffix to avoid duplicates
  local suffix=$(( idx / (${#prefixes[@]} * ${#services[@]}) ))
  if (( suffix > 0 )); then
    echo "${prefix}/${service}-${env}-${suffix}"
  else
    echo "${prefix}/${service}-${env}"
  fi
}

generate_stream_name() {
  local group_idx=$1
  local stream_idx=$2
  local prefix="${prefixes[$(( group_idx % ${#prefixes[@]} ))]}"

  case "$prefix" in
    /aws/lambda*)
      echo "2026/04/14/[\$LATEST]$(rand_hex 12)_stream${stream_idx}"
      ;;
    /aws/ecs*)
      echo "ecs/task-$(rand_hex 6)/${stream_idx}"
      ;;
    *)
      echo "i-$(rand_hex 8)-${stream_idx}"
      ;;
  esac
}

# --- put-log-events in batches (API limit: max 10000 events per call) ---
MAX_BATCH=500

put_events_batched() {
  local group=$1
  local stream=$2
  local base_ts=$3
  local count=$4

  local remaining=$count
  local offset=0

  while (( remaining > 0 )); do
    local batch_size=$(( remaining > MAX_BATCH ? MAX_BATCH : remaining ))
    local events="["
    local first=true

    for (( i = 0; i < batch_size; i++ )); do
      local ts=$(( base_ts + (offset + i) * 100 ))
      local msg
      msg=$(generate_log_message)
      # Escape special characters for JSON
      msg=$(echo "$msg" | sed 's/\\/\\\\/g; s/"/\\"/g')

      if [ "$first" = true ]; then
        first=false
      else
        events+=","
      fi
      events+="{\"timestamp\":$ts,\"message\":\"$msg\"}"
    done
    events+="]"

    aws logs put-log-events \
      --log-group-name "$group" \
      --log-stream-name "$stream" \
      --log-events "$events" > /dev/null

    offset=$(( offset + batch_size ))
    remaining=$(( remaining - batch_size ))
  done
}

# --- Main ---
echo "Creating $NUM_GROUPS log groups..."

base_ts=$(date +%s)000
created=0

for (( g = 0; g < NUM_GROUPS; g++ )); do
  group_name=$(generate_group_name "$g")
  retention=$(rand_element 7 14 30 60 90 365)

  aws logs create-log-group --log-group-name "$group_name" 2>/dev/null || true
  aws logs put-retention-policy --log-group-name "$group_name" --retention-in-days "$retention"

  for (( s = 0; s < NUM_STREAMS; s++ )); do
    stream_name=$(generate_stream_name "$g" "$s")
    aws logs create-log-stream \
      --log-group-name "$group_name" \
      --log-stream-name "$stream_name" 2>/dev/null || true

    # Stagger timestamps so streams have different lastEventTimestamp
    stream_base_ts=$(( base_ts - (g * NUM_STREAMS + s) * EVENTS_PER_STREAM * 100 ))
    put_events_batched "$group_name" "$stream_name" "$stream_base_ts" "$EVENTS_PER_STREAM"
  done

  created=$(( created + 1 ))
  printf "\r  Progress: %d/%d groups (%d streams, %d events each)" \
    "$created" "$NUM_GROUPS" "$NUM_STREAMS" "$EVENTS_PER_STREAM"
done

echo ""
echo ""
echo "=== Done! ==="
echo "Created: $NUM_GROUPS groups × $NUM_STREAMS streams × $EVENTS_PER_STREAM events"
echo "Total:   $((NUM_GROUPS * NUM_STREAMS)) streams, $((NUM_GROUPS * NUM_STREAMS * EVENTS_PER_STREAM)) events"
echo ""
echo "Run lazycwl with:"
echo "  AWS_ENDPOINT_URL=http://localhost:4566 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_DEFAULT_REGION=ap-northeast-1 ./lazycwl"
