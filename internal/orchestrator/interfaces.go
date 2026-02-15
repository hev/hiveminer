package orchestrator

import (
	"context"

	"threadminer/pkg/types"
)

// RunConfig holds configuration for an extraction run
type RunConfig struct {
	FormPath       string
	Form           *types.Form
	Query          string
	Subreddits     []string
	Limit          int
	Sort           string
	OutputDir      string
	Workers        int    // concurrent extraction workers (default 10)
	DiscoveryModel string // model for phases 0+1 (default "opus")
	EvalModel      string // model for phase 2 (default "opus")
	ExtractModel   string // model for phase 3 (default "haiku")
	RankModel      string // model for phase 4 (default "haiku")
}

// Orchestrator defines the interface for running extraction pipelines
type Orchestrator interface {
	// Run executes the full extraction pipeline and returns the session directory
	Run(ctx context.Context, config RunConfig) (string, error)
}
