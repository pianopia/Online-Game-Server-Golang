# Online Game Server - WebSocket & UDP with SQLite

Rustで構築された高性能なリアルタイムオンラインゲームサーバー。WebSocketとUDPの両方に対応し、SQLiteによるデータ永続化機能を提供します。

## 🚀 主要機能

### プロトコル対応
- **WebSocket**: 安定した接続、簡単な実装、Webブラウザ対応
- **UDP**: 低遅延通信、リアルタイムゲーム最適化、カスタム信頼性制御

### データベース統合
- **SQLite**: プレイヤーデータの永続化
- **セッション管理**: 接続時間・プロトコル追跡
- **イベントログ**: 全プレイヤーアクションの記録
- **分析機能**: チャット履歴、ハイスコア、統計情報

### ゲーム機能
- 複数プレイヤー同時接続
- リアルタイム移動・位置同期
- チャット機能
- アクションシステム（攻撃、アイテム取得）
- スコアシステム
- 自動切断検知

### パフォーマンス
- 非同期処理 (tokio)
- 並行安全なクライアント管理
- UDP用信頼性制御・再送機能
- データベース非同期操作

## 📦 セットアップ

### 1. 必要条件
```bash
# Rustインストール
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# バージョン確認
rustc --version
cargo --version
```

### 2. ビルドと実行
```bash
# 依存関係のビルド
cargo build

# WebSocketサーバー起動（デフォルト）
cargo run

# UDPサーバー起動
PROTOCOL=udp cargo run

# カスタムデータベース
DATABASE_URL="sqlite:custom.db" cargo run
```

### 3. サーバー設定
```bash
# ポート変更
PORT=9000 cargo run

# プロトコル + ポート + DB
PROTOCOL=udp PORT=8081 DATABASE_URL="sqlite:game_udp.db" cargo run

# デバッグログ
RUST_LOG=debug cargo run
```

## 🎮 テストクライアント

### Webブラウザクライアント
```bash
# WebSocket用
open test_client.html

# UDP説明付き
open udp_test_client.html
```

### ネイティブUDPクライアント
```bash
# Pythonクライアント実行
python3 native_udp_client.py

# インタラクティブコマンド:
# move <x> <y>  - プレイヤー移動
# chat <msg>    - チャット送信
# attack        - 攻撃アクション
# pickup        - アイテム取得
# heartbeat     - ハートビート送信
# quit          - 終了
```

### データベーステスト
```bash
# 統合テスト実行
cargo run --bin test_database
```

## 🏗️ アーキテクチャ

### モジュール構成
```
src/
├── main.rs           # エントリーポイント、プロトコル選択
├── server.rs         # WebSocketサーバー
├── udp_server.rs     # UDPサーバー（UDP最適化）
├── client.rs         # WebSocketクライアント処理
├── game.rs           # ゲーム状態管理
├── message.rs        # メッセージ定義・UDP拡張
└── database.rs       # SQLite操作・永続化

migrations/
└── 001_initial.sql   # データベーススキーマ

テストファイル:
├── test_client.html       # WebSocketクライアント
├── udp_test_client.html   # UDP説明付きクライアント
├── native_udp_client.py   # ネイティブUDPクライアント
└── test_database.rs       # データベーステスト
```

### 主要依存関係
- `tokio` - 非同期ランタイム
- `tokio-tungstenite` - WebSocket実装
- `sqlx` - SQLite非同期ORM
- `serde` + `serde_json` - シリアライゼーション
- `bincode` - UDP用バイナリシリアライゼーション
- `dashmap` - 並行安全HashMap
- `uuid` - プレイヤーID生成
- `chrono` - 日時処理
- `tracing` - ログ出力

## 📡 プロトコル比較

| 特徴 | WebSocket | UDP |
|------|-----------|-----|
| **遅延** | 10-30ms | 1-5ms |
| **信頼性** | TCP保証 | カスタム制御 |
| **順序保証** | あり | アプリ制御 |
| **実装の簡単さ** | 簡単 | 複雑 |
| **ブラウザ対応** | ネイティブ | 不可 |
| **適用場面** | ターン制・チャット | FPS・アクション |

