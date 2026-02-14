package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"threadminer/internal/agent"
	"threadminer/internal/schema"
	"threadminer/internal/search"
	"threadminer/internal/session"
	"threadminer/pkg/types"
)

// DefaultOrchestrator implements the extraction pipeline
type DefaultOrchestrator struct {
	searcher         search.Searcher
	extractor        agent.Extractor
	discoverer       agent.Discoverer
	threadDiscoverer agent.ThreadDiscoverer
	threadEvaluator  agent.ThreadEvaluator
}

// New creates a new orchestrator with a searcher
func New(searcher search.Searcher) *DefaultOrchestrator {
	return &DefaultOrchestrator{
		searcher: searcher,
	}
}

// SetExtractor sets the extractor to use
func (o *DefaultOrchestrator) SetExtractor(e agent.Extractor) {
	o.extractor = e
}

// SetDiscoverer sets the subreddit discoverer to use
func (o *DefaultOrchestrator) SetDiscoverer(d agent.Discoverer) {
	o.discoverer = d
}

// SetThreadDiscoverer sets the agentic thread discoverer to use
func (o *DefaultOrchestrator) SetThreadDiscoverer(td agent.ThreadDiscoverer) {
	o.threadDiscoverer = td
}

// SetThreadEvaluator sets the agentic thread evaluator to use
func (o *DefaultOrchestrator) SetThreadEvaluator(te agent.ThreadEvaluator) {
	o.threadEvaluator = te
}

// Run executes the full extraction pipeline and returns the session directory
func (o *DefaultOrchestrator) Run(ctx context.Context, config RunConfig) (string, error) {
	// Create session directory
	slug := session.GenerateSlugFromQuery(config.Query)
	if config.Query == "" && len(config.Subreddits) > 0 {
		slug = session.GenerateSlug(config.Subreddits[0])
	}
	sessionDir := filepath.Join(config.OutputDir, slug)

	// Check for existing session or create new
	manifest, err := session.LoadManifest(sessionDir)
	if err != nil {
		return "", fmt.Errorf("loading manifest: %w", err)
	}

	if manifest == nil {
		// Create new session
		formHash, err := schema.HashForm(config.Form)
		if err != nil {
			return "", fmt.Errorf("hashing form: %w", err)
		}

		formRef := types.FormRef{
			Title: config.Form.Title,
			Path:  config.FormPath,
			Hash:  formHash,
		}

		manifest = session.NewManifest(formRef, config.Query, config.Subreddits)
		fmt.Printf("Creating new session: %s\n", sessionDir)
	} else {
		fmt.Printf("Resuming session: %s\n", sessionDir)
	}

	// Start run log
	invocationID := fmt.Sprintf("run-%d", time.Now().Unix())
	session.StartRun(manifest, invocationID)

	// Save initial manifest
	if err := session.SaveManifest(sessionDir, manifest); err != nil {
		return "", fmt.Errorf("saving manifest: %w", err)
	}

	// Phase 0: Subreddit Discovery
	if config.Query != "" && len(config.Subreddits) == 0 {
		if manifest.DiscoveredSubreddits && len(manifest.Subreddits) > 0 {
			fmt.Printf("Reusing %d previously discovered subreddits\n", len(manifest.Subreddits))
			config.Subreddits = manifest.Subreddits
		} else {
			fmt.Println("\n=== Phase 0: Subreddit Discovery ===")
			if o.discoverer == nil {
				o.discoverer = agent.NewClaudeDiscoverer("prompts", config.DiscoveryModel)
			}
			discovered, err := o.discoverer.DiscoverSubreddits(ctx, config.Form, config.Query)
			if err != nil {
				fmt.Printf("  Warning: subreddit discovery failed: %v\n", err)
				fmt.Println("  Falling back to searching all of Reddit")
			} else if len(discovered) > 0 {
				fmt.Printf("Discovered %d subreddits:\n", len(discovered))
				for _, name := range discovered {
					fmt.Printf("  r/%s\n", name)
				}
				config.Subreddits = discovered
				manifest.Subreddits = discovered
				manifest.DiscoveredSubreddits = true
				if err := session.SaveManifest(sessionDir, manifest); err != nil {
					return "", fmt.Errorf("saving manifest: %w", err)
				}
			}
		}
	}

	// Phases 1+2+3 with retry loop: discover threads, then evaluate+extract in parallel
	const maxRounds = 3
	var totalProcessed int
	for round := 0; round < maxRounds; round++ {
		if ctx.Err() != nil {
			session.CompleteRun(manifest, "interrupted", totalProcessed)
			session.SaveManifest(sessionDir, manifest)
			return sessionDir, ctx.Err()
		}

		// Check if we already have enough extracted threads
		counts := session.CountByStatus(manifest)
		if counts["extracted"] >= config.Limit {
			fmt.Printf("Already have %d extracted threads (target: %d)\n", counts["extracted"], config.Limit)
			break
		}

		if round > 0 {
			fmt.Printf("\n=== Retry round %d: need more threads (have %d extracted, need %d) ===\n", round+1, counts["extracted"], config.Limit)
		}

		// Phase 1: Agentic Thread Discovery
		fmt.Println("\n=== Phase 1: Thread Discovery ===")
		if err := o.discover(ctx, config, manifest, sessionDir); err != nil {
			if ctx.Err() != nil {
				session.CompleteRun(manifest, "interrupted", totalProcessed)
				session.SaveManifest(sessionDir, manifest)
				return sessionDir, ctx.Err()
			}
			return "", fmt.Errorf("discovery: %w", err)
		}

		// Phase 2+3: Evaluate & Extract in parallel
		fmt.Println("\n=== Phase 2+3: Evaluate & Extract ===")
		processed, err := o.evaluateAndExtract(ctx, config, manifest, sessionDir)
		if err != nil {
			if ctx.Err() != nil {
				session.CompleteRun(manifest, "interrupted", totalProcessed+processed)
				session.SaveManifest(sessionDir, manifest)
				return sessionDir, ctx.Err()
			}
			return "", fmt.Errorf("evaluate+extract: %w", err)
		}
		totalProcessed += processed

		counts = session.CountByStatus(manifest)
		if counts["extracted"] >= config.Limit {
			break
		}
	}

	// Complete run
	session.CompleteRun(manifest, "completed", totalProcessed)
	if err := session.SaveManifest(sessionDir, manifest); err != nil {
		return "", fmt.Errorf("saving final manifest: %w", err)
	}

	// Print summary
	counts := session.CountByStatus(manifest)
	fmt.Printf("\n=== Complete ===\n")
	fmt.Printf("Session: %s\n", sessionDir)
	fmt.Printf("Threads: %d total\n", len(manifest.Threads))
	fmt.Printf("  - Extracted: %d\n", counts["extracted"])
	fmt.Printf("  - Collected: %d\n", counts["collected"])
	fmt.Printf("  - Skipped: %d\n", counts["skipped"])
	fmt.Printf("  - Failed: %d\n", counts["failed"])

	return sessionDir, nil
}

