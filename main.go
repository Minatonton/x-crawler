package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Minatonton/x-crawler/internal/ai"
	"github.com/Minatonton/x-crawler/internal/config"
	"github.com/Minatonton/x-crawler/internal/crawler"
	"github.com/Minatonton/x-crawler/internal/slack"
	"github.com/Minatonton/x-crawler/internal/storage"
	"github.com/Minatonton/x-crawler/internal/twitter"
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
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ログレベルを設定
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting X-Crawler for Trading (interval: %s)", cfg.Interval)

	// 環境変数をチェック
	xAPIToken := os.Getenv("X_API_BEARER_TOKEN")
	if xAPIToken == "" {
		log.Fatal("X_API_BEARER_TOKEN environment variable is required")
	}

	slackWebhookURL := cfg.Slack.WebhookURL
	if slackWebhookURL == "" {
		slackWebhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}
	if slackWebhookURL == "" {
		log.Fatal("SLACK_WEBHOOK_URL is required (in config or environment variable)")
	}

	// 既読ツイート管理を初期化
	seenTweets, err := storage.NewSeenTweets(*seenTweetsPath)
	if err != nil {
		log.Fatalf("Failed to initialize seen tweets: %v", err)
	}
	log.Printf("Loaded %d seen tweets from %s", seenTweets.Count(), *seenTweetsPath)

	// クライアントを初期化
	twitterClient := twitter.NewClient(xAPIToken)
	slackNotifier := slack.NewNotifier(slackWebhookURL, cfg.Slack.Username, cfg.Slack.IconEmoji)

	var aiFilter *ai.Filter
	if cfg.AI.Enabled {
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			log.Println("Warning: AI filter is enabled but ANTHROPIC_API_KEY is not set. AI analysis will be skipped.")
		} else {
			aiFilter = ai.NewFilter(apiKey, cfg.AI.Model)
			log.Printf("AI filter enabled (model: %s, min_score: %d)", cfg.AI.Model, cfg.AI.MinScore)
		}
	}

	// クローラーを作成
	crawlerInstance := crawler.New(cfg, twitterClient, aiFilter, slackNotifier, seenTweets)

	// 実行間隔を取得
	interval, err := cfg.GetInterval()
	if err != nil {
		log.Fatalf("Invalid interval: %v", err)
	}

	// 初回実行
	log.Println("Running initial crawl...")
	if err := crawlerInstance.Run(context.Background()); err != nil {
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
			if err := crawlerInstance.Run(ctx); err != nil {
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
