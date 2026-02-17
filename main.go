package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultConfigPath     = "config.yaml"
	defaultSeenTweetsPath = "seen_tweets.json"
)

func main() {
	// フラグ解析
	configPath := flag.String("config", defaultConfigPath, "設定ファイルのパス")
	seenTweetsPath := flag.String("seen", defaultSeenTweetsPath, "既読ツイートファイルのパス")
	flag.Parse()

	// .envファイルを読み込み（存在する場合）
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// 設定を読み込み
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ログレベルを設定
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting X-Crawler for Trading (interval: %s)", config.Interval)

	// 環境変数をチェック
	xAPIToken := os.Getenv("X_API_BEARER_TOKEN")
	if xAPIToken == "" {
		log.Fatal("X_API_BEARER_TOKEN environment variable is required")
	}

	slackWebhookURL := config.Slack.WebhookURL
	if slackWebhookURL == "" {
		slackWebhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}
	if slackWebhookURL == "" {
		log.Fatal("SLACK_WEBHOOK_URL is required (in config or environment variable)")
	}

	// 既読ツイート管理を初期化
	seenTweets, err := NewSeenTweets(*seenTweetsPath)
	if err != nil {
		log.Fatalf("Failed to initialize seen tweets: %v", err)
	}
	log.Printf("Loaded %d seen tweets from %s", seenTweets.Count(), *seenTweetsPath)

	// クライアントを初期化
	twitterClient := NewTwitterClient(xAPIToken)
	slackNotifier := NewSlackNotifier(slackWebhookURL, config.Slack.Username, config.Slack.IconEmoji)

	var aiFilter *AIFilter
	if config.AI.Enabled {
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			log.Println("Warning: AI filter is enabled but ANTHROPIC_API_KEY is not set. AI analysis will be skipped.")
		} else {
			aiFilter = NewAIFilter(apiKey, config.AI.Model)
			log.Printf("AI filter enabled (model: %s, min_score: %d)", config.AI.Model, config.AI.MinScore)
		}
	}

	// クローラーを作成
	crawler := &Crawler{
		config:        config,
		twitterClient: twitterClient,
		aiFilter:      aiFilter,
		slackNotifier: slackNotifier,
		seenTweets:    seenTweets,
	}

	// 実行間隔を取得
	interval, err := config.GetInterval()
	if err != nil {
		log.Fatalf("Invalid interval: %v", err)
	}

	// 初回実行
	log.Println("Running initial crawl...")
	if err := crawler.Run(context.Background()); err != nil {
		log.Printf("Error during initial crawl: %v", err)
	}

	// 定期実行
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// シグナルハンドリング
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Crawler started. Press Ctrl+C to stop.")

	for {
		select {
		case <-ticker.C:
			log.Println("Running scheduled crawl...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if err := crawler.Run(ctx); err != nil {
				log.Printf("Error during crawl: %v", err)
			}
			cancel()

		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			// 既読ツイートを保存
			if err := seenTweets.Save(); err != nil {
				log.Printf("Failed to save seen tweets: %v", err)
			}
			log.Println("Shutdown complete")
			return
		}
	}
}

// Crawler はクロール処理を実行
type Crawler struct {
	config        *Config
	twitterClient *TwitterClient
	aiFilter      *AIFilter
	slackNotifier *SlackNotifier
	seenTweets    *SeenTweets
}

// Run はクロール処理を実行
func (c *Crawler) Run(ctx context.Context) error {
	totalProcessed := 0
	totalNotified := 0

	// トレーダーのツイートを取得
	for _, trader := range c.config.Traders {
		processed, notified, err := c.processTrader(ctx, trader)
		if err != nil {
			log.Printf("Error processing trader @%s: %v", trader.Username, err)
			continue
		}
		totalProcessed += processed
		totalNotified += notified
	}

	// キーワード検索
	for _, keyword := range c.config.Keywords {
		processed, notified, err := c.processKeyword(ctx, keyword)
		if err != nil {
			log.Printf("Error processing keyword '%s': %v", keyword.Name, err)
			continue
		}
		totalProcessed += processed
		totalNotified += notified
	}

	// 既読ツイートを保存
	if err := c.seenTweets.Save(); err != nil {
		log.Printf("Failed to save seen tweets: %v", err)
	}

	log.Printf("Crawl complete: processed=%d, notified=%d, total_seen=%d",
		totalProcessed, totalNotified, c.seenTweets.Count())

	return nil
}

