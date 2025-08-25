#!/bin/bash

# GCP Cloud Run ローカルビルド・デプロイスクリプト
# 使用前に以下の変数を環境に合わせて設定してください

set -e

# 設定変数
PROJECT_ID="your-project-id"  # GCPプロジェクトIDを設定
REGION="asia-northeast1"      # デプロイリージョン（東京）
REPOSITORY="game-server-repo" # Artifact Registry リポジトリ名
IMAGE_NAME="online-game-server"
SERVICE_NAME="online-game-server"

# カラー出力用
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Cloud Run ローカルビルド・デプロイスクリプトを開始します${NC}"

# 必要な変数の確認
if [ "$PROJECT_ID" = "your-project-id" ]; then
    echo -e "${RED}❌ PROJECT_IDを設定してください${NC}"
    exit 1
fi

# gcloudの認証確認
echo -e "${YELLOW}📋 gcloudの認証を確認中...${NC}"
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" | grep -q "@"; then
    echo -e "${RED}❌ gcloudにログインしてください: gcloud auth login${NC}"
    exit 1
fi

# Dockerの確認
echo -e "${YELLOW}🐳 Dockerの動作確認中...${NC}"
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}❌ Dockerが動作していません。Dockerを起動してください${NC}"
    exit 1
fi

# プロジェクト設定
echo -e "${YELLOW}🔧 プロジェクトを設定中...${NC}"
gcloud config set project $PROJECT_ID

# 必要なAPIを有効化
echo -e "${YELLOW}🔌 必要なAPIを有効化中...${NC}"
gcloud services enable run.googleapis.com
gcloud services enable artifactregistry.googleapis.com

# Artifact Registry リポジトリの作成（存在しない場合）
echo -e "${YELLOW}📦 Artifact Registry リポジトリを確認中...${NC}"
if ! gcloud artifacts repositories describe $REPOSITORY --location=$REGION &>/dev/null; then
    echo -e "${YELLOW}🆕 リポジトリを作成中...${NC}"
    gcloud artifacts repositories create $REPOSITORY \
        --repository-format=docker \
        --location=$REGION \
        --description="Game server Docker repository"
fi

# Docker認証設定
echo -e "${YELLOW}🔐 Docker認証を設定中...${NC}"
gcloud auth configure-docker $REGION-docker.pkg.dev

# イメージのタグ設定
IMAGE_TAG="$REGION-docker.pkg.dev/$PROJECT_ID/$REPOSITORY/$IMAGE_NAME"
GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "latest")

# Dockerイメージをビルド
echo -e "${YELLOW}🏗️ Dockerイメージをビルド中...${NC}"
docker build -t "$IMAGE_TAG:$GIT_SHA" -t "$IMAGE_TAG:latest" .

# Artifact Registryにプッシュ
echo -e "${YELLOW}📤 Artifact Registryにプッシュ中...${NC}"
docker push "$IMAGE_TAG:$GIT_SHA"
docker push "$IMAGE_TAG:latest"

# Cloud Runにデプロイ
echo -e "${YELLOW}🚀 Cloud Runにデプロイ中...${NC}"
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

# デプロイ結果の表示
echo -e "${GREEN}✅ デプロイが完了しました！${NC}"
echo ""

# サービスのURLを取得
SERVICE_URL=$(gcloud run services describe $SERVICE_NAME --region=$REGION --format="value(status.url)")
echo -e "${GREEN}🌐 サービスURL: ${SERVICE_URL}${NC}"

# WebSocketエンドポイントの表示
WEBSOCKET_URL=$(echo $SERVICE_URL | sed 's/https:/wss:/')
echo -e "${GREEN}🔌 WebSocketエンドポイント: ${WEBSOCKET_URL}${NC}"

echo ""
echo -e "${YELLOW}📝 クライアント接続時は以下のURLを使用してください:${NC}"
echo -e "${YELLOW}   ${WEBSOCKET_URL}${NC}"

echo ""
echo -e "${GREEN}🎉 ローカルビルド・デプロイ完了！${NC}"