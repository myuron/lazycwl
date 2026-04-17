# lazycwl

A TUI tool for quickly browsing and investigating AWS CloudWatch Logs from the terminal. Browse log groups and streams with yazi-style hierarchical navigation, then open selected logs in `$EDITOR` for Vim-based incident investigation.

## Installation

```bash
go install github.com/myuron/lazycwl@latest
```

Or run directly via Nix Flake:

```bash
nix run github:myuron/lazycwl
```

To add to your own flake:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    lazycwl.url = "github:myuron/lazycwl";
  };

  outputs = { nixpkgs, lazycwl, ... }:
    let
      system = "x86_64-linux"; # or "aarch64-linux", "x86_64-darwin", "aarch64-darwin"
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ lazycwl.overlays.default ];
      };
    in {
      # Use pkgs.lazycwl in your packages, devShells, etc.
      devShells.${system}.default = pkgs.mkShell {
        packages = [ pkgs.lazycwl ];
      };
    };
}
```

## Usage

```bash
# Default (uses environment variables or AWS profile credentials)
lazycwl

# Specify profile and region
lazycwl --profile my-profile --region ap-northeast-1

# Jump directly to a log group's streams
lazycwl --group /aws/lambda/my-function

# Open a specific stream directly in $EDITOR (skips TUI)
lazycwl --group /aws/lambda/my-function --stream 'stream-name'
```

## Notes

- Log streams are sorted by last event time (descending) by default. Streams that have never received any log events are **not shown** in this mode because the AWS API excludes them when `OrderBy=LastEventTime` is specified. To see all streams including empty ones, switch to name-based sorting with `s`.

## Key Bindings

### Normal Mode

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `g` | Jump to top of list |
| `G` | Jump to bottom of list |
| `l` / `Enter` / `→` | Go deeper / open logs in editor |
| `h` / `Backspace` / `←` | Go back |
| `/` | Enter incremental search mode |
| `Space` | Toggle multiple stream selection (stream list only) |
| `s` | Toggle sort: last event time ↔ stream name (stream list only) |
| `q` | Quit |

### Search Mode (enter with `/`)

| Key | Action |
|-----|--------|
| Text input | Incrementally filter the list |
| `Enter` | Confirm search and return to normal mode |
| `Escape` | Clear search and return to normal mode |
| `Backspace` | Delete one character from search string |

## Layout

```
┌──────────────────┬────────────────────────────────────┐
│ Log Groups       │ Log Streams                        │
│                  │                                    │
│ /aws/lambda   ←  │ > stream-001  ←                    │
│ /aws/ecs         │   stream-002                       │
│ /app/api         │   stream-003                       │
│                  │                                    │
├──────────────────┴────────────────────────────────────┤
│ Sort: time ↓ | q: quit | /: search | s: sort         │
└───────────────────────────────────────────────────────┘
```

## Local Development (floci)

You can test without an AWS environment using [floci](https://github.com/floci-io/floci).

### Setup

```bash
# 1. Enter the dev environment
nix develop

# 2. Start floci
docker compose up -d

# 3. Seed test data
./scripts/seed-testdata.sh

# 4. Run lazycwl against floci
AWS_ENDPOINT_URL=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
AWS_DEFAULT_REGION=ap-northeast-1 \
go run .
```

### Test Data

The seed script creates the following log groups and streams:

| Log Group | Stream | Content |
|-----------|--------|---------|
| `/aws/lambda/api-handler` | `[$LATEST]abc123` | Normal API request processing |
| `/aws/lambda/api-handler` | `[$LATEST]def456` | DB connection timeout error |
| `/aws/lambda/batch-processor` | `[$LATEST]ghi789` | Batch processing (with slow query warnings) |
| `/aws/ecs/web-service` | `web-service/web/task-001` | Web server startup, includes 500 errors |
| `/app/api/backend` | `i-0abc123def456` | Circuit breaker activation |
| `/app/worker/queue-consumer` | `worker-1` | Queue processing, includes payment errors |

### Large Test Data

For testing pagination and scroll performance, a script for generating large datasets is also available.

```bash
# Default: 50 groups x 20 streams x 100 events = 100,000 events
./scripts/seed-large-testdata.sh

# Customizable with options
./scripts/seed-large-testdata.sh --groups 10 --streams 5 --events 50
./scripts/seed-large-testdata.sh --groups 100 --streams 50 --events 500
```

| Option | Default | Description |
|--------|---------|-------------|
| `--groups` | 50 | Number of log groups to create |
| `--streams` | 20 | Number of streams per group |
| `--events` | 100 | Number of events per stream |

### Stopping floci

```bash
docker compose down
```

## Tech Stack

- [Go](https://go.dev/)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Styling
- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2) — CloudWatch Logs API
- [Nix Flake](https://nixos.wiki/wiki/Flakes) — Build management
