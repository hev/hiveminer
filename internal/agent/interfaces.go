package agent

import (
	"context"

	"hiveminer/pkg/types"
)

// Extractor defines the interface for extracting structured data from threads
type Extractor interface {
	// ExtractFields extracts all form fields from a thread
	ExtractFields(ctx context.Context, thread *types.Thread, form *types.Form) (*types.ExtractionResult, error)
}

// Discoverer defines the interface for discovering relevant subreddits
type Discoverer interface {
	// DiscoverSubreddits finds relevant subreddits for a form and query
	DiscoverSubreddits(ctx context.Context, form *types.Form, query string) ([]string, error)
}

// ThreadDiscoverer defines the interface for agentically discovering relevant threads
type ThreadDiscoverer interface {
	// DiscoverThreads finds relevant threads across subreddits for a form and query
	DiscoverThreads(ctx context.Context, form *types.Form, query string, subreddits []string, limit int, sessionDir string) ([]types.Post, error)
}

// ThreadEvaluator defines the interface for evaluating thread relevance
type ThreadEvaluator interface {
	// EvaluateThread evaluates whether a thread is relevant to the form
	EvaluateThread(ctx context.Context, form *types.Form, thread types.ThreadState, sessionDir string) (*EvalResult, error)
}

// EvalResult holds the evaluation verdict for a single thread
type EvalResult struct {
	PostID           string `json:"post_id"`
	Verdict          string `json:"verdict"` // "keep" or "skip"
	Reason           string `json:"reason"`
	EstimatedEntries int    `json:"estimated_entries"`
}

// Ranker defines the interface for ranking extracted entries
type Ranker interface {
	// RankEntries scores and flags entries using algorithmic + agentic assessment
	RankEntries(ctx context.Context, form *types.Form, entries []RankInput) ([]RankOutput, error)
}

// RankInput provides entry data with thread-level signals for ranking
type RankInput struct {
	ThreadPostID string
	EntryIndex   int
	Entry        types.Entry
	ThreadScore  int
	NumComments  int
}

// RankOutput holds the ranking result for a single entry
type RankOutput struct {
	ThreadPostID string   // identifies which thread
	EntryIndex   int      // identifies which entry within thread
	AlgoScore    float64  // algorithmic score 0-100
	Penalty      float64  // agentic penalty (negative)
	FinalScore   float64  // algo + penalty, clamped >= 0
	Flags        []string // spam, joke, etc.
	Reason       string   // Claude's assessment text
}
