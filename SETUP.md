# X-Crawler セットアップガイド

## 📋 必要なもの

### 1. X (Twitter) API トークン

1. [X Developer Portal](https://developer.twitter.com/) にアクセス
2. "Create Project" をクリック
3. プロジェクトとアプリを作成
4. "Keys and tokens" タブから **Bearer Token** を取得

**必要なアクセスレベル:** Read（読み取り専用でOK）

### 2. Claude API キー（オプション）

1. [Anthropic Console](https://console.anthropic.com/) にアクセス
2. "API Keys" から新しいキーを作成
3. キーをコピー

**料金:** 従量課金（1ツイートあたり約$0.01-0.02）

### 3. Slack Webhook URL

#### 方法1: Incoming Webhooks（簡単）

1. [Slack API](https://api.slack.com/apps) にアクセス
2. "Create New App" → "From scratch"
3. アプリ名とワークスペースを選択
4. "Incoming Webhooks" を有効化
5. "Add New Webhook to Workspace" をクリック
6. 通知先チャンネルを選択
7. Webhook URLをコピー

#### 方法2: Slack Bot（高度な機能）

詳細は[Slack APIドキュメント](https://api.slack.com/messaging/webhooks)を参照

---

## 🚀 クイックスタート

### 1. リポジトリをクローン

```bash
git clone https://github.com/nagaseitteam/x-crawler.git
cd x-crawler
```

### 2. 環境変数を設定

```bash
cp .env.example .env
```

`.env` を編集:

```bash
X_API_BEARER_TOKEN=your_actual_bearer_token_here
ANTHROPIC_API_KEY=your_actual_api_key_here  # オプション
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### 3. 設定ファイルを作成

```bash
cp config.yaml.example config.yaml
```

`config.yaml` を編集して、監視したいトレーダーやキーワードを設定:

```yaml
interval: "5m"  # 5分ごとにクロール

ai:
  enabled: true
  min_score: 70  # 70点以上のツイートのみ通知

traders:
  - username: "DeItaone"
    display_name: "DeItaone (Market News)"
    priority: "critical"

keywords:
  - query: "$SPY OR $QQQ -is:retweet lang:en"
    name: "主要ETF"
```

### 4. ビルド & 実行

```bash
# ビルド
go build -o x-crawler

# 実行
./x-crawler

# または
make run
```

---

## 🖥️ GCEにデプロイ

### 1. GCEインスタンスを作成

```bash
gcloud compute instances create x-crawler \
  --machine-type=e2-micro \
  --zone=us-central1-a \
  --image-family=ubuntu-2204-lts \
  --image-project=ubuntu-os-cloud
```

### 2. ファイルをアップロード

```bash
# ローカルでLinuxバイナリをビルド
make build-linux

# ファイルをアップロード
gcloud compute scp x-crawler-linux x-crawler:~/x-crawler
gcloud compute scp config.yaml x-crawler:~/
gcloud compute scp .env x-crawler:~/
gcloud compute scp x-crawler.service x-crawler:~/
```

### 3. SSHで接続してセットアップ

```bash
gcloud compute ssh x-crawler

# ディレクトリ作成
mkdir -p ~/x-crawler-app
mv x-crawler ~/x-crawler-app/
mv config.yaml ~/x-crawler-app/
mv .env ~/x-crawler-app/
cd ~/x-crawler-app

# 実行権限を付与
chmod +x x-crawler

# テスト実行
./x-crawler
```

### 4. systemdサービス化（自動起動）

```bash
# サービスファイルを編集（パスとユーザー名を確認）
nano x-crawler.service

# サービスをインストール
sudo cp x-crawler /usr/local/bin/
sudo cp x-crawler.service /etc/systemd/system/

# サービスを有効化
sudo systemctl daemon-reload
sudo systemctl enable x-crawler
sudo systemctl start x-crawler

# ステータス確認
sudo systemctl status x-crawler

# ログ確認
sudo journalctl -u x-crawler -f
```

---

## 🔧 設定のカスタマイズ

### AI分析を無効にする（高速化）

```yaml
ai:
  enabled: false
```

AI分析なしだと通知は速くなりますが、全てのツイートが通知されます。

### 実行間隔を変更

```yaml
interval: "2m"   # 2分ごと
interval: "10m"  # 10分ごと
interval: "1h"   # 1時間ごと
```

### 監視対象を追加

```yaml
traders:
  - username: "new_trader"
    display_name: "New Trader"
    priority: "high"

keywords:
  - query: "$AAPL earnings -is:retweet"
    name: "Apple決算"
```

---

## 📊 ログとモニタリング

### ログを見る

```bash
# systemdの場合
sudo journalctl -u x-crawler -f

# 直接実行の場合
./x-crawler 2>&1 | tee x-crawler.log
```

### 既読ツイート数を確認

```bash
cat seen_tweets.json | jq 'length'
```

### プロセスを確認

```bash
ps aux | grep x-crawler
```

---

## 🐛 トラブルシューティング

### "Twitter API error (status 429)"

**原因:** レート制限に達しました

**解決策:**
- `interval` を長くする（例: "10m"）
- 監視対象を減らす

### "Claude API error (status 401)"

**原因:** APIキーが無効

**解決策:**
- `.env` の `ANTHROPIC_API_KEY` を確認
- [Anthropic Console](https://console.anthropic.com/)でキーを再生成

### "Slack webhook returned status 404"

**原因:** Webhook URLが無効

**解決策:**
- `.env` の `SLACK_WEBHOOK_URL` を確認
- Slack Appの設定で新しいWebhookを作成

### 同じツイートが何度も通知される

**原因:** `seen_tweets.json` が保存されていない

**解決策:**
- ファイルの書き込み権限を確認
- 手動で空のJSONファイルを作成: `echo '{}' > seen_tweets.json`

---

## 💡 ヒント

### コストを抑える

1. **AI分析を無効化** → Claude APIコストがゼロに
2. **実行間隔を長く** → API呼び出し回数が減る
3. **GCE e2-micro** → 無料枠で動作可能

### 精度を上げる

1. **min_score を調整** → 高くすると通知が厳選される
2. **キーワードを具体的に** → ノイズが減る
3. **リツイートを除外** → `-is:retweet` を追加

### 安定性を高める

1. **systemd で自動起動** → 再起動時も自動で起動
2. **ログローテーション** → ログが肥大化しない
3. **定期的にバックアップ** → `seen_tweets.json` を保存

---

## 📞 サポート

問題が発生した場合:

1. [GitHub Issues](https://github.com/nagaseitteam/x-crawler/issues) で報告
2. ログを添付して質問

---

## 📚 参考リンク

- [X API Documentation](https://developer.twitter.com/en/docs/twitter-api)
- [Claude API Documentation](https://docs.anthropic.com/claude/reference/getting-started-with-the-api)
- [Slack Webhooks](https://api.slack.com/messaging/webhooks)