// discover finds threads matching the search criteria using the agentic discoverer
func (o *DefaultOrchestrator) discover(ctx context.Context, config RunConfig, manifest *types.Manifest, sessionDir string) error {
	// Count threads that are still actionable (not skipped/failed)
	counts := session.CountByStatus(manifest)
	actionable := counts["pending"] + counts["collected"] + counts["extracted"]
	overprovisionTarget := config.Limit * 3

	if actionable >= overprovisionTarget {
		fmt.Printf("Already have %d actionable threads (target: %d), skipping discovery\n", actionable, overprovisionTarget)
		return nil
	}

	remaining := overprovisionTarget - actionable

	// Use agentic thread discoverer if available
	if o.threadDiscoverer != nil {
		fmt.Printf("Agent discovering %d threads across %v\n", remaining, config.Subreddits)

		// Ensure session dir exists for the agent to write to
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return fmt.Errorf("creating session dir: %w", err)
		}

		posts, err := o.threadDiscoverer.DiscoverThreads(ctx, config.Form, config.Query, config.Subreddits, remaining, sessionDir)
		if err != nil {
			fmt.Printf("  Warning: agentic discovery failed: %v\n", err)
			fmt.Println("  Falling back to direct search")
			return o.discoverDirect(ctx, config, manifest, sessionDir, remaining)
		}

		return o.addDiscoveredPosts(posts, manifest, sessionDir, remaining)
	}

	// Fallback to direct search
	return o.discoverDirect(ctx, config, manifest, sessionDir, remaining)
}

