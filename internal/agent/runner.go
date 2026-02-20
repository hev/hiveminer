package agent

import (
	"context"

	rack "go-rack"
)

// Runner abstracts the Claude CLI client for mockability.
type Runner interface {
	Run(ctx context.Context, prompt string, opts ...rack.RunOption) (rack.Result, error)
}
