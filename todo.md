# lazycwl TODO

## Phase 1: 基盤
- [x] 2026-04-13 go mod init + 依存追加
- [x] 2026-04-13 AWSクライアントインターフェース定義 + モックテスト (F11, F12)
- [x] 2026-04-13 AWSクライアント実装

## Phase 2: コアTUI
- [x] 2026-04-13 ロググループ一覧表示のテスト作成 (F1)
- [x] 2026-04-13 ロググループ一覧表示の実装
- [x] 2026-04-13 カーソル移動のテスト作成 (F4)
- [x] 2026-04-13 カーソル移動の実装

## Phase 3: 階層ナビゲーション
- [x] 2026-04-13 ログストリーム一覧のテスト作成 (F2)
- [x] 2026-04-13 ログストリーム一覧の実装
- [x] 2026-04-13 階層移動のテスト作成 (F3)
- [x] 2026-04-13 階層移動の実装
- [x] 2026-04-13 3カラムレイアウトの実装

## Phase 4: エディタ連携
- [x] 2026-04-13 ログフォーマットのテスト作成 (F10)
- [x] 2026-04-13 ログフォーマットの実装
- [x] 2026-04-13 $EDITOR起動のテスト作成 (F5)
- [x] 2026-04-13 $EDITOR起動の実装
- [x] 2026-04-13 TUIからのエディタ連携統合

## Phase 5: 検索・フィルター
- [x] 2026-04-13 インクリメンタル検索のテスト作成 (F6)
- [x] 2026-04-13 インクリメンタル検索の実装
- [x] 2026-04-13 時間範囲指定のテスト作成 (F7)
- [x] 2026-04-13 時間範囲指定の実装
- [x] 2026-04-13 ソート切り替えのテスト作成 (F15)
- [x] 2026-04-13 ソート切り替えの実装

## Phase 6: 拡張機能
- [x] 2026-04-13 プレビューペインの実装 (F13) — 3カラムレイアウト内
- [x] 2026-04-13 複数ストリーム選択のテスト作成 (F14)
- [x] 2026-04-13 複数ストリーム選択の実装
- [x] 2026-04-13 ページネーション対応 (F9) — NextToken管理

## Phase 7: CLI
- [x] 2026-04-13 CLI引数対応のテスト作成 (F8)
- [x] 2026-04-13 CLI引数対応の実装 (--group, --stream, --since, --profile, --region)

## バグ修正
- [x] 2026-04-13 検索経由でログストリームに移動した際、groupCursorがフィルタ済みインデックスを保存していたバグを修正

## コードレビュー指摘事項
- [x] 2026-04-13 cmd/lazycwl/main.go が存在しない — 確認の結果、存在していた（誤検知）
- [x] 2026-04-13 go.mod: 直接依存が全て // indirect になっている — go mod tidy で修正
- [x] 2026-04-13 client.go: endpointURL() が冗長 — os.Getenv を直接返すように簡略化
- [x] 2026-04-13 model.go: fetchMultiLogEvents が逐次API呼び出し — sync.WaitGroup で並行取得に変更
- [x] 2026-04-13 model.go: fetchLogGroups等で context.Background() をハードコード — Model にctx/cancelを保持、quit時にcancel呼び出し
- [x] 2026-04-13 model.go: 700行超の単一ファイル — groups.go/streams.go/preview.go/keys.go に分割
- [x] 2026-04-13 CLI引数パース部分のコード・テストが不在 — 確認の結果、main.goに実装済み（誤検知）
- [x] 2026-04-13 acceptance_test.go: execCmd の再帰がエディタ起動を防いでいない — tea.ExecMsg をスキップするガード追加
- [x] 2026-04-13 searchQuery のバックスペースがバイト単位 — rune単位の削除に修正（timeInputも同様）
- [x] 2026-04-14 parseDuration に負の値・ゼロの検証がない — num <= 0 をエラーにするバリデーション追加（TUI側・CLI側両方）
- [x] 2026-04-14 Spaceキー（複数選択トグル）が tea.KeySpace で届くのに tea.KeyRunes でハンドルしていた — KeySpace ケースを追加
- [x] 2026-04-14 3カラムレイアウト → 2カラム（左: LogGroups、右: LogStreams + Last Event）に変更
- [x] 2026-04-14 GetLogEvents にページネーション追加（NextForwardToken をループして全イベント取得）
- [x] 2026-04-14 複数ストリーム並行取得の並行数制限（セマフォで最大5並行）+ GetMultiStreamLogEvents をawsパッケージに移動

## スクロール対応 (#2, #4)
- [x] 2026-04-14 スクロールオフセットのテスト作成（10テスト）
- [x] 2026-04-14 スクロールオフセットの実装（Model に offset/groupOffset フィールド追加）
- [x] 2026-04-14 render関数をオフセット対応に修正（renderGroupList, renderGroupListInactive, renderStreamList）
- [x] 2026-04-14 カーソル移動時のビューポート追従ロジック実装（adjustOffset + 全カーソル操作箇所に適用）

## ペインサイズ不一致修正 (#3)
- [x] 2026-04-14 ペイン高さ/幅テスト作成（5テスト）
- [x] 2026-04-14 render関数の末尾改行によるHeight超過を修正（strings.TrimRight）

## WSLペインサイズ不一致修正 (#6)
- [x] 2026-04-14 render関数のmaxHeightパディングテスト作成（2テスト）
- [x] 2026-04-14 render関数でmaxHeight行にパディング + lipgloss Height()依存を除去

## WSLレイアウト崩れ根本修正 (#7, #8)
- [x] 2026-04-14 strings.TrimRight→TrimSuffixに変更（パディング全除去を防止）
- [x] 2026-04-14 lipgloss JoinHorizontal/JoinVerticalを廃止、行ごとに直接出力組み立て
- [x] 2026-04-14 最終出力をm.height-1行にハードキャップ

## ページネーション修正 (F9)
- [x] 2026-04-14 fetchLogGroups/fetchLogStreamsをPage API対応に変更（NextToken返却）
- [x] 2026-04-14 カーソル末尾到達時に次ページを非同期追加取得（maybeFetchMore）
- [x] 2026-04-14 追加ページはリストにappend（既存データを保持）
