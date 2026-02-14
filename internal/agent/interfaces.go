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
