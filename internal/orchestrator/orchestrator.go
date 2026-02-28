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

	"hiveminer/internal/agent"
	"hiveminer/internal/schema"
	"hiveminer/internal/search"
	"hiveminer/internal/session"
	"hiveminer/pkg/types"
)

// DefaultOrchestrator implements the extraction pipeline
type DefaultOrchestrator struct {
	searcher         search.Searcher
	extractor        agent.Extractor
	discoverer       agent.Discoverer
	threadDiscoverer agent.ThreadDiscoverer
	threadEvaluator  agent.ThreadEvaluator
	ranker           agent.Ranker
}

func emitPhase(config RunConfig, phaseName string) {
	if config.OnPhaseStart != nil {
		config.OnPhaseStart(phaseName)
	}
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

// SetRanker sets the entry ranker to use
func (o *DefaultOrchestrator) SetRanker(r agent.Ranker) {
	o.ranker = r
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

	runStart := time.Now()

	// Phase 0: Subreddit Discovery
	if config.Query != "" && len(config.Subreddits) == 0 {
		if manifest.DiscoveredSubreddits && len(manifest.Subreddits) > 0 {
			fmt.Printf("Reusing %d previously discovered subreddits\n", len(manifest.Subreddits))
			config.Subreddits = manifest.Subreddits
		} else {
			emitPhase(config, "subreddit-discovery")
			fmt.Println("\n=== Phase 0: Subreddit Discovery ===")
			phase0Start := time.Now()
			if o.discoverer != nil {
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
			fmt.Printf("  Phase 0 completed in %s\n", formatDuration(time.Since(phase0Start)))
		}
	}

	// Phases 1+2+3: Streaming pipeline — discover threads and evaluate+extract in parallel
	pipelineStart := time.Now()
	totalProcessed, err := o.runPipeline(ctx, config, manifest, sessionDir)
	if err != nil {
		if ctx.Err() != nil {
			session.CompleteRun(manifest, "interrupted", totalProcessed)
			session.SaveManifest(sessionDir, manifest)
			return sessionDir, ctx.Err()
		}
		return "", err
	}

	fmt.Printf("  Pipeline completed in %s\n", formatDuration(time.Since(pipelineStart)))

	if ctx.Err() != nil {
		session.CompleteRun(manifest, "interrupted", totalProcessed)
		session.SaveManifest(sessionDir, manifest)
		return sessionDir, ctx.Err()
	}

	// Phase 4: Rank all extracted entries
	if o.ranker != nil {
		emitPhase(config, "ranking")
		fmt.Println("\n=== Phase 4: Ranking ===")
		phase4Start := time.Now()
		ranked, err := o.rankEntries(ctx, config, manifest, sessionDir)
		if err != nil {
			if ctx.Err() != nil {
				session.CompleteRun(manifest, "interrupted", totalProcessed)
				session.SaveManifest(sessionDir, manifest)
				return sessionDir, ctx.Err()
			}
			fmt.Printf("  Warning: ranking failed: %v\n", err)
			fmt.Println("  Continuing without ranking")
		} else {
			fmt.Printf("  Ranked %d entries (%s)\n", ranked, formatDuration(time.Since(phase4Start)))
		}
	}

	// Complete run
	session.CompleteRun(manifest, "completed", totalProcessed)
	if err := session.SaveManifest(sessionDir, manifest); err != nil {
		return "", fmt.Errorf("saving final manifest: %w", err)
	}

	// Print summary
	totalDuration := time.Since(runStart)
	counts := session.CountByStatus(manifest)
	fmt.Printf("\n=== Complete (%s) ===\n", formatDuration(totalDuration))
	fmt.Printf("Session: %s\n", sessionDir)
	fmt.Printf("Threads: %d total\n", len(manifest.Threads))
	fmt.Printf("  - Ranked: %d\n", counts["ranked"])
	fmt.Printf("  - Extracted: %d\n", counts["extracted"])
	fmt.Printf("  - Collected: %d\n", counts["collected"])
	fmt.Printf("  - Skipped: %d\n", counts["skipped"])
	fmt.Printf("  - Failed: %d\n", counts["failed"])

	return sessionDir, nil
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

// runPipeline executes the streaming discovery + evaluate + extract pipeline.
// Workers run continuously while discovery feeds them threads across multiple rounds.
// Manifest saves are batched via a periodic saver instead of per-update.
func (o *DefaultOrchestrator) runPipeline(ctx context.Context, config RunConfig, manifest *types.Manifest, sessionDir string) (int, error) {
	if o.extractor == nil {
		return 0, fmt.Errorf("no extractor configured")
	}

	workers := config.Workers
	if workers <= 0 {
		workers = 10
	}
	if workers > 50 {
		workers = 50
	}

	// Log file
	logPath := filepath.Join(sessionDir, "extraction.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("creating extraction log: %w", err)
	}
	defer logFile.Close()
	logWriter := &syncWriter{w: logFile}

	var (
		mu        sync.Mutex // protects manifest and processed
		wg        sync.WaitGroup
		processed int
		extracted atomic.Int64
		done      atomic.Int64
		totalFed  atomic.Int64
	)

	// Periodic manifest saver — batches disk writes instead of saving on every update
	dirty := &atomic.Bool{}
	saveCtx, saveCancel := context.WithCancel(context.Background())
	saveDone := make(chan struct{})
	go func() {
		defer close(saveDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if dirty.CompareAndSwap(true, false) {
					mu.Lock()
					session.SaveManifest(sessionDir, manifest)
					mu.Unlock()
				}
			case <-saveCtx.Done():
				mu.Lock()
				session.SaveManifest(sessionDir, manifest)
				mu.Unlock()
				return
			}
		}
	}()
	markDirty := func() { dirty.Store(true) }

	// Work channel — buffered so discovery can feed without blocking
	workCh := make(chan workItem, 200)

	// Start worker pool — workers persist across discovery rounds
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for item := range workCh {
				if ctx.Err() != nil {
					return
				}

				// Early stop: enough threads extracted
				mu.Lock()
				counts := session.CountByStatus(manifest)
				enough := counts["extracted"]+counts["ranked"] >= config.Limit
				mu.Unlock()
				if enough {
					return
				}

				ts := item.state
				n := done.Add(1)
				total := totalFed.Load()
				markThreadFailed := func(err error) {
					idx := session.FindThreadIndex(manifest, ts.PostID)
					if idx >= 0 {
						manifest.Threads[idx].Status = "failed"
						if err != nil {
							manifest.Threads[idx].Error = err.Error()
						}
					}
				}

				// Step 1: Evaluate if needed
				if item.needsEval {
					if o.threadEvaluator != nil {
						evalResult, err := o.threadEvaluator.EvaluateThread(ctx, config.Form, ts, sessionDir)
						if err != nil {
							mu.Lock()
							markThreadFailed(fmt.Errorf("evaluation failed: %w", err))
							mu.Unlock()
							markDirty()
							fmt.Printf("  [%d/%d] %s → eval failed: %v\n", n, total, truncate(ts.Title, 50), err)
							continue
						}

						if evalResult.Verdict != "keep" {
							mu.Lock()
							session.UpdateThreadStatus(manifest, ts.PostID, "skipped")
							mu.Unlock()
							markDirty()
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
						mu.Unlock()
						markDirty()
					} else {
						// No evaluator: fetch thread directly
						thread, err := o.searcher.GetThread(ctx, ts.Permalink, 100)
						if err != nil {
							mu.Lock()
							markThreadFailed(fmt.Errorf("thread fetch failed: %w", err))
							mu.Unlock()
							markDirty()
							fmt.Printf("  [%d/%d] %s → fetch failed: %v\n", n, total, truncate(ts.Title, 50), err)
							continue
						}

						// Write thread JSON OUTSIDE the lock
						threadPath := filepath.Join(sessionDir, fmt.Sprintf("thread_%s.json", ts.PostID))
						threadData, err := json.MarshalIndent(thread, "", "  ")
						if err != nil {
							mu.Lock()
							markThreadFailed(fmt.Errorf("thread marshal failed: %w", err))
							mu.Unlock()
							markDirty()
							continue
						}
						if err := os.WriteFile(threadPath, threadData, 0644); err != nil {
							mu.Lock()
							markThreadFailed(fmt.Errorf("thread write failed: %w", err))
							mu.Unlock()
							markDirty()
							continue
						}

						mu.Lock()
						now := time.Now()
						idx := session.FindThreadIndex(manifest, ts.PostID)
						if idx >= 0 {
							manifest.Threads[idx].Status = "collected"
							manifest.Threads[idx].CollectedAt = &now
						}
						mu.Unlock()
						markDirty()
					}
				}

				// Step 2: Extract fields from thread JSON
				thread, err := o.loadThreadForExtraction(ctx, ts, sessionDir)
				if err != nil {
					mu.Lock()
					markThreadFailed(err)
					mu.Unlock()
					markDirty()
					fmt.Printf("  [%d/%d] %s → thread load failed: %v\n", n, total, truncate(ts.Title, 50), err)
					continue
				}

				result, err := o.extractSingle(ctx, thread, config.Form, logWriter)
				if err != nil {
					mu.Lock()
					markThreadFailed(fmt.Errorf("extraction failed: %w", err))
					mu.Unlock()
					markDirty()
					fmt.Printf("  [%d/%d] %s → extract failed: %v\n", n, total, truncate(ts.Title, 50), err)
					continue
				}

				e := extracted.Add(1)

				mu.Lock()
				session.UpdateThreadEntries(manifest, ts.PostID, result.Entries)
				processed++
				mu.Unlock()
				markDirty()

				fmt.Printf("  [%d extracted] %s (%d entries)\n", e, truncate(ts.Title, 50), len(result.Entries))
			}
		}()
	}

	// Track which threads have been fed to the work channel
	fed := make(map[string]bool)

	// Feed already-collected threads (resume case)
	mu.Lock()
	collected := session.GetCollectedThreads(manifest)
	mu.Unlock()
	for _, ts := range collected {
		fed[ts.PostID] = true
		totalFed.Add(1)
		workCh <- workItem{ts, false}
	}

	// Discovery + feed loop — runs discovery and feeds workers across multiple rounds
	const maxRounds = 3
	for round := 0; round < maxRounds; round++ {
		if ctx.Err() != nil {
			break
		}

		// Check if we already have enough extracted threads
		mu.Lock()
		counts := session.CountByStatus(manifest)
		haveEnough := counts["extracted"]+counts["ranked"] >= config.Limit
		mu.Unlock()
		if haveEnough {
			fmt.Printf("Already have %d extracted threads (target: %d)\n", counts["extracted"]+counts["ranked"], config.Limit)
			break
		}

		if round > 0 {
			fmt.Printf("\n=== Retry round %d: need more threads (have %d extracted, need %d) ===\n",
				round+1, counts["extracted"]+counts["ranked"], config.Limit)
		}

		// Phase 1: Discover threads
		emitPhase(config, "thread-discovery")
		fmt.Println("\n=== Phase 1: Thread Discovery ===")
		discoveryStart := time.Now()

		mu.Lock()
		counts = session.CountByStatus(manifest)
		actionable := counts["pending"] + counts["collected"] + counts["extracted"] + counts["ranked"]
		mu.Unlock()
		overprovisionTarget := config.Limit * 3
		remaining := overprovisionTarget - actionable

		if remaining <= 0 {
			fmt.Printf("Already have %d actionable threads (target: %d), skipping discovery\n", actionable, overprovisionTarget)
		} else {
			posts, err := o.findThreads(ctx, config, remaining, sessionDir)
			if err != nil {
				if ctx.Err() != nil {
					break
				}
				if round == 0 {
					close(workCh)
					wg.Wait()
					saveCancel()
					<-saveDone
					return 0, fmt.Errorf("discovery: %w", err)
				}
				fmt.Printf("  Warning: discovery failed: %v\n", err)
				break
			}

			// Add discovered posts to manifest under lock
			mu.Lock()
			added := 0
			for _, post := range posts {
				if added >= remaining {
					break
				}
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
			mu.Unlock()
			markDirty()
			fmt.Printf("Added %d new threads to session\n", added)
		}
		fmt.Printf("  Discovery completed in %s\n", formatDuration(time.Since(discoveryStart)))

		// Feed newly pending threads to workers
		mu.Lock()
		var newItems []workItem
		for _, ts := range manifest.Threads {
			if ts.Status == "pending" && !fed[ts.PostID] {
				newItems = append(newItems, workItem{ts, true})
				fed[ts.PostID] = true
			}
		}
		mu.Unlock()

		if len(newItems) == 0 && round > 0 {
			fmt.Println("No new threads to process, stopping")
			break
		}

		fmt.Println("\n=== Phase 2+3: Evaluate & Extract ===")
		emitPhase(config, "evaluate-extract")
		fmt.Printf("Feeding %d threads to %d workers\n", len(newItems), workers)
		evalExtractStart := time.Now()
		totalFed.Add(int64(len(newItems)))
		for _, item := range newItems {
			if ctx.Err() != nil {
				break
			}
			workCh <- item
		}

		// Wait for this round's items to be consumed before deciding on next round
		roundTarget := totalFed.Load()
		for {
			if ctx.Err() != nil {
				break
			}
			if done.Load() >= roundTarget {
				break
			}
			mu.Lock()
			counts = session.CountByStatus(manifest)
			haveEnough = counts["extracted"]+counts["ranked"] >= config.Limit
			mu.Unlock()
			if haveEnough {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		fmt.Printf("  Evaluate & Extract completed in %s (%d extracted)\n",
			formatDuration(time.Since(evalExtractStart)), extracted.Load())
		mu.Lock()
		counts = session.CountByStatus(manifest)
		mu.Unlock()
		fmt.Printf("  Round status: %d extracted, %d skipped, %d failed, %d pending\n",
			counts["extracted"], counts["skipped"], counts["failed"], counts["pending"])

		// Circuit breaker: if first round produced zero extractions and everything failed, abort
		if extracted.Load() == 0 && round == 0 {
			mu.Lock()
			counts = session.CountByStatus(manifest)
			failCount := counts["failed"] + counts["skipped"]
			total := failCount + counts["extracted"]
			mu.Unlock()
			if total > 0 && failCount == total {
				fmt.Printf("\n=== Circuit breaker: all %d threads failed or were skipped with 0 extracted. Aborting. ===\n", failCount)
				break
			}
		}
	}

	close(workCh)
	wg.Wait()

	// Final manifest save
	saveCancel()
	<-saveDone

	fmt.Printf("Extraction log: %s\n", logPath)
	return processed, nil
}

func (o *DefaultOrchestrator) loadThreadForExtraction(ctx context.Context, ts types.ThreadState, sessionDir string) (*types.Thread, error) {
	threadPath := filepath.Join(sessionDir, fmt.Sprintf("thread_%s.json", ts.PostID))
	threadData, readErr := os.ReadFile(threadPath)
	if readErr == nil {
		thread, parseErr := parseThreadJSON(threadData)
		if parseErr == nil {
			return thread, nil
		}
		fmt.Printf("  [%s] thread payload invalid (%v), refetching canonical JSON\n", ts.PostID, parseErr)
	} else if !os.IsNotExist(readErr) {
		fmt.Printf("  [%s] thread payload unreadable (%v), refetching canonical JSON\n", ts.PostID, readErr)
	}

	thread, err := o.searcher.GetThread(ctx, ts.Permalink, 100)
	if err != nil {
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("refetch failed after read error (%v): %w", readErr, err)
		}
		return nil, fmt.Errorf("refetch failed: %w", err)
	}

	canonical, err := json.MarshalIndent(thread, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling canonical thread JSON: %w", err)
	}
	if err := os.WriteFile(threadPath, canonical, 0644); err != nil {
		return nil, fmt.Errorf("writing canonical thread JSON: %w", err)
	}
	fmt.Printf("  [%s] refetched thread and wrote canonical payload\n", ts.PostID)

	return thread, nil
}

func parseThreadJSON(data []byte) (*types.Thread, error) {
	var thread types.Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		return nil, err
	}
	if thread.Post.ID == "" || thread.Post.Permalink == "" {
		return nil, fmt.Errorf("missing post id/permalink in payload")
	}
	return &thread, nil
}

// findThreads discovers threads using the agentic discoverer or direct search.
// Returns posts without modifying the manifest — the caller handles that under lock.
func (o *DefaultOrchestrator) findThreads(ctx context.Context, config RunConfig, remaining int, sessionDir string) ([]types.Post, error) {
	if o.threadDiscoverer != nil {
		fmt.Printf("Agent discovering %d threads across %v\n", remaining, config.Subreddits)

		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return nil, fmt.Errorf("creating session dir: %w", err)
		}

		posts, err := o.threadDiscoverer.DiscoverThreads(ctx, config.Form, config.Query, config.Subreddits, remaining, sessionDir)
		if err != nil {
			fmt.Printf("  Warning: agentic discovery failed: %v\n", err)
			fmt.Println("  Falling back to direct search")
			return o.searchDirect(ctx, config, remaining)
		}
		return posts, nil
	}

	return o.searchDirect(ctx, config, remaining)
}

// searchDirect performs parallel API searches across subreddits
func (o *DefaultOrchestrator) searchDirect(ctx context.Context, config RunConfig, remaining int) ([]types.Post, error) {
	if config.Query != "" {
		if len(config.Subreddits) == 0 {
			fmt.Printf("Searching all of Reddit for: %s\n", config.Query)
			posts, err := o.searcher.Search(ctx, config.Query, "all", remaining)
			if err != nil {
				return nil, err
			}
			fmt.Printf("  Found %d posts\n", len(posts))
			return posts, nil
		}

		// Parallel search across subreddits
		var (
			posts []types.Post
			mu    sync.Mutex
			wg    sync.WaitGroup
		)
		for _, sub := range config.Subreddits {
			wg.Add(1)
			go func(sub string) {
				defer wg.Done()
				if ctx.Err() != nil {
					return
				}
				fmt.Printf("Searching r/%s for: %s\n", sub, config.Query)
				subPosts, err := o.searcher.Search(ctx, config.Query, sub, remaining)
				if err != nil {
					fmt.Printf("  Warning: search failed for r/%s: %v\n", sub, err)
					return
				}
				mu.Lock()
				posts = append(posts, subPosts...)
				mu.Unlock()
				fmt.Printf("  Found %d posts in r/%s\n", len(subPosts), sub)
			}(sub)
		}
		wg.Wait()
		return posts, nil
	}

	// List mode — parallel across subreddits
	var (
		posts []types.Post
		mu    sync.Mutex
		wg    sync.WaitGroup
	)
	for _, sub := range config.Subreddits {
		wg.Add(1)
		go func(sub string) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			fmt.Printf("Listing r/%s (%s)\n", sub, config.Sort)
			subPosts, err := o.searcher.ListSubreddit(ctx, sub, config.Sort, remaining)
			if err != nil {
				fmt.Printf("  Warning: list failed for r/%s: %v\n", sub, err)
				return
			}
			mu.Lock()
			posts = append(posts, subPosts...)
			mu.Unlock()
			fmt.Printf("  Found %d posts in r/%s\n", len(subPosts), sub)
		}(sub)
	}
	wg.Wait()
	return posts, nil
}

