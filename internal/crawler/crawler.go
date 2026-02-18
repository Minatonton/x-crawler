package crawler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Minatonton/x-crawler/internal/ai"
	"github.com/Minatonton/x-crawler/internal/config"
	"github.com/Minatonton/x-crawler/internal/slack"
	"github.com/Minatonton/x-crawler/internal/storage"
	"github.com/Minatonton/x-crawler/internal/twitter"
)

// Crawler はクロール処理を実行
type Crawler struct {
	config        *config.Config
	twitterClient *twitter.Client
	aiFilter      *ai.Filter
	slackNotifier *slack.Notifier
	seenTweets    *storage.SeenTweets
}

// New は新しいCrawlerを作成
func New(
	cfg *config.Config,
	twitterClient *twitter.Client,
	aiFilter *ai.Filter,
	slackNotifier *slack.Notifier,
	seenTweets *storage.SeenTweets,
) *Crawler {
	return &Crawler{
		config:        cfg,
		twitterClient: twitterClient,
		aiFilter:      aiFilter,
		slackNotifier: slackNotifier,
		seenTweets:    seenTweets,
	}
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
func (c *Crawler) processTrader(ctx context.Context, trader config.Trader) (processed, notified int, err error) {
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
func (c *Crawler) processKeyword(ctx context.Context, keyword config.Keyword) (processed, notified int, err error) {
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
