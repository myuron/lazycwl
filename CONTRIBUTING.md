# Contributing to lazycwl

lazycwl へのコントリビュートに興味を持っていただきありがとうございます。

## 開発環境のセットアップ

### 前提条件

- [Nix](https://nixos.org/download.html)（Flake対応）
- [Docker](https://docs.docker.com/get-docker/) / Docker Compose

### セットアップ手順

```bash
# 1. リポジトリをクローン
git clone https://github.com/myuron/lazycwl.git
cd lazycwl

# 2. 開発環境に入る（Go、各種ツールが利用可能になる）
nix develop

# 3. 依存解決
go mod tidy

# 4. テストを実行して環境を確認
go test ./...
```

### ローカル動作確認（AWS環境不要）

floci を使ってローカルで動作確認できます。詳細は [README.md の Local Development セクション](README.md#local-development-floci) を参照してください。

## 開発の進め方

### 1. Issue を確認する

まず [Issues](https://github.com/myuron/lazycwl/issues) を確認してください。取り組みたい Issue があればコメントで意思表示をお願いします。新しい機能の提案はまず Issue を作成して議論してください。

### 2. ブランチを作成する

```bash
git checkout -b <type>/<short-description>
```

ブランチ名の例:
- `feat/preview-pane`
- `fix/cursor-reset-on-paginate`
- `docs/keybind-table`

### 3. TDDで実装する

このプロジェクトではTDD（テスト駆動開発）を採用しています。

1. **Red**: 失敗するテストを先に書く
2. **Green**: テストを通す最小限のコードを書く
3. **Refactor**: コードを整理する（テストが通る状態を維持）

```bash
# テスト実行
go test ./...

# 特定パッケージのテスト
go test ./internal/tui/...
```

### 4. コードスタイル

- `gofmt` / `goimports` でフォーマットする
- コード内のコメントは英語で書く
- エラーは `fmt.Errorf("doing X: %w", err)` でラップして呼び出し元に返す
- Bubble Tea の Model / Update / View パターンを遵守する
- `Update` 内で直接I/Oを行わず、副作用は `tea.Cmd` で実行する

### 5. コミットする

[Conventional Commits](https://www.conventionalcommits.org/) に従ってください。コミットメッセージは日本語で書きます（プレフィックスは英語）。

```
feat: ロググループ一覧表示を実装
fix: ページネーション時のカーソル位置リセットを修正
test: AWSクライアントのモックテストを追加
refactor: TUIモデルの状態管理を整理
docs: READMEにキーバインド表を追加
```

### 6. Pull Request を作成する

- PRタイトルはコミットメッセージと同じ Conventional Commits 形式にする
- テンプレートに沿って説明を記入する
- テストが通っていることを確認する

## プロジェクト構成

```
lazycwl/
├── main.go                   # エントリーポイント
├── internal/
│   ├── aws/                  # AWS CloudWatch Logs APIクライアント
│   ├── tui/                  # Bubble Tea TUIレイヤー
│   ├── editor/               # $EDITOR連携
│   └── formatter/            # ログイベントのフォーマット
├── docs/
│   └── requirements.md       # 要件定義書
└── CLAUDE.md                 # Claude Code用の開発ガイドライン
```

実装の詳細は `CLAUDE.md` と `docs/requirements.md` を参照してください。

## 注意事項

- `docs/requirements.md` のスコープ外の機能は実装しない
- テストなしのコードはマージされない
- `internal/` 外に実装コードを配置しない
