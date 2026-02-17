# X-Crawler for Trading

X (Twitter) のポストをクロールして、有名トレーダーの投稿や株価関連情報をSlackに通知するアプリケーション。

## 特徴

- 🎯 **トレーディング特化**: 有名トレーダーや株価関連キーワードを監視
- 🤖 **AI分析**: Claude APIで投稿の重要度を自動判定
- 📱 **Slack通知**: 重要な情報をSlackにリアルタイム通知
- 🚀 **シンプル**: DBレス設計で簡単にデプロイ可能

## 必要な準備

### 1. X (Twitter) API

[X Developer Portal](https://developer.twitter.com/) でアカウントを作成し、API v2のBearer Tokenを取得

### 2. Claude API (オプション)

[Anthropic Console](https://console.anthropic.com/) でAPIキーを取得

### 3. Slack Webhook

Slack Appを作成し、Incoming Webhookを有効化

## セットアップ

### 1. 環境変数の設定

```bash
cp .env.example .env
```

`.env` ファイルを編集:

```bash
X_API_BEARER_TOKEN=your_twitter_bearer_token
ANTHROPIC_API_KEY=your_anthropic_api_key
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### 2. 設定ファイルの作成

```bash
cp config.yaml.example config.yaml
```

`config.yaml` で監視対象のトレーダーやキーワードを設定

### 3. ビルド & 実行

```bash
# ビルド
go build -o x-crawler

# 実行
./x-crawler
```

## 設定例

```yaml
# 実行間隔
interval: "5m"

# AI分析の設定
ai:
  enabled: true
  min_score: 70

# 監視するトレーダー
traders:
  - username: "DeItaone"
    priority: "critical"
  - username: "zerohedge"
    priority: "high"
  - username: "cathiedwood"
    priority: "high"

# 監視するキーワード
keywords:
  - query: "$SPY OR $QQQ"
    name: "主要ETF"
  - query: "($AAPL OR $MSFT) earnings"
    name: "FAANG決算"

# Slack通知設定
slack:
  webhook_url: "${SLACK_WEBHOOK_URL}"
  channel: "#trading-alerts"
```

## デプロイ (GCE)

```bash
# ビルド
GOOS=linux GOARCH=amd64 go build -o x-crawler

# GCEにアップロード
gcloud compute scp x-crawler your-instance:~/
gcloud compute scp config.yaml your-instance:~/
gcloud compute scp .env your-instance:~/

# SSH接続
gcloud compute ssh your-instance

# systemdサービス化 (オプション)
sudo cp x-crawler.service /etc/systemd/system/
sudo systemctl enable x-crawler
sudo systemctl start x-crawler
```

## ログ確認

```bash
# 標準出力にログが表示されます
./x-crawler

# systemdの場合
sudo journalctl -u x-crawler -f
```

## License

MIT
