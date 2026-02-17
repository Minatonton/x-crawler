package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SlackNotifier はSlack通知を送信
type SlackNotifier struct {
	webhookURL string
	username   string
	iconEmoji  string
	httpClient *http.Client
}

// NewSlackNotifier は新しいSlackNotifierを作成
func NewSlackNotifier(webhookURL, username, iconEmoji string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		username:   username,
		iconEmoji:  iconEmoji,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NotifyTweet はツイートをSlackに通知
func (s *SlackNotifier) NotifyTweet(ctx context.Context, tweet Tweet, analysis *AIAnalysis) error {
	message := s.buildMessage(tweet, analysis)

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// buildMessage はSlackメッセージを構築
func (s *SlackNotifier) buildMessage(tweet Tweet, analysis *AIAnalysis) map[string]interface{} {
	emoji := s.getEmojiByUrgency(analysis.Urgency)
	color := s.getColorByUrgency(analysis.Urgency)
	sentimentEmoji := s.getSentimentEmoji(analysis.Sentiment)

	// ティッカーリンクを生成
	tickerLinks := make([]string, len(analysis.Tickers))
	for i, ticker := range analysis.Tickers {
		tickerLinks[i] = fmt.Sprintf("<https://finance.yahoo.com/quote/%s|$%s>", ticker, ticker)
	}

	// フィールドを構築
	fields := []map[string]interface{}{
		{
			"title": "📝 AI分析サマリー",
			"value": analysis.Summary,
			"short": false,
		},
	}

	if analysis.Sentiment != "" {
		fields = append(fields, map[string]interface{}{
			"title": "💹 センチメント",
			"value": sentimentEmoji,
			"short": true,
		})
	}

	if len(tickerLinks) > 0 {
		fields = append(fields, map[string]interface{}{
			"title": "🎯 関連銘柄",
			"value": strings.Join(tickerLinks, ", "),
			"short": true,
		})
	}

	if len(analysis.KeyPoints) > 0 {
		points := "• " + strings.Join(analysis.KeyPoints, "\n• ")
		fields = append(fields, map[string]interface{}{
			"title": "📌 重要ポイント",
			"value": points,
			"short": false,
		})
	}

	// アタッチメントを構築
	attachment := map[string]interface{}{
		"color":       color,
		"author_name": fmt.Sprintf("@%s", tweet.Username),
		"title":       fmt.Sprintf("%s [%s] スコア: %d/100", emoji, analysis.Category, analysis.Score),
		"text":        tweet.Text,
		"fields":      fields,
		"footer":      "X Trading Crawler",
		"footer_icon": "https://abs.twimg.com/icons/apple-touch-icon-192x192.png",
		"ts":          tweet.CreatedAt.Unix(),
		"actions": []map[string]interface{}{
			{
				"type":  "button",
				"text":  "🔗 ポストを見る",
				"url":   fmt.Sprintf("https://x.com/%s/status/%s", tweet.Username, tweet.ID),
				"style": "primary",
			},
		},
	}

	// 最初のティッカーがある場合、チャートリンクを追加
	if len(analysis.Tickers) > 0 {
		attachment["actions"] = append(attachment["actions"].([]map[string]interface{}), map[string]interface{}{
			"type": "button",
			"text": "📊 チャート",
			"url":  fmt.Sprintf("https://www.tradingview.com/chart/?symbol=%s", analysis.Tickers[0]),
		})
	}

	return map[string]interface{}{
		"username":    s.username,
		"icon_emoji":  s.iconEmoji,
		"attachments": []map[string]interface{}{attachment},
	}
}

// NotifySimple はシンプルな通知（AI分析なし）
func (s *SlackNotifier) NotifySimple(ctx context.Context, tweet Tweet, traderInfo string) error {
	text := fmt.Sprintf("*@%s* さんの新しい投稿:\n%s\n\n🔗 <%s|ポストを見る>",
		tweet.Username,
		tweet.Text,
		fmt.Sprintf("https://x.com/%s/status/%s", tweet.Username, tweet.ID),
	)

	message := map[string]interface{}{
		"username":   s.username,
		"icon_emoji": s.iconEmoji,
		"text":       text,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// getEmojiByUrgency は緊急度に応じた絵文字を返す
func (s *SlackNotifier) getEmojiByUrgency(urgency string) string {
	switch urgency {
	case "critical":
		return "🚨"
	case "high":
		return "⚠️"
	case "normal":
		return "💡"
	case "low":
		return "ℹ️"
	default:
		return "💡"
	}
}

// getColorByUrgency は緊急度に応じた色を返す
func (s *SlackNotifier) getColorByUrgency(urgency string) string {
	switch urgency {
	case "critical":
		return "#FF0000" // 赤
	case "high":
		return "#FF9900" // オレンジ
	case "normal":
		return "#36A64F" // 緑
	case "low":
		return "#808080" // グレー
	default:
		return "#36A64F"
	}
}

// getSentimentEmoji はセンチメントに応じた絵文字を返す
func (s *SlackNotifier) getSentimentEmoji(sentiment string) string {
	switch sentiment {
	case "bullish":
		return "📈 強気"
	case "bearish":
		return "📉 弱気"
	case "neutral":
		return "➡️ 中立"
	default:
		return "❓ 不明"
	}
}