// discoverDirect performs non-agentic thread discovery using direct API calls
func (o *DefaultOrchestrator) discoverDirect(ctx context.Context, config RunConfig, manifest *types.Manifest, sessionDir string, remaining int) error {
	var posts []types.Post

	if config.Query != "" {
		// Search mode
		for _, sub := range config.Subreddits {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			fmt.Printf("Searching r/%s for: %s\n", sub, config.Query)
			subPosts, err := o.searcher.Search(ctx, config.Query, sub, remaining)
			if err != nil {
				fmt.Printf("  Warning: search failed: %v\n", err)
				continue
			}
			posts = append(posts, subPosts...)
			fmt.Printf("  Found %d posts\n", len(subPosts))
		}

		// If no subreddits specified, search all
		if len(config.Subreddits) == 0 {
			fmt.Printf("Searching all of Reddit for: %s\n", config.Query)
			subPosts, err := o.searcher.Search(ctx, config.Query, "all", remaining)
			if err != nil {
				return err
			}
			posts = subPosts
			fmt.Printf("  Found %d posts\n", len(subPosts))
		}
	} else {
		// List mode
		for _, sub := range config.Subreddits {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			fmt.Printf("Listing r/%s (%s)\n", sub, config.Sort)
			subPosts, err := o.searcher.ListSubreddit(ctx, sub, config.Sort, remaining)
			if err != nil {
				fmt.Printf("  Warning: list failed: %v\n", err)
				continue
			}
			posts = append(posts, subPosts...)
			fmt.Printf("  Found %d posts\n", len(subPosts))
		}
	}

	return o.addDiscoveredPosts(posts, manifest, sessionDir, remaining)
}

