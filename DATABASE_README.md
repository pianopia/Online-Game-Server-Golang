# SQLite Database Integration

ゲームサーバーにSQLiteデータベースを統合し、プレイヤーデータの永続化と分析機能を実装しました。

## 実装機能

### 1. データベーススキーマ

**Players テーブル**
- プレイヤー情報の永続化
- 位置、スコア、体力の追跡
- 作成・更新・最終接続時刻の記録

**Game Sessions テーブル**
- セッション管理（WebSocket/UDP）
- 接続時間の追跡
- クライアントIP記録

**Player Events テーブル**
- 全プレイヤーアクションのログ
- move, chat, attack, pickup, join, leave イベント
- JSONデータ付きイベント詳細

**Chat Messages テーブル**
- チャット履歴の永続化
- セッション紐付け

**High Scores テーブル**
- ハイスコア・リーダーボード
- ゲーム時間記録

### 2. 自動機能

**自動マイグレーション**
- サーバー起動時にテーブル作成
- スキーマ更新の自動実行

**リアルタイム同期**
- プレイヤー移動→DB位置更新
- スコア変更→DB同期
- チャット→DB保存
- イベント→自動ログ

**セッション管理**
- 接続時：セッション作成
- 切断時：セッション終了
- タイムアウト：自動クリーンアップ

## 使用方法

### 基本起動

```bash
# デフォルト（SQLite ファイル: game.db）
cargo run

# カスタムデータベース
DATABASE_URL="sqlite:custom.db" cargo run

# インメモリデータベース（テスト用）
DATABASE_URL="sqlite::memory:" cargo run
```

### プロトコル別

```bash
# WebSocket + SQLite
DATABASE_URL="sqlite:websocket_game.db" cargo run

# UDP + SQLite
DATABASE_URL="sqlite:udp_game.db" PROTOCOL=udp cargo run
```

### データベーステスト

```bash
# 統合テスト実行
cargo run --bin test_database
```

## API機能

### プレイヤー操作
- `create_or_update_player()` - プレイヤー作成・更新
- `get_player()` - プレイヤー情報取得
- `update_player_position()` - 位置更新
- `update_player_score()` - スコア更新
- `get_top_players()` - リーダーボード取得

### セッション管理
- `create_session()` - セッション開始
- `end_session()` - セッション終了
- `cleanup_old_sessions()` - タイムアウト処理

### イベント・分析
- `log_event()` - イベント記録
- `get_player_events()` - プレイヤー履歴
- `save_chat_message()` - チャット保存
- `get_recent_chat_messages()` - チャット履歴

###統計情報
- `get_player_count()` - 総プレイヤー数
- `get_active_sessions_count()` - アクティブセッション数
- `get_high_scores()` - ハイスコア一覧

## パフォーマンス最適化

### UDP専用最適化
```rust
// 移動イベントは10回に1回のみログ（負荷軽減）
if sequence % 10 == 0 {
    database.log_event(&player_id, session_id, "move", Some(&move_msg)).await?;
}
```

### インデックス最適化
- プレイヤー名・スコア・最終接続時刻
- セッション・イベント・チャット検索
- リーダーボード高速化

### 非同期処理
- 全データベース操作は非ブロッキング
- ゲームループに影響なし
- エラーハンドリング付き

## データ分析例

### プレイヤー分析
```sql
-- アクティブプレイヤー（過去24時間）
SELECT name, score, last_seen_at 
FROM players 
WHERE last_seen_at > datetime('now', '-1 day')
ORDER BY score DESC;

-- プロトコル別統計
SELECT protocol, COUNT(*) as sessions, 
       AVG(julianday(COALESCE(session_end, datetime('now'))) - julianday(session_start)) * 24 * 60 as avg_minutes
FROM game_sessions 
GROUP BY protocol;
```

### イベント分析
```sql
-- 人気アクション
SELECT event_type, COUNT(*) as count
FROM player_events 
WHERE timestamp > datetime('now', '-1 day')
GROUP BY event_type
ORDER BY count DESC;

-- チャット統計
SELECT DATE(timestamp) as date, COUNT(*) as messages
FROM chat_messages
GROUP BY DATE(timestamp)
ORDER BY date DESC;
```

## ファイル構造

```
src/
├── database.rs       # データベース操作・API
├── main.rs          # DB初期化・サーバー起動
├── game.rs          # ゲーム状態とDB連携
├── client.rs        # WebSocketクライアントDB連携
├── udp_server.rs    # UDPサーバーDB連携
└── ...

migrations/
└── 001_initial.sql  # 初期スキーマ定義

test_database.rs     # データベース機能テスト
```

## トラブルシューティング

### よくある問題

1. **データベースファイル権限**
   ```bash
   chmod 666 game.db
   ```

2. **マイグレーションエラー**
   ```bash
   # データベースファイル削除して再作成
   rm game.db && cargo run
   ```

3. **パフォーマンス問題**
   ```bash
   # ログレベル調整
   RUST_LOG=error cargo run
   ```

### デバッグ

```bash
# 詳細ログ出力
RUST_LOG=debug cargo run

# SQLクエリログ
RUST_LOG=sqlx::query=debug cargo run
```

## 今後の拡張

### 高度な分析
- プレイヤー行動予測
- チート検出システム
- パフォーマンス分析

### スケーラビリティ
- PostgreSQL対応
- 分散データベース
- データ分割・アーカイブ

### セキュリティ
- データ暗号化
- バックアップ自動化
- アクセス制御

## テスト結果

✅ **全データベーステストが成功**
- プレイヤー操作：作成・更新・取得
- セッション管理：開始・終了・クリーンアップ  
- イベントログ：join・move・attack・pickup
- チャット機能：保存・履歴取得
- 統計機能：カウント・リーダーボード
- ハイスコア：保存・取得

データベース統合により、ゲームデータの永続化と分析が可能になりました。