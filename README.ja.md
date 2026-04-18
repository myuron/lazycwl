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

AWS CloudWatch Logsをターミナルで素早く閲覧・調査するためのTUIツール。yazi風の階層ナビゲーションでロググループ/ストリームをブラウズし、選択したログを`$EDITOR`で開いてVimベースの障害調査を行う。

## インストール

```bash
go install github.com/myuron/lazycwl@latest
```

または Nix Flake で直接実行:

```bash
nix run github:myuron/lazycwl
```

自分の flake に追加する場合:

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
      # pkgs.lazycwl が利用可能
      devShells.${system}.default = pkgs.mkShell {
        packages = [ pkgs.lazycwl ];
      };
    };
}
```

## 使い方

```bash
# デフォルト（環境変数やAWSプロファイルの認証情報を使用）
lazycwl

# プロファイル・リージョン指定
lazycwl --profile my-profile --region ap-northeast-1

# ロググループを直接指定してストリーム一覧から開始
lazycwl --group /aws/lambda/my-function

# ロググループとストリームを直接指定して$EDITORで開く（TUIスキップ）
lazycwl --group /aws/lambda/my-function --stream 'stream-name'
```

## 注意事項

- ログストリームはデフォルトで最終イベント時刻の降順でソートされる。一度もログイベントを受信していないストリームは、AWS APIが `OrderBy=LastEventTime` 指定時に除外するため、このモードでは**表示されない**。空のストリームも含めて全件表示するには、`s` キーでストリーム名順ソートに切り替える。

## キーバインド

### ノーマルモード

| キー | 動作 |
|------|------|
| `j` / `↓` | カーソルを下に移動 |
| `k` / `↑` | カーソルを上に移動 |
| `g` | リスト先頭に移動 |
| `G` | リスト末尾に移動 |
| `l` / `Enter` / `→` | 階層を深く進む / ログをエディタで開く |
| `h` / `Backspace` / `←` | 階層を戻る |
| `/` | インクリメンタル検索モードに入る |
| `Space` | ストリームの複数選択トグル（ストリーム一覧画面のみ） |
| `s` | ソート切り替え: 最終イベント時刻順 ↔ ストリーム名順（ストリーム一覧画面のみ） |
| `q` | アプリケーション終了 |

### 検索モード（`/` で入る）

| キー | 動作 |
|------|------|
| 文字入力 | インクリメンタルにリストを絞り込み |
| `Enter` | 検索を確定してノーマルモードに戻る |
| `Escape` | 検索をクリアしてノーマルモードに戻る |
| `Backspace` | 検索文字列を1文字削除 |

## レイアウト

```
┌──────────────────┬────────────────────────────────────┐
│ ロググループ      │ ログストリーム                      │
│                  │                                    │
│ /aws/lambda   ←  │ > stream-001  ←                    │
│ /aws/ecs         │   stream-002                       │
│ /app/api         │   stream-003                       │
│                  │                                    │
├──────────────────┴────────────────────────────────────┤
│ Sort: time ↓ | q: quit | /: search | s: sort         │
└───────────────────────────────────────────────────────┘
```

## ローカル開発（floci）

[floci](https://github.com/floci-io/floci)を使ってAWS環境なしで動作確認できる。

### セットアップ

```bash
# 1. 開発環境に入る
nix develop

# 2. flociを起動
docker compose up -d

# 3. テストデータを投入
./scripts/seed-testdata.sh

# 4. lazycwlをfloci向けに起動
AWS_ENDPOINT_URL=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
AWS_DEFAULT_REGION=ap-northeast-1 \
go run .
```

### テストデータの内容

seedスクリプトは以下のロググループとストリームを作成する:

| ロググループ | ストリーム | 内容 |
|---|---|---|
| `/aws/lambda/api-handler` | `[$LATEST]abc123` | 正常なAPIリクエスト処理 |
| `/aws/lambda/api-handler` | `[$LATEST]def456` | DB接続タイムアウトエラー |
| `/aws/lambda/batch-processor` | `[$LATEST]ghi789` | バッチ処理（スロークエリ警告あり） |
| `/aws/ecs/web-service` | `web-service/web/task-001` | Webサーバー起動、500エラー含む |
| `/app/api/backend` | `i-0abc123def456` | サーキットブレーカー動作 |
| `/app/worker/queue-consumer` | `worker-1` | キュー処理、決済エラー含む |

### 大量テストデータの投入

ページネーションやスクロール性能のテスト用に、大量データを生成するスクリプトも用意されている。

```bash
# デフォルト: 50グループ × 20ストリーム × 100イベント = 100,000イベント
./scripts/seed-large-testdata.sh

# オプションでカスタマイズ可能
./scripts/seed-large-testdata.sh --groups 10 --streams 5 --events 50
./scripts/seed-large-testdata.sh --groups 100 --streams 50 --events 500
```

| オプション | デフォルト | 説明 |
|---|---|---|
| `--groups` | 50 | 作成するロググループ数 |
| `--streams` | 20 | グループあたりのストリーム数 |
| `--events` | 100 | ストリームあたりのイベント数 |

### flociの停止

```bash
docker compose down
```

## Tips

### Vimでログを保存する

`$EDITOR`でログを開くと、一時ファイルに書き出された内容が表示される。この一時ファイルはエディタ終了後に自動削除される。ログを永続的に保存するには、終了前にVimの`:w`コマンドで別ファイルに書き出す:

```vim
:w ~/logs/incident-2024-01-15.log
```

必要な行だけ抽出・加工してから保存することもできる。これが障害調査の想定ワークフローとなっている。

## 技術スタック

- [Go](https://go.dev/)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUIフレームワーク
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUIコンポーネント
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — スタイリング
- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2) — CloudWatch Logs API
- [Nix Flake](https://nixos.wiki/wiki/Flakes) — ビルド管理