// rankEntries collects all extracted entries and runs them through the ranker
func (o *DefaultOrchestrator) rankEntries(ctx context.Context, config RunConfig, manifest *types.Manifest, sessionDir string) (int, error) {
	// Collect entries from all extracted threads
	var inputs []agent.RankInput
	for _, ts := range manifest.Threads {
		if ts.Status != "extracted" || len(ts.Entries) == 0 {
			continue
		}
		for j, entry := range ts.Entries {
			inputs = append(inputs, agent.RankInput{
				ThreadPostID: ts.PostID,
				EntryIndex:   j,
				Entry:        entry,
				ThreadScore:  ts.Score,
				NumComments:  ts.NumComments,
			})
		}
	}

	if len(inputs) == 0 {
		fmt.Println("  No entries to rank")
		return 0, nil
	}

	fmt.Printf("  Ranking %d entries from %d threads\n", len(inputs), len(session.GetExtractedThreads(manifest)))

	outputs, err := o.ranker.RankEntries(ctx, config.Form, inputs)
	if err != nil {
		return 0, err
	}

	// Write scores back to entries in the manifest
	for _, out := range outputs {
		idx := session.FindThreadIndex(manifest, out.ThreadPostID)
		if idx < 0 {
			continue
		}
		thread := &manifest.Threads[idx]
		if out.EntryIndex < 0 || out.EntryIndex >= len(thread.Entries) {
			continue
		}
		score := out.FinalScore
		thread.Entries[out.EntryIndex].RankScore = &score
		if len(out.Flags) > 0 {
			thread.Entries[out.EntryIndex].RankFlags = out.Flags
		}
		if out.Reason != "" {
			thread.Entries[out.EntryIndex].RankReason = out.Reason
		}
	}

	// Update thread statuses to "ranked"
	for _, ts := range manifest.Threads {
		if ts.Status == "extracted" && len(ts.Entries) > 0 {
			session.UpdateThreadRanked(manifest, ts.PostID)
		}
	}

	if err := session.SaveManifest(sessionDir, manifest); err != nil {
		return 0, fmt.Errorf("saving manifest after ranking: %w", err)
	}

	return len(outputs), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Millisecond)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}
