# lazycwl

AWS CloudWatch Logsをターミナルで素早く閲覧・調査するためのTUIツール。yazi風の階層ナビゲーションでロググループ/ストリームをブラウズし、選択したログを`$EDITOR`で開いてVimベースの障害調査を行う。

## インストール

```bash
go install github.com/myuron/lazycwl@latest
```

または Nix Flake 経由:

```bash
nix develop  # 開発環境
go build -o lazycwl .
```

## 使い方

```bash
# デフォルト（環境変数やAWSプロファイルの認証情報を使用）
lazycwl

# プロファイル・リージョン指定
lazycwl --profile my-profile --region ap-northeast-1
```

### キーバインド

#### ノーマルモード

| キー | 動作 |
|------|------|
| `j` / `↓` | カーソルを下に移動 |
| `k` / `↑` | カーソルを上に移動 |
| `g` | リスト先頭に移動 |
| `G` | リスト末尾に移動 |
| `l` / `Enter` / `→` | 階層を深く進む / ログをエディタで開く |
| `h` / `Backspace` / `←` | 階層を戻る |
| `/` | インクリメンタル検索モードに入る |
| `t` | 時間範囲入力モードに入る |
| `Space` | ストリームの複数選択トグル（ストリーム一覧画面のみ） |
| `s` | ソート切り替え: 最終イベント時刻順 ↔ ストリーム名順（ストリーム一覧画面のみ） |
| `q` | アプリケーション終了 |

#### 検索モード（`/` で入る）

| キー | 動作 |
|------|------|
| 文字入力 | インクリメンタルにリストを絞り込み |
| `Enter` | 検索を確定してノーマルモードに戻る |
| `Escape` | 検索をクリアしてノーマルモードに戻る |
| `Backspace` | 検索文字列を1文字削除 |

#### 時間範囲入力モード（`t` で入る）

| キー | 動作 |
|------|------|
| 文字入力 | 時間範囲を入力（例: `30m`, `2h`, `7d`） |
| `Enter` | 時間範囲を確定してノーマルモードに戻る |
| `Escape` | 入力をキャンセルしてノーマルモードに戻る |
| `Backspace` | 入力文字列を1文字削除 |

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
