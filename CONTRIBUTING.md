# Contributing to lazycwl

Thank you for your interest in contributing to lazycwl!

## Development Setup

### Prerequisites

- [Nix](https://nixos.org/download.html) (with Flakes enabled)
- [Docker](https://docs.docker.com/get-docker/) / Docker Compose

### Getting Started

```bash
# 1. Clone the repository
git clone https://github.com/myuron/lazycwl.git
cd lazycwl

# 2. Enter the development environment (provides Go and other tools)
nix develop

# 3. Install dependencies
go mod tidy

# 4. Run tests to verify your setup
go test ./...
```

### Local Testing (No AWS Required)

You can test locally using floci. See the [Local Development section in README.md](README.md#local-development-floci) for details.

## Development Workflow

### 1. Check Issues

Start by checking [Issues](https://github.com/myuron/lazycwl/issues). If you'd like to work on an issue, leave a comment to let us know. For new feature proposals, please create an issue first for discussion.

### 2. Create a Branch

```bash
git checkout -b <type>/<short-description>
```

Branch name examples:
- `feat/preview-pane`
- `fix/cursor-reset-on-paginate`
- `docs/keybind-table`

### 3. Develop with TDD

This project follows TDD (Test-Driven Development).

1. **Red**: Write a failing test first
2. **Green**: Write the minimal code to make the test pass
3. **Refactor**: Clean up the code (keeping tests passing)

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/tui/...
```

### 4. Code Style

- Format with `gofmt` / `goimports`
- Write code comments in English
- Wrap errors with `fmt.Errorf("doing X: %w", err)` and return them to the caller
- Follow Bubble Tea's Model / Update / View pattern
- Never perform I/O directly in `Update` — use `tea.Cmd` for side effects

### 5. Commit

Follow [Conventional Commits](https://www.conventionalcommits.org/). Commit messages are written in Japanese (with English prefixes).

```
feat: ロググループ一覧表示を実装
fix: ページネーション時のカーソル位置リセットを修正
test: AWSクライアントのモックテストを追加
refactor: TUIモデルの状態管理を整理
docs: READMEにキーバインド表を追加
```

### 6. Create a Pull Request

- Use the Conventional Commits format for the PR title
- Fill in the description following the template
- Make sure all tests pass

## Project Structure

```
lazycwl/
├── main.go                   # Entry point
├── internal/
│   ├── aws/                  # AWS CloudWatch Logs API client
│   ├── tui/                  # Bubble Tea TUI layer
│   ├── editor/               # $EDITOR integration
│   └── formatter/            # Log event formatting
├── docs/
│   └── requirements.md       # Requirements specification
└── CLAUDE.md                 # Development guidelines for Claude Code
```

See `CLAUDE.md` and `docs/requirements.md` for implementation details.

## Important Notes

- Do not implement features outside the scope of `docs/requirements.md`
- Code without tests will not be merged
- Do not place implementation code outside of `internal/`
