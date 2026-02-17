package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hiveminer/pkg/types"
)

const (
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
	baseURL   = "https://www.reddit.com"
)

// RedditSearcher implements Searcher for the Reddit API
type RedditSearcher struct {
	client *http.Client
}

// NewRedditSearcher creates a new Reddit API searcher
func NewRedditSearcher() *RedditSearcher {
	return &RedditSearcher{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// redditResponse represents the JSON response from Reddit's API for posts
type redditResponse struct {
	Data struct {
		Children []struct {
			Data postData `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type postData struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	Domain      string  `json:"domain"`
	Permalink   string  `json:"permalink"`
	Selftext    string  `json:"selftext"`
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	NSFW        bool    `json:"over_18"`
	Created     float64 `json:"created_utc"`
}

// commentResponse for thread comments
type commentResponse []struct {
	Data struct {
		Children []commentChild `json:"children"`
	} `json:"data"`
}

type commentChild struct {
	Kind string      `json:"kind"`
	Data commentData `json:"data"`
}

type commentData struct {
	ID        string  `json:"id"`
	Body      string  `json:"body"`
	Author    string  `json:"author"`
	Score     int     `json:"score"`
	Created   float64 `json:"created_utc"`
	Permalink string  `json:"permalink"`
	Replies   any     `json:"replies"`
	Depth     int     `json:"depth"`
	// Post fields (for the first element)
	Title       string `json:"title"`
	Selftext    string `json:"selftext"`
	URL         string `json:"url"`
	Subreddit   string `json:"subreddit"`
	NumComments int    `json:"num_comments"`
	Domain      string `json:"domain"`
	NSFW        bool   `json:"over_18"`
}

// Search searches Reddit for posts matching a query
func (r *RedditSearcher) Search(ctx context.Context, query, subreddit string, limit int) ([]types.Post, error) {
	encoded := url.QueryEscape(query)
	apiURL := fmt.Sprintf("%s/r/%s/search.json?q=%s&limit=%d&restrict_sr=1&raw_json=1", baseURL, subreddit, encoded, limit)
	return r.fetchPosts(ctx, apiURL)
}

// ListSubreddit lists posts from a subreddit with sorting
func (r *RedditSearcher) ListSubreddit(ctx context.Context, subreddit, sort string, limit int) ([]types.Post, error) {
	apiURL := fmt.Sprintf("%s/r/%s/%s.json?limit=%d&raw_json=1", baseURL, subreddit, sort, limit)
	return r.fetchPosts(ctx, apiURL)
}

// GetThread fetches a complete thread with comments
func (r *RedditSearcher) GetThread(ctx context.Context, permalink string, commentLimit int) (*types.Thread, error) {
	// Clean up permalink
	permalink = strings.TrimPrefix(permalink, "https://reddit.com")
	permalink = strings.TrimPrefix(permalink, "https://www.reddit.com")
	if !strings.HasPrefix(permalink, "/") {
		permalink = "/" + permalink
	}

	apiURL := fmt.Sprintf("%s%s.json?limit=%d&raw_json=1&depth=10", baseURL, permalink, commentLimit)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var result commentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	thread := &types.Thread{}

	// First element contains the post
	if len(result) > 0 && len(result[0].Data.Children) > 0 {
		postData := result[0].Data.Children[0].Data
		thread.Post = types.Post{
			ID:          postData.ID,
			Title:       postData.Title,
			Selftext:    postData.Selftext,
			URL:         postData.URL,
			Author:      postData.Author,
			Subreddit:   postData.Subreddit,
			Score:       postData.Score,
			NumComments: postData.NumComments,
			Domain:      postData.Domain,
			Permalink:   permalink,
			NSFW:        postData.NSFW,
			Created:     postData.Created,
		}
	}

	// Second element contains comments
	if len(result) > 1 {
		thread.Comments = r.parseComments(result[1].Data.Children, 0)
	}

	return thread, nil
}

// parseComments recursively parses comments and their replies
func (r *RedditSearcher) parseComments(children []commentChild, depth int) []*types.Comment {
	var comments []*types.Comment

	for _, child := range children {
		if child.Kind != "t1" { // t1 = comment
			continue
		}

		comment := &types.Comment{
			ID:        child.Data.ID,
			Body:      child.Data.Body,
			Author:    child.Data.Author,
			Score:     child.Data.Score,
			Created:   child.Data.Created,
			Permalink: child.Data.Permalink,
			Depth:     depth,
		}

		// Parse nested replies
		if child.Data.Replies != nil {
			if repliesMap, ok := child.Data.Replies.(map[string]any); ok {
				if data, ok := repliesMap["data"].(map[string]any); ok {
					if childrenData, ok := data["children"].([]any); ok {
						var replyChildren []commentChild
						for _, c := range childrenData {
							if cMap, ok := c.(map[string]any); ok {
								var rc commentChild
								// Marshal and unmarshal to get proper struct
								if b, err := json.Marshal(cMap); err == nil {
									if json.Unmarshal(b, &rc) == nil {
										replyChildren = append(replyChildren, rc)
									}
								}
							}
						}
						comment.Replies = r.parseComments(replyChildren, depth+1)
					}
				}
			}
		}

		comments = append(comments, comment)
	}

	return comments
}

// fetchPosts fetches posts from a Reddit API URL
func (r *RedditSearcher) fetchPosts(ctx context.Context, apiURL string) ([]types.Post, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var result redditResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	posts := make([]types.Post, 0, len(result.Data.Children))
	for _, child := range result.Data.Children {
		posts = append(posts, types.Post{
			ID:          child.Data.ID,
			Title:       child.Data.Title,
			Score:       child.Data.Score,
			NumComments: child.Data.NumComments,
			Domain:      child.Data.Domain,
			Permalink:   child.Data.Permalink,
			Selftext:    child.Data.Selftext,
			URL:         child.Data.URL,
			Author:      child.Data.Author,
			Subreddit:   child.Data.Subreddit,
			NSFW:        child.Data.NSFW,
			Created:     child.Data.Created,
		})
	}

	return posts, nil
}
