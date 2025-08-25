# Cloud Run デプロイガイド

このガイドでは、WebSocketゲームサーバーをGCP Cloud RunにArtifact Registry経由でデプロイする方法を説明します。

## 前提条件

1. **Google Cloud Platform プロジェクト**
   - 有効な課金アカウント
   - 必要なAPIの有効化権限

2. **ローカル環境**
   - Google Cloud SDK (gcloud) がインストールされている
   - Docker がインストールされている（オプション）
   - 適切な権限でGCPにログイン済み

## クイックスタート

### 1. 自動デプロイ（推奨）

```bash
# デプロイスクリプトを編集してPROJECT_IDを設定
vim deploy.sh

# PROJECT_ID を実際のプロジェクトIDに変更
PROJECT_ID="your-actual-project-id"

# デプロイ実行
./deploy.sh
```

### 2. 手動デプロイ

```bash
# 1. 必要な変数を設定
export PROJECT_ID="your-project-id"
export REGION="asia-northeast1"
export REPOSITORY="game-server-repo"
export IMAGE_NAME="online-game-server"
export SERVICE_NAME="online-game-server"

# 2. プロジェクト設定とAPIの有効化
gcloud config set project $PROJECT_ID
gcloud services enable run.googleapis.com artifactregistry.googleapis.com

# 3. Artifact Registry リポジトリを作成
gcloud artifacts repositories create $REPOSITORY \
    --repository-format=docker \
    --location=$REGION

# 4. Docker認証
gcloud auth configure-docker $REGION-docker.pkg.dev

# 5. ローカルビルド・プッシュ・デプロイ
IMAGE_TAG="$REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$IMAGE_NAME"
docker build -t "$IMAGE_TAG:latest" .
docker push "$IMAGE_TAG:latest"
gcloud run deploy $SERVICE_NAME \
    --image "$IMAGE_TAG:latest" \
    --region $REGION \
    --allow-unauthenticated \
    --port 8080
```

詳細なコマンドは [commands.md](commands.md) を参照してください。

## ファイル構成

- `Dockerfile` - マルチステージビルドでRustアプリをコンテナ化
- `deploy.sh` - 自動ローカルビルド・デプロイスクリプト
- `commands.md` - 手動デプロイ用のコマンド集
- `.dockerignore` - Dockerビルドから除外するファイル

## 設定のカスタマイズ

### Cloud Run の設定

`deploy.sh` またはコマンドラインで以下のパラメータを変更できます：

```bash
gcloud run deploy $SERVICE_NAME \
    --cpu 1 \              # CPU数
    --memory 512Mi \       # メモリサイズ
    --min-instances 0 \    # 最小インスタンス数
    --max-instances 10 \   # 最大インスタンス数
    --concurrency 100 \    # 同時接続数
    --timeout 300          # タイムアウト（秒）
```

### 環境変数

`deploy.sh` スクリプトの変数セクション：

```bash
PROJECT_ID="your-project-id"     # GCPプロジェクトID
REGION="asia-northeast1"         # デプロイリージョン
REPOSITORY="game-server-repo"    # Artifact Registry名
IMAGE_NAME="online-game-server"  # イメージ名
SERVICE_NAME="online-game-server"# サービス名
```

## 接続方法

デプロイ後、以下のようなURLが生成されます：

- **HTTPSエンドポイント**: `https://online-game-server-xxx-an.a.run.app`
- **WebSocketエンドポイント**: `wss://online-game-server-xxx-an.a.run.app`

クライアントコード（test_client.html）で接続URLを更新：

```javascript
// ローカル開発時
ws = new WebSocket('ws://127.0.0.1:8080');

// Cloud Run本番環境
ws = new WebSocket('wss://your-service-url.a.run.app');
```

## トラブルシューティング

### よくある問題

1. **認証エラー**
   ```bash
   gcloud auth login
   gcloud auth configure-docker asia-northeast1-docker.pkg.dev
   ```

2. **API未有効化**
   ```bash
   gcloud services enable cloudbuild.googleapis.com run.googleapis.com artifactregistry.googleapis.com
   ```

3. **権限不足**
   - プロジェクトオーナーまたは編集者権限が必要
   - Cloud Build Service Account に適切な権限を付与

### ログ確認

```bash
# Cloud Runログ
gcloud logs read --service=online-game-server

# Cloud Buildログ
gcloud builds list
gcloud builds log [BUILD_ID]
```

## パフォーマンス最適化

### CPU・メモリ設定
- WebSocket接続数に応じてリソースを調整
- 接続数が多い場合は `--cpu=2 --memory=1Gi` を検討

### インスタンス設定
- コールドスタートを避けたい場合は `--min-instances=1` を設定
- コスト重視の場合は `--min-instances=0`（デフォルト）

### ネットワーク設定
- WebSocketのタイムアウトは `--timeout=3600`（1時間）まで設定可能

## セキュリティ

### 認証
現在の設定では `--allow-unauthenticated` で誰でもアクセス可能です。  
本番環境では以下を検討：

```bash
# 認証を必要とする場合
gcloud run deploy online-game-server \
    --remove-flags=allow-unauthenticated
```

### HTTPS/WSS
Cloud Runは自動的にHTTPS/WSSを提供します。独自ドメインも設定可能です。

## コスト管理

- **無料利用枠**: 月200万リクエストまで無料
- **従量課金**: CPU使用時間とメモリ使用量に基づく
- **アイドル時間**: リクエストがない間は課金されません

## 監視・ログ

Cloud Consoleから以下を監視可能：

- リクエスト数・レスポンス時間
- エラー率
- インスタンス数
- リソース使用量

---

📖 詳細な情報は [Cloud Run公式ドキュメント](https://cloud.google.com/run/docs) を参照してください。