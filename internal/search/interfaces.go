package search

import (
	"context"

	"hiveminer/pkg/types"
)

// Searcher defines the interface for searching and fetching Reddit content
type Searcher interface {
	// Search searches Reddit for posts matching a query
	Search(ctx context.Context, query, subreddit string, limit int) ([]types.Post, error)

	// ListSubreddit lists posts from a subreddit with sorting
	ListSubreddit(ctx context.Context, subreddit, sort string, limit int) ([]types.Post, error)

	// GetThread fetches a complete thread with comments
	GetThread(ctx context.Context, permalink string, commentLimit int) (*types.Thread, error)
}
