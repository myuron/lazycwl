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

English: [README.md](README.md)

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

## Tips

### Vimでログを保存する

`$EDITOR`でログを開くと、一時ファイルに書き出された内容が表示される。この一時ファイルはエディタ終了後に自動削除される。ログを永続的に保存するには、終了前にVimの`:w`コマンドで別ファイルに書き出す:

```vim
:w ~/logs/incident-2024-01-15.log
```

必要な行だけ抽出・加工してから保存することもできる。これが障害調査の想定ワークフローとなっている。

## コントリビュート

バグ報告・機能提案・プルリクエストを歓迎する。開発環境のセットアップ、flociを使ったローカルテスト、TDDワークフローについては [CONTRIBUTING.md](CONTRIBUTING.md)（英語）を参照。

## ライセンス

[MIT](LICENSE)
