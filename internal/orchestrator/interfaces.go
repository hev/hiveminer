package orchestrator

import (
	"context"

	"threadminer/pkg/types"
)

// RunConfig holds configuration for an extraction run
type RunConfig struct {
	FormPath   string
	Form       *types.Form
	Query      string
	Subreddits []string
	Limit      int
	Sort       string
	OutputDir  string
}

// Orchestrator defines the interface for running extraction pipelines
type Orchestrator interface {
	// Run executes the full extraction pipeline
	Run(ctx context.Context, config RunConfig) error
}
