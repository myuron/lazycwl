package formatter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/myuron/lazycwl/internal/aws"
)

// Format formats log events as "[ISO8601] message" lines, sorted by timestamp.
func Format(events []aws.LogEvent) string {
	if len(events) == 0 {
		return ""
	}

	sorted := make([]aws.LogEvent, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	var b strings.Builder
	for _, e := range sorted {
		fmt.Fprintf(&b, "[%s] %s\n", e.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z"), e.Message)
	}
	return b.String()
}
