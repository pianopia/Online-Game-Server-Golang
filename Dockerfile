# マルチステージビルドでRustアプリケーションを最適化
FROM rust:1.75-slim as builder

# 必要なパッケージをインストール
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# 作業ディレクトリを設定
WORKDIR /app

# 依存関係のキャッシュ最適化のため、まずCargo.tomlをコピー
COPY Cargo.toml Cargo.lock ./

# ダミーのsrc/main.rsを作成して依存関係をビルド
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build --release
RUN rm src/main.rs

# 実際のソースコードをコピー
COPY src ./src

# アプリケーションをビルド
RUN cargo build --release

# ランタイムステージ
FROM debian:bookworm-slim

# 必要なランタイム依存関係をインストール
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libssl3 \
    && rm -rf /var/lib/apt/lists/*

# 非rootユーザーを作成
RUN useradd --create-home --shell /bin/bash app

# 作業ディレクトリを設定
WORKDIR /app

# ビルドステージから実行可能ファイルをコピー
COPY --from=builder /app/target/release/online-game-server /app/online-game-server

# ファイルの所有権をappユーザーに変更
RUN chown -R app:app /app

# 非rootユーザーに切り替え
USER app

# Cloud Runは環境変数PORTでポートを指定するため、それを利用
# デフォルトは8080
ENV PORT=8080

# ポートを公開
EXPOSE $PORT

# ヘルスチェック用のエンドポイント（オプション）
# Cloud Runは自動的にヘルスチェックを行うため、通常は不要

# アプリケーションを実行
CMD ["./online-game-server"]