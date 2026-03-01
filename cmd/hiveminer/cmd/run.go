package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"belaykit"
	"belaykit/claude"
	"belaykit/codex"
	"belaykit/providers/belay"

	"hiveminer/internal/agent"
	"hiveminer/internal/orchestrator"
	"hiveminer/internal/schema"
	"hiveminer/internal/search"
)

type tracedRunner struct {
	base    agent.Runner
	traceID string
}

func (r tracedRunner) Run(ctx context.Context, prompt string, opts ...belaykit.RunOption) (belaykit.Result, error) {
	if r.traceID != "" {
		opts = append(opts, belaykit.WithTraceID(r.traceID))
	}
	return r.base.Run(ctx, prompt, opts...)
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	formPath := fs.String("form", "", "Path to form JSON file (required)")
	query := fs.String("query", "", "Search query")
	subreddits := fs.String("subreddits", "", "Comma-separated list of subreddits")
	limit := fs.Int("limit", 20, "Maximum number of threads to process")
	sort := fs.String("sort", "hot", "Sort method for subreddit listing: hot, new, top, rising")
	outputDir := fs.String("output", "./output", "Output directory for session")
	workers := fs.Int("workers", 10, "Concurrent extraction workers")
	discoveryModel := fs.String("discovery-model", "sonnet", "Model for phases 0+1 (subreddit/thread discovery)")
	evalModel := fs.String("eval-model", "sonnet", "Model for phase 2 (thread evaluation)")
	extractModel := fs.String("extract-model", "haiku", "Model for phase 3 (field extraction)")
	rankModel := fs.String("rank-model", "haiku", "Model for phase 4 (entry ranking)")
	fs.StringVar(query, "q", "", "Search query (shorthand)")
	fs.StringVar(subreddits, "r", "", "Subreddits (shorthand)")
	fs.IntVar(limit, "l", 20, "Limit (shorthand)")
	fs.StringVar(outputDir, "o", "./output", "Output directory (shorthand)")
	useCodex := fs.Bool("codex", false, "Use Codex backend instead of Claude")
	verbose := fs.Bool("verbose", false, "Show full agent log output")
	fs.BoolVar(verbose, "v", false, "Verbose (shorthand)")

	fs.Parse(args)

	// When using codex, switch to codex-appropriate model defaults unless explicitly set
	if *useCodex {
		explicit := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
		if !explicit["discovery-model"] {
			*discoveryModel = "" // codex CLI default
		}
		if !explicit["eval-model"] {
			*evalModel = "" // codex CLI default
		}
		if !explicit["extract-model"] {
			*extractModel = "gpt-5.1-codex-mini"
		}
		if !explicit["rank-model"] {
			*rankModel = "gpt-5.1-codex-mini"
		}
	}

	if *formPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --form is required")
		fmt.Fprintln(os.Stderr, "Usage: hiveminer run --form forms/gifts.json [-q \"search query\"] [-r subreddits] --limit 20")
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

	// Create shared client and prompt filesystem
	var client agent.Runner
	var bp *belay.Provider
	var traceID string
	var belayHandler belaykit.EventHandler
	backend := "claude"
	if *useCodex {
		client = codex.NewClient()
		backend = "codex"
	} else {
		bp = belay.NewProvider(belay.WithPricing(claude.PricingForModel(*discoveryModel)), belay.WithContextWindow(200_000))
		client = claude.NewClient(claude.WithObservability(bp))
		traceID = bp.StartTrace(belaykit.TraceConfig{Name: form.Title}, nil)
		belayHandler = bp.EventHandler()
		client = tracedRunner{base: client, traceID: traceID}
	}
	agentLogger := func(name, model string) belaykit.EventHandler {
		logOpts := []belaykit.LoggerOption{
			belaykit.LogTokens(true),
			belaykit.LogContent(*verbose),
			belaykit.WithAgentName(name),
			belaykit.WithModelName(model),
		}
		if backend != "codex" {
			logOpts = append(logOpts,
				belaykit.WithPricing(claude.PricingForModel(model)),
				belaykit.WithContextWindow(claude.ContextWindowForModel(model)),
			)
		}
		logger := belaykit.NewLogger(os.Stderr, logOpts...)
		if bp == nil {
			return logger
		}
		return func(e belaykit.Event) {
			logger(e)
			belayHandler(e)
		}
	}
	prompts := os.DirFS("prompts")

	// Create orchestrator with agentic phases
	searcher := search.NewRedditSearcher()
	orch := orchestrator.New(searcher)
	orch.SetDiscoverer(agent.NewClaudeDiscoverer(client, prompts, *discoveryModel, agentLogger("discovery", *discoveryModel), backend))
	orch.SetThreadDiscoverer(agent.NewClaudeThreadDiscoverer(client, prompts, *discoveryModel, agentLogger("threads", *discoveryModel), backend))
	orch.SetThreadEvaluator(agent.NewClaudeEvaluator(client, prompts, *evalModel, agentLogger("eval", *evalModel), backend))
	orch.SetExtractor(agent.NewClaudeExtractor(client, prompts, *extractModel, agentLogger("extract", *extractModel), backend))
	orch.SetRanker(agent.NewClaudeRanker(client, prompts, *rankModel, agentLogger("rank", *rankModel), backend))

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
		RankModel:      *rankModel,
		OnPhaseStart: func(phaseName string) {
			if belayHandler != nil {
				belayHandler(belaykit.Event{Type: belaykit.EventPhase, PhaseName: phaseName})
			}
		},
	}

	sessionDir, err := orch.Run(ctx, config)

	if bp != nil {
		bp.EndTrace(traceID, nil)
	}
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
