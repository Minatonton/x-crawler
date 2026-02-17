package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Minatonton/x-crawler/internal/twitter"
)

// Filter はClaude APIを使った分析フィルター
type Filter struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// Analysis はAI分析結果
type Analysis struct {
	Score     int      `json:"score"`
	Category  string   `json:"category"`
	Sentiment string   `json:"sentiment"`
	Tickers   []string `json:"tickers"`
	Summary   string   `json:"summary"`
	KeyPoints []string `json:"key_points"`
	Urgency   string   `json:"urgency"`
	Reasoning string   `json:"reasoning"`
}

// NewFilter は新しいAIフィルターを作成
func NewFilter(apiKey, model string) *Filter {
	return &Filter{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Analyze はツイートを分析
func (f *Filter) Analyze(ctx context.Context, tweet twitter.Tweet, traderInfo string) (*Analysis, error) {
	prompt := f.buildPrompt(tweet, traderInfo)

	analysis, err := f.callClaudeAPI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return analysis, nil
}

// buildPrompt はAI分析用のプロンプトを構築
func (f *Filter) buildPrompt(tweet twitter.Tweet, traderInfo string) string {
	return fmt.Sprintf(`あなたは経験豊富な金融アナリストです。以下のXポストを分析してください。

投稿者: @%s
投稿者情報: %s
投稿時刻: %s
内容:
%s

以下の形式でJSONを返してください:
{
  "score": 0-100,
  "category": "buy_signal|sell_signal|earnings_beat|earnings_miss|sec_filing|merger_acquisition|analyst_upgrade|analyst_downgrade|market_news|executive_trade|other",
  "sentiment": "bullish|bearish|neutral",
  "tickers": ["AAPL", "TSLA"],
  "summary": "簡潔な日本語サマリー (1-2行)",
  "key_points": ["ポイント1", "ポイント2"],
  "urgency": "critical|high|normal|low",
  "reasoning": "スコアの理由"
}

評価基準:
1. 投稿者の信頼性と影響力
2. 情報の具体性 (数値、ティッカーシンボル、価格目標)
3. 時間的価値 (速報性、タイムリー性)
4. アクション可能性 (すぐに取引判断に使えるか)
5. 情報源の信頼性 (一次情報か)

高スコア例 (80-100):
- 決算発表の速報
- SEC提出書類の通知
- 有名投資家の売買報告
- M&A発表
- 大口取引の検出

中スコア例 (60-79):
- アナリストレポート
- 市場コメンタリー
- 業界ニュース

低スコア例 (0-59):
- 一般的な市場コメント
- 個人的な意見
- 既知の情報`,
		tweet.Username,
		traderInfo,
		tweet.CreatedAt.Format("2006-01-02 15:04:05 MST"),
		tweet.Text,
	)
}

// callClaudeAPI はClaude APIを呼び出し
func (f *Filter) callClaudeAPI(ctx context.Context, prompt string) (*Analysis, error) {
	requestBody := map[string]interface{}{
		"model":       f.model,
		"max_tokens":  2048,
		"temperature": 0.2,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", f.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Claude API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return nil, err
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude API")
	}

	// JSONレスポンスをパース
	var analysis Analysis
	text := claudeResp.Content[0].Text

	// JSONブロックを抽出（```json ... ```のような形式に対応）
	text = extractJSON(text)

	if err := json.Unmarshal([]byte(text), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w (response: %s)", err, text)
	}

	return &analysis, nil
}

// extractJSON はマークダウンのコードブロックからJSONを抽出
func extractJSON(text string) string {
	// ```json ... ``` の形式を探す
	start := -1
	end := -1

	for i := 0; i < len(text)-6; i++ {
		if text[i:i+7] == "```json" {
			start = i + 7
		} else if text[i:i+3] == "```" && start != -1 {
			end = i
			break
		}
	}

	if start != -1 && end != -1 {
		return text[start:end]
	}

	// JSONブロックが見つからない場合は、{}で囲まれた部分を探す
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			// 最後の}を探す
			for j := len(text) - 1; j > i; j-- {
				if text[j] == '}' {
					return text[i : j+1]
				}
			}
		}
	}

	return text
}
