package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myuron/lazycwl/internal/aws"
	"github.com/myuron/lazycwl/internal/editor"
	"github.com/myuron/lazycwl/internal/formatter"
	"github.com/myuron/lazycwl/internal/tui"
)

func main() {
	var (
		group   string
		stream  string
		since   string
		profile string
		region  string
	)

	flag.StringVar(&group, "group", "", "Log group name")
	flag.StringVar(&stream, "stream", "", "Log stream name (requires --group)")
	flag.StringVar(&since, "since", "1h", "Time range (e.g. 1h, 30m, 7d)")
	flag.StringVar(&profile, "profile", "", "AWS profile")
	flag.StringVar(&region, "region", "", "AWS region")
	flag.Parse()

	if stream != "" && group == "" {
		fmt.Fprintln(os.Stderr, "Error: --stream requires --group")
		os.Exit(1)
	}

	ctx := context.Background()

	client, err := aws.NewClient(ctx, profile, region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sinceDuration, err := parseSince(since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid --since value: %v\n", err)
		os.Exit(1)
	}

	// Direct mode: --group and --stream both specified, skip TUI
	if group != "" && stream != "" {
		if err := directOpen(ctx, client, group, stream, sinceDuration); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	m := tui.NewModelWithOptions(client, tui.Options{
		InitialGroup:  group,
		SinceDuration: sinceDuration,
	})
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func directOpen(ctx context.Context, client *aws.Client, group, stream string, since time.Duration) error {
	now := time.Now()
	events, err := client.GetLogEvents(ctx, group, stream, now.Add(-since), now)
	if err != nil {
		return fmt.Errorf("fetching log events: %w", err)
	}

	content := formatter.Format(events)
	path, cleanup, err := editor.WriteTempFile(content)
	if err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	defer cleanup()

	return editor.Open(path)
}

func parseSince(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid duration number: %w", err)
	}
	if num <= 0 {
		return 0, fmt.Errorf("duration must be positive: %d", num)
	}

	switch unit {
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %c", unit)
	}
}