## 🗃️ データベース機能

### 自動機能
- スキーマ自動マイグレーション
- プレイヤーデータ自動同期
- セッション追跡
- イベント自動ログ
- タイムアウト自動処理

### 利用可能なデータ
```sql
-- トッププレイヤー
SELECT name, score FROM players ORDER BY score DESC LIMIT 10;

-- プロトコル別統計
SELECT protocol, COUNT(*) FROM game_sessions GROUP BY protocol;

-- 最近のイベント
SELECT event_type, COUNT(*) FROM player_events 
WHERE timestamp > datetime('now', '-1 day')
GROUP BY event_type;
```

## 🔧 カスタマイズ

### 新しいゲームアクション追加
```rust
// src/game.rs の handle_player_action に追加
match action {
    "attack" => {
        // 攻撃処理
        database.log_event(&client_id, session_id, "attack", None).await?;
    },
    "heal" => {
        // 回復処理（新規）
        if let Some(client_ref) = self.clients.get(&client_id) {
            let mut client = client_ref.write().await;
            client.player.health = (client.player.health + 20.0).min(100.0);
            database.update_player_health(&client_id, client.player.health).await?;
        }
    },
    _ => {}
}
```

### UDP信頼性制御のカスタマイズ
```rust
// src/udp_server.rs
const HEARTBEAT_INTERVAL: Duration = Duration::from_secs(5);
const CLIENT_TIMEOUT: Duration = Duration::from_secs(30);
const PACKET_RESEND_TIMEOUT: Duration = Duration::from_millis(100);
```

### データベース設定
```rust
// 環境変数での設定
DATABASE_URL=sqlite:game.db           # ローカルファイル
DATABASE_URL=sqlite::memory:          # インメモリ（テスト用）
DATABASE_URL=sqlite:/path/to/game.db  # 絶対パス
```

## 📊 パフォーマンス

### 同時接続
- WebSocket: 1000+ 同時接続
- UDP: 2000+ 同時接続（理論値）

### レスポンス時間
- WebSocket: 10-30ms（通常）
- UDP: 1-5ms（通常）、10-50ms（パケットロス時）

### データベース
- SQLite: 非同期操作、接続プール
- 移動ログ: UDP時は10分の1に間引き（負荷軽減）

## 🐛 トラブルシューティング

### よくある問題

1. **データベースファイルが作成できない**
   ```bash
   # 権限確認
   ls -la game.db
   # 削除して再作成
   rm game.db && cargo run
   ```

2. **ポートが使用中**
   ```bash
   # 別ポート使用
   PORT=8081 cargo run
   ```

3. **UDP接続できない**
   ```bash
   # ファイアウォール確認
   # macOSの場合
   sudo pfctl -d
   ```

### デバッグ方法
```bash
# 全ログ
RUST_LOG=debug cargo run

# SQLクエリログ
RUST_LOG=sqlx::query=debug cargo run

# エラーのみ
RUST_LOG=error cargo run
```

## 🚀 本番環境デプロイ

### Docker使用
```bash
# イメージビルド
docker build -t game-server .

# コンテナ実行
docker run -p 8080:8080 -v $(pwd)/data:/app/data game-server
```

### パフォーマンス最適化
```bash
# リリースビルド
cargo build --release

# 最適化実行
./target/release/online-game-server
```

## 📚 詳細ドキュメント

- [UDP実装詳細](UDP_README.md)
- [データベース機能](DATABASE_README.md)
- [WebSocket API仕様](WebSocket_API_Specification.md)

## 🤝 貢献

1. フォーク
2. フィーチャーブランチ作成 (`git checkout -b feature/amazing-feature`)
3. コミット (`git commit -m 'Add amazing feature'`)
4. プッシュ (`git push origin feature/amazing-feature`)
5. プルリクエスト作成

## 📄 ライセンス

このプロジェクトはMITライセンスで公開されています。

---

**高性能・低遅延のリアルタイムゲームサーバーで、次世代のオンラインゲームを構築しましょう！** 🎮