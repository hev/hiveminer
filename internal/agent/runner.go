package agent

import (
	"context"

	"belaykit"
)

// Runner abstracts the Claude CLI client for mockability.
type Runner interface {
	Run(ctx context.Context, prompt string, opts ...belaykit.RunOption) (belaykit.Result, error)
}
