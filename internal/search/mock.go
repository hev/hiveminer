package search

import (
	"context"

	"threadminer/pkg/types"
)

// MockSearcher implements Searcher for testing
type MockSearcher struct {
	Posts   []types.Post
	Threads map[string]*types.Thread
	Err     error
}

// NewMockSearcher creates a new mock searcher
func NewMockSearcher() *MockSearcher {
	return &MockSearcher{
		Threads: make(map[string]*types.Thread),
	}
}

// Search returns mock posts
func (m *MockSearcher) Search(ctx context.Context, query, subreddit string, limit int) ([]types.Post, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if limit > len(m.Posts) {
		return m.Posts, nil
	}
	return m.Posts[:limit], nil
}

// ListSubreddit returns mock posts
func (m *MockSearcher) ListSubreddit(ctx context.Context, subreddit, sort string, limit int) ([]types.Post, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if limit > len(m.Posts) {
		return m.Posts, nil
	}
	return m.Posts[:limit], nil
}

// GetThread returns a mock thread
func (m *MockSearcher) GetThread(ctx context.Context, permalink string, commentLimit int) (*types.Thread, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if thread, ok := m.Threads[permalink]; ok {
		return thread, nil
	}
	return &types.Thread{}, nil
}