// processTrader はトレーダーのツイートを処理
func (c *Crawler) processTrader(ctx context.Context, trader Trader) (processed, notified int, err error) {
	tweets, err := c.twitterClient.GetUserTweets(ctx, trader.Username, 10)
	if err != nil {
		return 0, 0, err
	}

	traderInfo := fmt.Sprintf("%s (Priority: %s)", trader.DisplayName, trader.Priority)

	for _, tweet := range tweets {
		// 既読チェック
		if c.seenTweets.Has(tweet.ID) {
			continue
		}

		processed++

		// AI分析（有効な場合）
		if c.aiFilter != nil {
			analysis, err := c.aiFilter.Analyze(ctx, tweet, traderInfo)
			if err != nil {
				log.Printf("AI analysis failed for tweet %s: %v", tweet.ID, err)
				// AI分析失敗時はシンプル通知にフォールバック
				if err := c.slackNotifier.NotifySimple(ctx, tweet, traderInfo); err != nil {
					log.Printf("Failed to send simple notification: %v", err)
					continue
				}
			} else {
				// スコアチェック
				if analysis.Score < c.config.AI.MinScore {
					log.Printf("Tweet %s score too low: %d < %d", tweet.ID, analysis.Score, c.config.AI.MinScore)
					c.seenTweets.Add(tweet.ID)
					continue
				}

				// Slack通知
				if err := c.slackNotifier.NotifyTweet(ctx, tweet, analysis); err != nil {
					log.Printf("Failed to notify tweet %s: %v", tweet.ID, err)
					continue
				}

				log.Printf("Notified: @%s - Score: %d, Category: %s, Sentiment: %s",
					tweet.Username, analysis.Score, analysis.Category, analysis.Sentiment)
			}
		} else {
			// AI分析なしでシンプル通知
			if err := c.slackNotifier.NotifySimple(ctx, tweet, traderInfo); err != nil {
				log.Printf("Failed to notify tweet %s: %v", tweet.ID, err)
				continue
			}
			log.Printf("Notified (no AI): @%s", tweet.Username)
		}

		c.seenTweets.Add(tweet.ID)
		notified++

		// レート制限対策: 少し待機
		time.Sleep(500 * time.Millisecond)
	}

	return processed, notified, nil
}

// processKeyword はキーワード検索を処理
func (c *Crawler) processKeyword(ctx context.Context, keyword Keyword) (processed, notified int, err error) {
	tweets, err := c.twitterClient.SearchTweets(ctx, keyword.Query, 10)
	if err != nil {
		return 0, 0, err
	}

	for _, tweet := range tweets {
		// 既読チェック
		if c.seenTweets.Has(tweet.ID) {
			continue
		}

		processed++

		keywordInfo := fmt.Sprintf("Keyword: %s", keyword.Name)

		// AI分析（有効な場合）
		if c.aiFilter != nil {
			analysis, err := c.aiFilter.Analyze(ctx, tweet, keywordInfo)
			if err != nil {
				log.Printf("AI analysis failed for tweet %s: %v", tweet.ID, err)
				if err := c.slackNotifier.NotifySimple(ctx, tweet, keywordInfo); err != nil {
					log.Printf("Failed to send simple notification: %v", err)
					continue
				}
			} else {
				// スコアチェック
				if analysis.Score < c.config.AI.MinScore {
					log.Printf("Tweet %s score too low: %d < %d", tweet.ID, analysis.Score, c.config.AI.MinScore)
					c.seenTweets.Add(tweet.ID)
					continue
				}

				// Slack通知
				if err := c.slackNotifier.NotifyTweet(ctx, tweet, analysis); err != nil {
					log.Printf("Failed to notify tweet %s: %v", tweet.ID, err)
					continue
				}

				log.Printf("Notified (keyword): @%s - Score: %d, Category: %s",
					tweet.Username, analysis.Score, analysis.Category)
			}
		} else {
			// AI分析なしでシンプル通知
			if err := c.slackNotifier.NotifySimple(ctx, tweet, keywordInfo); err != nil {
				log.Printf("Failed to notify tweet %s: %v", tweet.ID, err)
				continue
			}
			log.Printf("Notified (keyword, no AI): @%s", tweet.Username)
		}

		c.seenTweets.Add(tweet.ID)
		notified++

		// レート制限対策: 少し待機
		time.Sleep(500 * time.Millisecond)
	}

	return processed, notified, nil
}
