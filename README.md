# lazycwl

AWS CloudWatch Logsをターミナルで素早く閲覧・調査するためのTUIツール。yazi風の階層ナビゲーションでロググループ/ストリームをブラウズし、選択したログを`$EDITOR`で開いてVimベースの障害調査を行う。

## インストール

```bash
go install github.com/myuron/lazycwl/cmd/lazycwl@latest
```

または Nix Flake 経由:

```bash
nix develop  # 開発環境
go build -o lazycwl ./cmd/lazycwl/
```

## 使い方

```bash
# デフォルト（環境変数やAWSプロファイルの認証情報を使用）
lazycwl

# プロファイル・リージョン指定
lazycwl --profile my-profile --region ap-northeast-1
```

### キーバインド

| キー | 動作 |
|------|------|
| `j` / `k` | カーソル移動 |
| `l` / `Enter` | 階層を進む / ログをエディタで開く |
| `h` / `Backspace` | 階層を戻る |
| `g` / `G` | リスト先頭 / 末尾 |
| `q` | 終了 |

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
go run ./cmd/lazycwl/
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

### flociの停止

```bash
docker compose down
```
