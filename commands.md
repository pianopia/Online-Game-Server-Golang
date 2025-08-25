# GCP Cloud Run デプロイコマンド集

## 必要な準備

```bash
# 1. 変数設定
export PROJECT_ID="your-project-id"
export REGION="asia-northeast1"
export REPOSITORY="game-server-repo"
export IMAGE_NAME="online-game-server"
export SERVICE_NAME="online-game-server"

# 2. gcloudログイン
gcloud auth login
gcloud config set project $PROJECT_ID

# 3. Docker認証
gcloud auth configure-docker $REGION-docker.pkg.dev
```

## 手動ビルド・プッシュ・デプロイコマンド

### 1. Artifact Registry リポジトリ作成

```bash
# リポジトリ作成
gcloud artifacts repositories create $REPOSITORY \
    --repository-format=docker \
    --location=$REGION \
    --description="Game server Docker repository"

# 作成確認
gcloud artifacts repositories list --location=$REGION
```

### 2. Dockerイメージのビルド

```bash
# イメージタグ設定
IMAGE_TAG="$REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$IMAGE_NAME"
GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "latest")

# ビルド実行
docker build -t "$IMAGE_TAG:$GIT_SHA" -t "$IMAGE_TAG:latest" .

# ビルド確認
docker images | grep $IMAGE_NAME
```

### 3. Artifact Registryにプッシュ

```bash
# プッシュ実行
docker push "$IMAGE_TAG:$GIT_SHA"
docker push "$IMAGE_TAG:latest"

# プッシュ確認
gcloud artifacts docker images list $REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$IMAGE_NAME
```

### 4. Cloud Runにデプロイ

```bash
# デプロイ実行
gcloud run deploy $SERVICE_NAME \
    --image "$IMAGE_TAG:$GIT_SHA" \
    --region $REGION \
    --platform managed \
    --allow-unauthenticated \
    --port 8080 \
    --cpu 1 \
    --memory 512Mi \
    --min-instances 0 \
    --max-instances 10 \
    --concurrency 100 \
    --timeout 300 \
    --set-env-vars RUST_LOG=info

# デプロイ確認
gcloud run services list --region=$REGION
```

## 自動デプロイ（推奨）

```bash
# deploy.shを編集してPROJECT_IDを設定
vim deploy.sh

# 実行
./deploy.sh
```

## 管理コマンド

### サービス確認

```bash
# サービス一覧
gcloud run services list --region=$REGION

# サービス詳細
gcloud run services describe $SERVICE_NAME --region=$REGION

# サービスURL取得
gcloud run services describe $SERVICE_NAME --region=$REGION --format="value(status.url)"
```

### ログ確認

```bash
# リアルタイムログ
gcloud logs tail --service=$SERVICE_NAME

# 過去のログ
gcloud logs read --service=$SERVICE_NAME --limit=100
```

### トラフィック設定

```bash
# 特定のリビジョンに100%トラフィック
gcloud run services update-traffic $SERVICE_NAME \
    --to-revisions=REVISION_NAME=100 \
    --region=$REGION

# 最新リビジョンに100%トラフィック  
gcloud run services update-traffic $SERVICE_NAME \
    --to-latest \
    --region=$REGION
```

### サービス削除

```bash
# Cloud Runサービス削除
gcloud run services delete $SERVICE_NAME --region=$REGION

# Artifact Registryイメージ削除
gcloud artifacts docker images delete $IMAGE_TAG:latest --quiet
gcloud artifacts docker images delete $IMAGE_TAG:$GIT_SHA --quiet

# リポジトリ削除
gcloud artifacts repositories delete $REPOSITORY --location=$REGION
```

## ローカル開発

```bash
# ローカルでDockerコンテナ実行
docker run -p 8080:8080 "$IMAGE_TAG:latest"

# ローカルでRustアプリを直接実行
cargo run
```

## トラブルシューティング

### 認証エラー
```bash
gcloud auth login
gcloud auth configure-docker $REGION-docker.pkg.dev
```

### 権限エラー
```bash
# 現在の権限確認
gcloud iam service-accounts list
gcloud projects get-iam-policy $PROJECT_ID

# 必要な権限
# - Cloud Run Admin
# - Artifact Registry Administrator  
# - Service Account User
```

### リソース不足エラー
```bash
# より多くのリソースでデプロイ
gcloud run deploy $SERVICE_NAME \
    --image "$IMAGE_TAG:$GIT_SHA" \
    --region $REGION \
    --cpu 2 \
    --memory 1Gi \
    --timeout 900
```

## 環境別設定例

### 開発環境
```bash
gcloud run deploy $SERVICE_NAME-dev \
    --image "$IMAGE_TAG:latest" \
    --region $REGION \
    --cpu 1 \
    --memory 512Mi \
    --min-instances 0 \
    --max-instances 3 \
    --set-env-vars RUST_LOG=debug
```

### 本番環境
```bash
gcloud run deploy $SERVICE_NAME-prod \
    --image "$IMAGE_TAG:$GIT_SHA" \
    --region $REGION \
    --cpu 2 \
    --memory 1Gi \
    --min-instances 1 \
    --max-instances 100 \
    --set-env-vars RUST_LOG=info
```

## コスト最適化

```bash
# 最小構成（コスト重視）
gcloud run deploy $SERVICE_NAME \
    --image "$IMAGE_TAG:$GIT_SHA" \
    --region $REGION \
    --cpu 1 \
    --memory 128Mi \
    --min-instances 0 \
    --max-instances 5 \
    --concurrency 1000

# パフォーマンス重視
gcloud run deploy $SERVICE_NAME \
    --image "$IMAGE_TAG:$GIT_SHA" \
    --region $REGION \
    --cpu 4 \
    --memory 2Gi \
    --min-instances 2 \
    --max-instances 50 \
    --concurrency 50
```