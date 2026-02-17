package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client はX (Twitter) APIクライアント
type Client struct {
	bearerToken string
	httpClient  *http.Client
}

// Tweet はツイート情報
type Tweet struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
	Username  string    // APIレスポンスには含まれないが後で設定
}

// Response はTwitter API v2のレスポンス
type Response struct {
	Data     []Tweet           `json:"data"`
	Includes *ResponseIncludes `json:"includes,omitempty"`
	Meta     *ResponseMeta     `json:"meta,omitempty"`
}

// ResponseIncludes はユーザー情報など
type ResponseIncludes struct {
	Users []User `json:"users"`
}

// User はユーザー情報
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// ResponseMeta はメタ情報
type ResponseMeta struct {
	ResultCount int    `json:"result_count"`
	NewestID    string `json:"newest_id"`
	OldestID    string `json:"oldest_id"`
}

// NewClient は新しいTwitterクライアントを作成
func NewClient(bearerToken string) *Client {
	return &Client{
		bearerToken: bearerToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetUserTweets は指定されたユーザーの最新ツイートを取得
func (c *Client) GetUserTweets(ctx context.Context, username string, maxResults int) ([]Tweet, error) {
	// まずユーザーIDを取得
	userID, err := c.getUserIDByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID for @%s: %w", username, err)
	}

	// ツイートを取得
	endpoint := fmt.Sprintf("https://api.twitter.com/2/users/%s/tweets", userID)
	params := url.Values{}
	params.Set("max_results", fmt.Sprintf("%d", maxResults))
	params.Set("tweet.fields", "created_at,author_id")
	params.Set("exclude", "retweets,replies") // リツイートとリプライを除外

	tweets, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	// ユーザー名を設定
	for i := range tweets {
		tweets[i].Username = username
	}

	return tweets, nil
}

// SearchTweets はキーワードでツイートを検索
func (c *Client) SearchTweets(ctx context.Context, query string, maxResults int) ([]Tweet, error) {
	endpoint := "https://api.twitter.com/2/tweets/search/recent"
	params := url.Values{}
	params.Set("query", query)
	params.Set("max_results", fmt.Sprintf("%d", maxResults))
	params.Set("tweet.fields", "created_at,author_id")
	params.Set("expansions", "author_id")
	params.Set("user.fields", "username")

	resp, err := c.makeRequestWithUsers(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// getUserIDByUsername はユーザー名からユーザーIDを取得
func (c *Client) getUserIDByUsername(ctx context.Context, username string) (string, error) {
	// @を除去
	username = strings.TrimPrefix(username, "@")

	endpoint := fmt.Sprintf("https://api.twitter.com/2/users/by/username/%s", username)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.bearerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data User `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Data.ID, nil
}

// makeRequest は共通のリクエスト処理
func (c *Client) makeRequest(ctx context.Context, endpoint string, params url.Values) ([]Tweet, error) {
	urlStr := endpoint
	if len(params) > 0 {
		urlStr += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.bearerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return []Tweet{}, nil
	}

	return result.Data, nil
}

// makeRequestWithUsers はユーザー情報を含むリクエスト処理
func (c *Client) makeRequestWithUsers(ctx context.Context, endpoint string, params url.Values) ([]Tweet, error) {
	urlStr := endpoint
	if len(params) > 0 {
		urlStr += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.bearerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return []Tweet{}, nil
	}

	// ユーザー名をマッピング
	userMap := make(map[string]string)
	if result.Includes != nil && result.Includes.Users != nil {
		for _, user := range result.Includes.Users {
			userMap[user.ID] = user.Username
		}
	}

	tweets := result.Data
	for i := range tweets {
		if username, ok := userMap[tweets[i].AuthorID]; ok {
			tweets[i].Username = username
		}
	}

	return tweets, nil
}