// addDiscoveredPosts adds discovered posts to the manifest
func (o *DefaultOrchestrator) addDiscoveredPosts(posts []types.Post, manifest *types.Manifest, sessionDir string, remaining int) error {
	added := 0
	for _, post := range posts {
		if added >= remaining {
			break
		}

		// Skip if already in manifest
		if session.FindThread(manifest, post.ID) != nil {
			continue
		}

		thread := types.ThreadState{
			PostID:      post.ID,
			Permalink:   post.Permalink,
			Title:       post.Title,
			Subreddit:   post.Subreddit,
			Score:       post.Score,
			NumComments: post.NumComments,
			Status:      "pending",
		}
		session.AddThread(manifest, thread)
		added++
	}

	fmt.Printf("Added %d new threads to session\n", added)

	if err := session.SaveManifest(sessionDir, manifest); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// outputExtractor is an optional interface for extractors that support directing output to a writer
type outputExtractor interface {
	ExtractFieldsWithOutput(ctx context.Context, thread *types.Thread, form *types.Form, output io.Writer) (*types.ExtractionResult, error)
}

// syncWriter wraps an io.Writer with a mutex for safe concurrent writes
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

// extractSingle runs extraction on a single thread, using output-aware method if available
func (o *DefaultOrchestrator) extractSingle(ctx context.Context, thread *types.Thread, form *types.Form, output io.Writer) (*types.ExtractionResult, error) {
	if oe, ok := o.extractor.(outputExtractor); ok {
		return oe.ExtractFieldsWithOutput(ctx, thread, form, output)
	}
	return o.extractor.ExtractFields(ctx, thread, form)
}

// workItem represents a thread to process in the combined evaluate+extract pipeline
type workItem struct {
	state     types.ThreadState
	needsEval bool // true for pending threads, false for already-collected threads
}

// evaluateAndExtract runs evaluation and extraction in a single parallel pipeline.
// Pending threads are evaluated first; if kept, extraction follows immediately in the same worker.
// Already-collected threads (from resume) skip evaluation and go straight to extraction.
func (o *DefaultOrchestrator) evaluateAndExtract(ctx context.Context, config RunConfig, manifest *types.Manifest, sessionDir string) (int, error) {
	if o.extractor == nil {
		o.extractor = agent.NewClaudeExtractor("prompts", config.ExtractModel)
	}

	// Gather work: pending threads need eval+extract, collected threads need extract only
	pending := session.GetPendingThreads(manifest)
	collected := session.GetCollectedThreads(manifest)

	var items []workItem
	for _, ts := range pending {
		items = append(items, workItem{ts, true})
	}
	for _, ts := range collected {
		items = append(items, workItem{ts, false})
	}

	if len(items) == 0 {
		fmt.Println("No threads to process")
		return 0, nil
	}

	// Determine worker count
	workers := config.Workers
	if workers <= 0 {
		workers = 4
	}
	if workers > len(items) {
		workers = len(items)
	}

	// Open extraction log file
	logPath := filepath.Join(sessionDir, "extraction.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("creating extraction log: %w", err)
	}
	defer logFile.Close()
	logWriter := &syncWriter{w: logFile}

	fmt.Printf("Processing %d threads with %d workers (%d to evaluate, %d to extract)\n",
		len(items), workers, len(pending), len(collected))

	// Fill work channel
	work := make(chan workItem, len(items))
	for _, item := range items {
		work <- item
	}
	close(work)

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		extracted atomic.Int64
		done      atomic.Int64
		total     = len(items)
		processed int
	)

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for item := range work {
				if ctx.Err() != nil {
					return
				}

				// Early stop: enough threads extracted
				mu.Lock()
				counts := session.CountByStatus(manifest)
				enough := counts["extracted"] >= config.Limit
				mu.Unlock()
				if enough {
					return
				}

				ts := item.state
				n := done.Add(1)

				// Step 1: Evaluate if needed
				if item.needsEval {
					if o.threadEvaluator != nil {
						evalResult, err := o.threadEvaluator.EvaluateThread(ctx, config.Form, ts, sessionDir)
						if err != nil {
							mu.Lock()
							session.UpdateThreadStatus(manifest, ts.PostID, "failed")
							session.SaveManifest(sessionDir, manifest)
							mu.Unlock()
							fmt.Printf("  [%d/%d] %s → eval failed: %v\n", n, total, truncate(ts.Title, 50), err)
							continue
						}

						if evalResult.Verdict != "keep" {
							mu.Lock()
							session.UpdateThreadStatus(manifest, ts.PostID, "skipped")
							session.SaveManifest(sessionDir, manifest)
							mu.Unlock()
							fmt.Printf("  [%d/%d] %s → SKIP: %s\n", n, total, truncate(ts.Title, 50), evalResult.Reason)
							continue
						}

						// Mark as collected
						mu.Lock()
						now := time.Now()
						idx := session.FindThreadIndex(manifest, ts.PostID)
						if idx >= 0 {
							manifest.Threads[idx].Status = "collected"
							manifest.Threads[idx].CollectedAt = &now
						}
						session.SaveManifest(sessionDir, manifest)
						mu.Unlock()
					} else {
						// No evaluator: fetch thread directly
						thread, err := o.searcher.GetThread(ctx, ts.Permalink, 100)
						if err != nil {
							mu.Lock()
							session.UpdateThreadStatus(manifest, ts.PostID, "failed")
							session.SaveManifest(sessionDir, manifest)
							mu.Unlock()
							fmt.Printf("  [%d/%d] %s → fetch failed: %v\n", n, total, truncate(ts.Title, 50), err)
							continue
						}

						threadPath := filepath.Join(sessionDir, fmt.Sprintf("thread_%s.json", ts.PostID))
						threadData, err := json.MarshalIndent(thread, "", "  ")
						if err != nil {
							mu.Lock()
							session.UpdateThreadStatus(manifest, ts.PostID, "failed")
							session.SaveManifest(sessionDir, manifest)
							mu.Unlock()
							continue
						}
						if err := os.WriteFile(threadPath, threadData, 0644); err != nil {
							mu.Lock()
							session.UpdateThreadStatus(manifest, ts.PostID, "failed")
							session.SaveManifest(sessionDir, manifest)
							mu.Unlock()
							continue
						}

						mu.Lock()
						now := time.Now()
						idx := session.FindThreadIndex(manifest, ts.PostID)
						if idx >= 0 {
							manifest.Threads[idx].Status = "collected"
							manifest.Threads[idx].CollectedAt = &now
						}
						session.SaveManifest(sessionDir, manifest)
						mu.Unlock()
					}
				}

				// Step 2: Extract fields from thread JSON
				threadPath := filepath.Join(sessionDir, fmt.Sprintf("thread_%s.json", ts.PostID))
				threadData, err := os.ReadFile(threadPath)
				if err != nil {
					mu.Lock()
					session.UpdateThreadStatus(manifest, ts.PostID, "failed")
					session.SaveManifest(sessionDir, manifest)
					mu.Unlock()
					fmt.Printf("  [%d/%d] %s → thread file missing: %v\n", n, total, truncate(ts.Title, 50), err)
					continue
				}

				var thread types.Thread
				if err := json.Unmarshal(threadData, &thread); err != nil {
					mu.Lock()
					session.UpdateThreadStatus(manifest, ts.PostID, "failed")
					session.SaveManifest(sessionDir, manifest)
					mu.Unlock()
					continue
				}

				result, err := o.extractSingle(ctx, &thread, config.Form, logWriter)
				if err != nil {
					mu.Lock()
					idx := session.FindThreadIndex(manifest, ts.PostID)
					if idx >= 0 {
						manifest.Threads[idx].Status = "failed"
						manifest.Threads[idx].Error = err.Error()
					}
					session.SaveManifest(sessionDir, manifest)
					mu.Unlock()
					fmt.Printf("  [%d/%d] %s → extract failed: %v\n", n, total, truncate(ts.Title, 50), err)
					continue
				}

				e := extracted.Add(1)

				mu.Lock()
				session.UpdateThreadEntries(manifest, ts.PostID, result.Entries)
				processed++
				session.SaveManifest(sessionDir, manifest)
				mu.Unlock()

				fmt.Printf("  [%d extracted] %s (%d entries)\n", e, truncate(ts.Title, 50), len(result.Entries))
			}
		}()
	}

	wg.Wait()

	fmt.Printf("Extraction log: %s\n", logPath)

	return processed, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
