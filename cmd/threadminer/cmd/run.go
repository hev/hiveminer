package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"threadminer/internal/agent"
	"threadminer/internal/orchestrator"
	"threadminer/internal/schema"
	"threadminer/internal/search"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	formPath := fs.String("form", "", "Path to form JSON file (required)")
	query := fs.String("query", "", "Search query")
	subreddits := fs.String("subreddits", "", "Comma-separated list of subreddits")
	limit := fs.Int("limit", 20, "Maximum number of threads to process")
	sort := fs.String("sort", "hot", "Sort method for subreddit listing: hot, new, top, rising")
	outputDir := fs.String("output", "./output", "Output directory for session")
	workers := fs.Int("workers", 4, "Concurrent extraction workers")
	discoveryModel := fs.String("discovery-model", "opus", "Model for phases 0+1 (subreddit/thread discovery)")
	evalModel := fs.String("eval-model", "opus", "Model for phase 2 (thread evaluation)")
	extractModel := fs.String("extract-model", "haiku", "Model for phase 3 (field extraction)")
	fs.StringVar(query, "q", "", "Search query (shorthand)")
	fs.StringVar(subreddits, "r", "", "Subreddits (shorthand)")
	fs.IntVar(limit, "l", 20, "Limit (shorthand)")
	fs.StringVar(outputDir, "o", "./output", "Output directory (shorthand)")

	fs.Parse(args)

	if *formPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --form is required")
		fmt.Fprintln(os.Stderr, "Usage: threadminer run --form forms/gifts.json [-q \"search query\"] [-r subreddits] --limit 20")
		return fmt.Errorf("--form is required")
	}

	// Load form
	form, err := schema.LoadForm(*formPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading form: %v\n", err)
		return err
	}

	// Infer query from form if not provided
	if *query == "" && *subreddits == "" {
		if len(form.SearchHints) > 0 {
			*query = form.SearchHints[0]
		} else {
			*query = form.Title
		}
		fmt.Printf("Using query from form: %s\n", *query)
	}

	// Parse subreddits
	var subs []string
	if *subreddits != "" {
		subs = strings.Split(*subreddits, ",")
		for i := range subs {
			subs[i] = strings.TrimSpace(subs[i])
		}
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted, saving progress...")
		cancel()
	}()

	// Create orchestrator with agentic phases
	searcher := search.NewRedditSearcher()
	orch := orchestrator.New(searcher)
	orch.SetThreadDiscoverer(agent.NewClaudeThreadDiscoverer("prompts", *discoveryModel))
	orch.SetThreadEvaluator(agent.NewClaudeEvaluator("prompts", *evalModel))
	orch.SetExtractor(agent.NewClaudeExtractor("prompts", *extractModel))

	// Run extraction
	config := orchestrator.RunConfig{
		FormPath:       *formPath,
		Form:           form,
		Query:          *query,
		Subreddits:     subs,
		Limit:          *limit,
		Sort:           *sort,
		OutputDir:      *outputDir,
		Workers:        *workers,
		DiscoveryModel: *discoveryModel,
		EvalModel:      *evalModel,
		ExtractModel:   *extractModel,
	}

	sessionDir, err := orch.Run(ctx, config)
	if err != nil {
		if ctx.Err() == context.Canceled {
			fmt.Println("Session saved. Run again to resume.")
			return nil
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	// Automatically show results
	return cmdRunsShow([]string{sessionDir})
}
