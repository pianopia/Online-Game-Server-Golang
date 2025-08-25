# UDP Game Server Implementation

このプロジェクトはWebSocketベースのゲームサーバーをUDPに拡張したものです。

## 実装の特徴

### UDP vs WebSocket の主な違い

| 項目 | UDP | WebSocket (TCP) |
|------|-----|----------------|
| **遅延** | 低遅延 | 高遅延 |
| **順序保証** | なし（アプリで制御） | あり |
| **信頼性** | なし（アプリで制御） | あり |
| **接続** | コネクションレス | コネクション指向 |
| **オーバーヘッド** | 小 | 大 |
| **輻輳制御** | なし | あり |

### 実装したUDP機能

1. **パケット構造**
   - バイナリシリアライゼーション（bincode）
   - シーケンス番号による順序管理
   - タイムスタンプ
   - 信頼性フラグ

2. **信頼性保証**
   - ACK/NACK メカニズム
   - タイムアウトによる再送
   - パケット重複検出

3. **接続管理**
   - ハートビート機能
   - クライアントタイムアウト検出
   - 自動切断処理

4. **パフォーマンス最適化**
   - 移動メッセージは非信頼性（高頻度更新）
   - チャット・アクションは信頼性あり
   - バックグラウンドタスクによる非同期処理

## 使用方法

### UDPサーバーの起動

```bash
# UDP モードでサーバーを起動
PROTOCOL=udp cargo run

# ポート指定
PROTOCOL=udp PORT=8080 cargo run
```

### WebSocketサーバーの起動（従来通り）

```bash
# デフォルト（WebSocket）
cargo run

# または明示的に指定
PROTOCOL=websocket cargo run
```

### クライアントテスト

#### 1. ブラウザベース（WebSocket）
```bash
# ブラウザで開く
open test_client.html
# または
open udp_test_client.html  # UDPの説明付き
```

#### 2. ネイティブUDPクライアント
```bash
# Python クライアントを実行
python3 native_udp_client.py
```

## アーキテクチャ

### サーバー構造

```
src/
├── main.rs           # エントリーポイント、プロトコル選択
├── server.rs         # WebSocket サーバー（既存）
├── udp_server.rs     # UDP サーバー（新規）
├── client.rs         # WebSocket クライアント処理
├── game.rs           # ゲーム状態管理
└── message.rs        # メッセージ定義（拡張）
```

### UDP パケット形式

```rust
pub struct UdpPacket {
    pub sequence: u32,      // シーケンス番号
    pub timestamp: u64,     // タイムスタンプ（ミリ秒）
    pub message: GameMessage, // ゲームメッセージ
    pub reliable: bool,     // 信頼性フラグ
}
```

### 信頼性制御

```
Client                    Server
  |                         |
  |  Reliable Message       |
  |------------------------>|
  |                         |
  |         ACK             |
  |<------------------------|
  |                         |
  | (timeout) Resend        |
  |------------------------>|
```

## パフォーマンス比較

### レスポンス時間（概算）

| プロトコル | 通常時 | 高負荷時 | パケットロス時 |
|----------|-------|---------|---------------|
| **UDP** | 1-5ms | 2-8ms | 5-15ms（再送） |
| **WebSocket** | 10-30ms | 20-100ms | 50-500ms（再送・輻輳制御） |

### 適用場面

#### UDP が有利
- FPS、アクションゲーム
- リアルタイム位置情報更新
- 高頻度の状態同期
- 低遅延が最優先

#### WebSocket が有利
- ターン制ゲーム
- チャット中心のゲーム
- 確実な配信が必要
- 実装の簡単さを重視

## トラブルシューティング

### よくある問題

1. **ポートが使用中**
   ```bash
   # 別のポートを使用
   PROTOCOL=udp PORT=8081 cargo run
   ```

2. **パケット解析エラー**
   ```
   # Rustサーバーはbincode、PythonクライアントはJSONを使用
   # 実際の本格実装では統一が必要
   ```

3. **ファイアウォール問題**
   ```bash
   # macOS の場合、UDPポートを許可
   sudo pfctl -d  # 一時的にファイアウォール無効化（テスト用）
   ```

### ログレベル調整

```bash
# 詳細ログ
RUST_LOG=debug PROTOCOL=udp cargo run

# エラーのみ
RUST_LOG=error PROTOCOL=udp cargo run
```

## 今後の拡張案

1. **高度な信頼性制御**
   - 選択的再送（SACK）
   - 適応的タイムアウト

2. **パフォーマンス改善**
   - パケット圧縮
   - 差分更新
   - 予測・補間

3. **セキュリティ**
   - パケット暗号化
   - 認証機能
   - DDoS対策

4. **スケーラビリティ**
   - 負荷分散
   - クラスター対応
   - 状態同期

## 参考資料

- [UDP vs TCP for Gaming](https://gafferongames.com/post/udp_vs_tcp/)
- [Reliable UDP Implementation](https://gafferongames.com/post/reliable_ordered_messages/)
- [Game Network Architecture](https://docs.unity3d.com/Manual/UNet.html)