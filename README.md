# lazycwl

[![CI](https://github.com/myuron/lazycwl/actions/workflows/ci.yml/badge.svg)](https://github.com/myuron/lazycwl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/myuron/lazycwl)](https://goreportcard.com/report/github.com/myuron/lazycwl)
[![Go Reference](https://pkg.go.dev/badge/github.com/myuron/lazycwl.svg)](https://pkg.go.dev/github.com/myuron/lazycwl)
[![codecov](https://codecov.io/gh/myuron/lazycwl/branch/main/graph/badge.svg)](https://codecov.io/gh/myuron/lazycwl)
[![Release](https://img.shields.io/github/v/release/myuron/lazycwl)](https://github.com/myuron/lazycwl/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/myuron/lazycwl)](https://github.com/myuron/lazycwl/blob/main/go.mod)
[![GitHub Stars](https://img.shields.io/github/stars/myuron/lazycwl)](https://github.com/myuron/lazycwl/stargazers)
[![GitHub Downloads](https://img.shields.io/github/downloads/myuron/lazycwl/total)](https://github.com/myuron/lazycwl/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A TUI tool for quickly browsing and investigating AWS CloudWatch Logs from the terminal. Browse log groups and streams with yazi-style hierarchical navigation, then open selected logs in `$EDITOR` for Vim-based incident investigation.

日本語版: [README.ja.md](README.ja.md)

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

## Tips

### Saving logs from Vim

When you open logs in `$EDITOR`, they are written to a temporary file that is automatically deleted when the editor exits. To save logs permanently, use Vim's `:w` command to write to a separate file before quitting:

```vim
:w ~/logs/incident-2024-01-15.log
```

You can also filter and extract specific lines before saving — this is the intended workflow for incident investigation.

## Contributing

Bug reports, feature requests, and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, local testing with floci, and the project's TDD workflow.

## License

[MIT](LICENSE)
