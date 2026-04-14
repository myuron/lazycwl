# lazycwl — Claude Code ハーネス

## プロジェクト概要

lazycwl（Lazy CloudWatch Logs）は、AWS CloudWatch Logsをターミナルで素早く閲覧・調査するためのTUIツール。yazi風の階層ナビゲーションでロググループ/ストリームをブラウズし、選択したログを$EDITORで開いてVimベースの障害調査を行う。

## 要件定義書

実装時は必ず `docs/requirements.md` を参照すること。全ての機能実装はこの要件定義書に基づく。要件定義書に記載のない機能は実装しない。

## 全体方針

**TDD（テスト駆動開発）で開発する。** 全ての機能について以下のサイクルを遵守すること:

1. Red: 失敗するテストを先に書く
2. Green: テストを通す最小限のコードを書く
3. Refactor: コードを整理する（テストが通る状態を維持）

テストなしのコードをコミットしない。

**進捗管理は `todo.md` で行う。** 作業開始前・作業中・作業完了時に必ず `todo.md` を更新すること:

- 作業予定: これから着手するタスクを記載する
- 作業実績: 完了したタスクにチェックを入れ、実施日を記録する
- 進捗状況が常に可視化された状態を維持する

```markdown
# todo.md の例
## Phase 1: 基盤
- [x] 2026-04-13 go mod init
- [x] 2026-04-13 AWSクライアントインターフェース定義 + モックテスト
- [ ] AWSクライアント実装

## Phase 2: コアTUI
- [ ] ロググループ一覧表示のテスト作成
- [ ] ロググループ一覧表示の実装
```

## 技術スタック

| 項目 | 技術 |
|------|------|
| 実装言語 | Go |
| TUIフレームワーク | [Bubble Tea](https://github.com/charmbracelet/bubbletea) |
| TUIコンポーネント | [Bubbles](https://github.com/charmbracelet/bubbles) |
| TUIスタイリング | [Lip Gloss](https://github.com/charmbracelet/lipgloss) |
| AWS SDK | [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2) |
| CLIフラグ解析 | [cobra](https://github.com/spf13/cobra) または標準 `flag` パッケージ |
| ビルド管理 | Nix Flake |

## プロジェクト構成

```
lazycwl/
├── main.go                   # エントリーポイント
├── internal/
│   ├── aws/                  # AWS CloudWatch Logs APIクライアント
│   │   ├── client.go
│   │   └── client_test.go
│   ├── tui/                  # Bubble Tea TUIレイヤー
│   │   ├── model.go          # ルートModel
│   │   ├── model_test.go
│   │   ├── groups.go         # ロググループ一覧ビュー
│   │   ├── streams.go        # ログストリーム一覧ビュー
│   │   ├── preview.go        # プレビューペイン
│   │   └── keys.go           # キーバインド定義
│   ├── editor/               # $EDITOR連携
│   │   ├── editor.go
│   │   └── editor_test.go
│   └── formatter/            # ログイベントのフォーマット
│       ├── formatter.go
│       └── formatter_test.go
├── docs/
│   └── requirements.md       # 要件定義書
├── flake.nix
├── flake.lock
├── go.mod
├── go.sum
└── CLAUDE.md
```

## コーディング規約

- `gofmt` / `goimports` でフォーマットする
- 命名: Go標準の命名規則に従う（CamelCase、短い変数名、パッケージ名は小文字単語）
- エラーハンドリング: エラーは呼び出し元に返す。握りつぶさない。`fmt.Errorf("doing X: %w", err)` でラップする
- パッケージ間の依存方向: `cmd` → `internal/*`。`internal` パッケージ間は循環させない
- インターフェースはコンシューマ側で定義する（AWS クライアントのモック用）

## Bubble Tea アーキテクチャパターン

- Model / Update / View パターンを遵守する
- 副作用（API呼び出し等）は `tea.Cmd` で実行し、`Update` 内で直接I/Oを行わない
- 状態遷移は `Update` 内で完結させ、`View` は描画のみに集中する
- 非同期処理（ログ取得、プレビュー更新）は `tea.Cmd` で起動し、結果を `tea.Msg` で受け取る

## テスト方針

- **ユニットテスト必須**: `internal/` 配下の全パッケージにテストを書く
- **AWS APIのモック**: インターフェースを定義し、テスト時はモック実装を注入する
  ```go
  type LogsClient interface {
      DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, ...) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
      DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, ...) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
      GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, ...) (*cloudwatchlogs.GetLogEventsOutput, error)
  }
  ```
- **TUIテスト**: Bubble Tea の `tea.Msg` を直接 `Update` に渡してモデルの状態遷移をテストする
- **テスト実行**: `go test ./...`

## ビルド・実行方法

```bash
# 開発環境に入る
nix develop

# 依存解決
go mod tidy

# ビルド
go build -o lazycwl .

# 実行
./lazycwl

# テスト
go test ./...
```

## 実装優先順位

以下の順で機能を実装する（依存関係とTDDサイクルを考慮）:

1. **Phase 1: 基盤** — プロジェクト初期化（go mod init）、AWSクライアントラッパー（F11, F12）
2. **Phase 2: コアTUI** — ロググループ一覧（F1）、カーソル移動（F4）、基本的な1カラムリスト表示
3. **Phase 3: 階層ナビゲーション** — ログストリーム一覧（F2）、階層移動（F3）、3カラムレイアウト
4. **Phase 4: エディタ連携** — ログフォーマット（F10）、$EDITOR起動（F5）
5. **Phase 5: 検索・フィルター** — インクリメンタル検索（F6）、時間範囲指定（F7）、ソート（F15）
6. **Phase 6: 拡張機能** — プレビューペイン（F13）、複数ストリーム選択（F14）、ページネーション（F9）
7. **Phase 7: CLI** — CLI引数対応（F8）

## コミット規約

Conventional Commits に従う:

```
feat: ロググループ一覧表示を実装
fix: ページネーション時のカーソル位置リセットを修正
test: AWSクライアントのモックテストを追加
refactor: TUIモデルの状態管理を整理
```

## 言語方針

- コード内のコメント: 英語
- コミットメッセージ: 日本語（Conventional Commits のプレフィックスは英語）
- ドキュメント: 日本語

## 禁止事項

- `docs/requirements.md` のスコープ外に記載された機能の実装
- 不要なファイル（README.md等）の自動生成
- 過度な抽象化やデザインパターンの適用（YAGNI原則を守る）
- テストなしのコードコミット
- `internal/` 外への実装コードの配置
