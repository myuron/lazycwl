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

## ログストリーム格納サイズ表示
- [x] 2026-04-15 要件定義書 F2 に格納サイズ表示を追加
- [x] 2026-04-15 LogStream 構造体に StoredBytes フィールド追加 + テスト
- [x] 2026-04-15 TUI表示にストリーム格納サイズを追加 + formatBytes ヘルパー + テスト

## 時間範囲フィルタ削除 (F7)
- [x] 2026-04-15 GetLogEvents/GetMultiStreamLogEventsからstartTime/endTimeパラメータを削除
- [x] 2026-04-15 TUIからsinceDuration/timeInput/modeTimeInput/tキーバインドを削除
- [x] 2026-04-15 main.goから--sinceフラグ/parseSinceを削除
- [x] 2026-04-15 ステータスバーからSince表示を削除
- [x] 2026-04-15 関連テストの削除・更新

## ページネーション修正 (F9)
- [x] 2026-04-14 fetchLogGroups/fetchLogStreamsをPage API対応に変更（NextToken返却）
- [x] 2026-04-14 カーソル末尾到達時に次ページを非同期追加取得（maybeFetchMore）
- [x] 2026-04-14 追加ページはリストにappend（既存データを保持）

## DescribeLogStreams パラメータ修正 (F2)
- [x] 2026-04-18 テスト追加: OrderBy/Descending/Limit パラメータの検証
- [x] 2026-04-18 実装修正: ListLogStreamsPage に OrderBy=LastEventTime, Descending=true, Limit=50 を追加
- [x] 2026-04-18 リファクタ: ListLogStreams を ListLogStreamsPage のラッパーに簡略化

## リアルタイム監視 (F16)
- [x] 2026-04-23 要件定義書にF16を追加、スコープ外からtailの記述を削除
- [x] 2026-04-23 LogsClientインターフェースにStartLiveTailを追加
- [x] 2026-04-23 LogGroup構造体にARNフィールドを追加
- [x] 2026-04-23 Client.StartLiveTailSessionラッパーメソッド追加 + テスト
- [x] 2026-04-23 viewTail状態、tailEventMsg/tailErrMsg/tailStartedMsg メッセージ型追加
- [x] 2026-04-23 Model にtail関連フィールド追加（tailEvents, tailStreams, tailCancel, tailPaused, tailScrollOffset, tailEventsCh）
- [x] 2026-04-23 tail.go: enterTailMode, exitTailMode, handleTailKey, startTailStream, waitForTailEvent, renderTailView 実装
- [x] 2026-04-23 スクロール（j/k/g/G）、一時停止/再開（p）、終了（q/Esc）のキーバインド実装
- [x] 2026-04-23 tail_test.go: 27テスト作成（入力バリデーション、イベント処理、スクロール、描画、状態遷移）
- [x] 2026-04-23 keys.go にfキーバインド追加、preview.go にviewTailケースとステータスバーヒント追加

## レビュー指摘事項 (PR #22 ultrareview)
- [x] 2026-04-25 merged_bug_001: tailEventMsg でPause/スクロール中の表示位置固定（offset += added + maxOffsetでclamp）+ 4テスト追加
- [x] 2026-04-25 bug_004: tailErrMsg に viewTail ガード追加（終了直後の "tail stream closed" 誤表示修正）+ テスト追加
- [x] 2026-04-25 bug_017: tailErrCh追加、stream.Err()をerrCh経由で surface、waitForTailEventで優先読み取り + 2テスト追加
- [x] 2026-04-25 bug_005: G キーで tailPaused=false にして auto-scroll 再開 + テスト追加
- [x] 2026-04-25 bug_018: マルチ選択ストリーム名を sort.Strings で決定的順序に + テスト追加

## ローディング表示改善
- [x] 2026-05-04 ローディングポップアップのテスト9件追加（loadingMessage/ポップアップ重ね合わせ/サイズ制約/コンテキスト別メッセージ/クリア）
- [x] 2026-05-04 Init() の loading=true がvalue receiverで失われるバグを修正（NewModel/NewModelWithOptionsで初期化）
- [x] 2026-05-04 loadingMessage フィールド追加 + groups/streams/events/sort 各fetch呼び出しで文脈に応じたメッセージを設定
- [x] 2026-05-04 lipgloss + x/ansi.Cut で中央オーバーレイのポップアップを実装（preview.goに renderBaseView/renderLoadingPopup/overlayPopup を追加）

## 起動時エラー表示の見切れ修正
- [x] 2026-05-04 起動時エラー表示のテスト追加3件（Err()アクセサ / View()のwidth折り返し / width=0フォールバック）
- [x] 2026-05-04 Model.Err() アクセサ追加 + preview.go の renderErrorView() で lipgloss 折り返し
- [x] 2026-05-04 main.go で p.Run() 終了後に Model.Err() を stderr へ出力

## ログ取得の並列化 (Issue #26)
- [x] 2026-05-22 LogStream に FirstEventTimestamp フィールド追加 + toLogStream ヘルパー切り出し
- [x] 2026-05-22 DescribeLogStream 単一ストリーム metadata 取得を追加（exact-match filter）
- [x] 2026-05-22 GetLogEventsByTimeRange を追加（StartTime/EndTime 指定 + ページネーション）
- [x] 2026-05-22 GetMultiStreamLogEvents を時間チャンク並列に再実装（planTimeChunks で [first,last+1) を timeChunks 等分）
- [x] 2026-05-22 グローバルセマフォ maxConcurrent=8 で全 GetLogEvents 呼び出しの並列度を抑制
- [x] 2026-05-22 describe 失敗 / range 不正 / 極小ストリームは逐次 GetLogEvents にフォールバック
- [x] 2026-05-22 テスト追加 7件（Describe / TimeRange / 並列チャンク / フォールバック / FirstEventTimestamp 伝播）
