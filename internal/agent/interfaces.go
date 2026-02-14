package agent

import (
	"context"
	"strings"

	"threadminer/pkg/types"
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

// ANSI color codes for streaming output
const (
	colorDim   = "\033[90m" // dark gray — subdued streaming output
	colorReset = "\033[0m"
)

// stripCodeFences removes markdown code fences from LLM responses so the
// JSON inside can be parsed cleanly. Handles ```json ... ``` wrapping and
// duplicated blocks that some models produce.
func stripCodeFences(s string) string {
	var result strings.Builder
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			continue
		}
		result.WriteString(line)
		result.WriteByte('\n')
	}
	return result.String()
}
