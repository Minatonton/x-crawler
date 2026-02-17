package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// SeenTweets は既に通知済みのツイートIDを管理
type SeenTweets struct {
	mu       sync.RWMutex
	tweets   map[string]bool
	filePath string
}

// NewSeenTweets は新しいSeenTweetsを作成
func NewSeenTweets(filePath string) (*SeenTweets, error) {
	st := &SeenTweets{
		tweets:   make(map[string]bool),
		filePath: filePath,
	}

	// ファイルが存在する場合は読み込み
	if _, err := os.Stat(filePath); err == nil {
		if err := st.Load(); err != nil {
			return nil, err
		}
	}

	return st, nil
}

// Has は指定されたツイートIDが既に通知済みかチェック
func (st *SeenTweets) Has(tweetID string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.tweets[tweetID]
}

// Add は新しいツイートIDを追加
func (st *SeenTweets) Add(tweetID string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.tweets[tweetID] = true
}

// Save は既読ツイートをファイルに保存
func (st *SeenTweets) Save() error {
	st.mu.RLock()
	defer st.mu.RUnlock()

	data, err := json.MarshalIndent(st.tweets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal seen tweets: %w", err)
	}

	if err := os.WriteFile(st.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write seen tweets file: %w", err)
	}

	return nil
}

// Load は既読ツイートをファイルから読み込み
func (st *SeenTweets) Load() error {
	data, err := os.ReadFile(st.filePath)
	if err != nil {
		return fmt.Errorf("failed to read seen tweets file: %w", err)
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if err := json.Unmarshal(data, &st.tweets); err != nil {
		return fmt.Errorf("failed to unmarshal seen tweets: %w", err)
	}

	return nil
}

// Count は既読ツイート数を返す
func (st *SeenTweets) Count() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.tweets)
}
