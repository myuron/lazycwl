package main

import (
	"context"
	"flag"
	"fmt"
	"os"

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
		profile string
		region  string
	)

	flag.StringVar(&group, "group", "", "Log group name")
	flag.StringVar(&stream, "stream", "", "Log stream name (requires --group)")
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

	// Direct mode: --group and --stream both specified, skip TUI
	if group != "" && stream != "" {
		if err := directOpen(ctx, client, group, stream); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	m := tui.NewModelWithOptions(client, tui.Options{
		InitialGroup: group,
	})
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func directOpen(ctx context.Context, client *aws.Client, group, stream string) error {
	events, err := client.GetLogEvents(ctx, group, stream)
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
