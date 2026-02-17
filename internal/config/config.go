package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config はアプリケーション全体の設定
type Config struct {
	Interval string      `yaml:"interval"`
	AI       AIConfig    `yaml:"ai"`
	Traders  []Trader    `yaml:"traders"`
	Keywords []Keyword   `yaml:"keywords"`
	Slack    SlackConfig `yaml:"slack"`
	Log      LogConfig   `yaml:"log"`
}

// AIConfig はAI分析の設定
type AIConfig struct {
	Enabled  bool   `yaml:"enabled"`
	MinScore int    `yaml:"min_score"`
	Model    string `yaml:"model"`
}

// Trader は監視対象のトレーダー
type Trader struct {
	Username    string `yaml:"username"`
	DisplayName string `yaml:"display_name"`
	Priority    string `yaml:"priority"` // critical, high, normal, low
}

// Keyword は監視対象のキーワード
type Keyword struct {
	Query string `yaml:"query"`
	Name  string `yaml:"name"`
}

// SlackConfig はSlack通知の設定
type SlackConfig struct {
	WebhookURL string `yaml:"webhook_url"`
	Username   string `yaml:"username"`
	IconEmoji  string `yaml:"icon_emoji"`
}

// LogConfig はログの設定
type LogConfig struct {
	Level string `yaml:"level"` // debug, info, warn, error
}

// Load は設定ファイルを読み込む
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 環境変数を展開
	content := os.ExpandEnv(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// デフォルト値の設定
	if config.Interval == "" {
		config.Interval = "5m"
	}
	if config.AI.MinScore == 0 {
		config.AI.MinScore = 70
	}
	if config.AI.Model == "" {
		config.AI.Model = "claude-3-5-sonnet-20241022"
	}
	if config.Slack.Username == "" {
		config.Slack.Username = "X Trading Bot"
	}
	if config.Slack.IconEmoji == "" {
		config.Slack.IconEmoji = ":chart_with_upwards_trend:"
	}
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}

	return &config, nil
}

// GetInterval は設定された間隔をtime.Durationとして返す
func (c *Config) GetInterval() (time.Duration, error) {
	return time.ParseDuration(c.Interval)
}

// GetPriorityScore は優先度をスコアに変換
func (t *Trader) GetPriorityScore() int {
	switch strings.ToLower(t.Priority) {
	case "critical":
		return 100
	case "high":
		return 80
	case "normal":
		return 60
	case "low":
		return 40
	default:
		return 60
	}
}
