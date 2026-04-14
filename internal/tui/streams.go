package tui

import (
	"strings"

	"github.com/myuron/lazycwl/internal/aws"
)

func (m Model) filteredStreams() []aws.LogStream {
	if m.searchQuery == "" {
		return m.logStreams
	}
	q := strings.ToLower(m.searchQuery)
	var result []aws.LogStream
	for _, s := range m.logStreams {
		if strings.Contains(strings.ToLower(s.Name), q) {
			result = append(result, s)
		}
	}
	return result
}
