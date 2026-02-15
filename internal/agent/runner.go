package agent

import (
	"context"

	claude "go-claude"
)

// Runner abstracts the Claude CLI client for mockability.
type Runner interface {
	Run(ctx context.Context, prompt string, opts ...claude.RunOption) (claude.Result, error)
}
