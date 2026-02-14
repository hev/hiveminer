package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"threadminer/internal/agent"
	"threadminer/internal/schema"
	"threadminer/internal/search"
	"threadminer/internal/session"
	"threadminer/pkg/types"
)

// DefaultOrchestrator implements the extraction pipeline
type DefaultOrchestrator struct {
	searcher   search.Searcher
	extractor  agent.Extractor
	discoverer agent.Discoverer
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

// Run executes the full extraction pipeline
func (o *DefaultOrchestrator) Run(ctx context.Context, config RunConfig) error {
	// Create session directory
	slug := session.GenerateSlugFromQuery(config.Query)
	if config.Query == "" && len(config.Subreddits) > 0 {
		slug = session.GenerateSlug(config.Subreddits[0])
	}
	sessionDir := filepath.Join(config.OutputDir, slug)

	// Check for existing session or create new
	manifest, err := session.LoadManifest(sessionDir)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if manifest == nil {
		// Create new session
		formHash, err := schema.HashForm(config.Form)
		if err != nil {
			return fmt.Errorf("hashing form: %w", err)
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
		return fmt.Errorf("saving manifest: %w", err)
	}

	// Phase 0: Subreddit Discovery
	if config.Query != "" && len(config.Subreddits) == 0 {
		if manifest.DiscoveredSubreddits && len(manifest.Subreddits) > 0 {
			fmt.Printf("Reusing %d previously discovered subreddits\n", len(manifest.Subreddits))
			config.Subreddits = manifest.Subreddits
		} else {
			fmt.Println("\n=== Phase 0: Subreddit Discovery ===")
			if o.discoverer == nil {
				o.discoverer = agent.NewClaudeDiscoverer("prompts")
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
					return fmt.Errorf("saving manifest: %w", err)
				}
			}
		}
	}

	// Phase 1: Discovery
	fmt.Println("\n=== Phase 1: Discovery ===")
	if err := o.discover(ctx, config, manifest, sessionDir); err != nil {
		if ctx.Err() != nil {
			session.CompleteRun(manifest, "interrupted", 0)
			session.SaveManifest(sessionDir, manifest)
			return ctx.Err()
		}
		return fmt.Errorf("discovery: %w", err)
	}

	// Phase 2: Collection
	fmt.Println("\n=== Phase 2: Collection ===")
	if err := o.collect(ctx, manifest, sessionDir); err != nil {
		if ctx.Err() != nil {
			session.CompleteRun(manifest, "interrupted", 0)
			session.SaveManifest(sessionDir, manifest)
			return ctx.Err()
		}
		return fmt.Errorf("collection: %w", err)
	}

	// Phase 3: Extraction
	fmt.Println("\n=== Phase 3: Extraction ===")
	processed, err := o.extract(ctx, config, manifest, sessionDir)
	if err != nil {
		if ctx.Err() != nil {
			session.CompleteRun(manifest, "interrupted", processed)
			session.SaveManifest(sessionDir, manifest)
			return ctx.Err()
		}
		return fmt.Errorf("extraction: %w", err)
	}

	// Complete run
	session.CompleteRun(manifest, "completed", processed)
	if err := session.SaveManifest(sessionDir, manifest); err != nil {
		return fmt.Errorf("saving final manifest: %w", err)
	}

	// Print summary
	counts := session.CountByStatus(manifest)
	fmt.Printf("\n=== Complete ===\n")
	fmt.Printf("Session: %s\n", sessionDir)
	fmt.Printf("Threads: %d total\n", len(manifest.Threads))
	fmt.Printf("  - Extracted: %d\n", counts["extracted"])
	fmt.Printf("  - Failed: %d\n", counts["failed"])
	fmt.Printf("\nView results: threadminer serve %s\n", sessionDir)

	return nil
}

// discover finds threads matching the search criteria
func (o *DefaultOrchestrator) discover(ctx context.Context, config RunConfig, manifest *types.Manifest, sessionDir string) error {
	// Skip if we already have threads
	if len(manifest.Threads) >= config.Limit {
		fmt.Printf("Already have %d threads, skipping discovery\n", len(manifest.Threads))
		return nil
	}

	remaining := config.Limit - len(manifest.Threads)
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

	// Add new posts to manifest
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

// collect fetches full thread content
func (o *DefaultOrchestrator) collect(ctx context.Context, manifest *types.Manifest, sessionDir string) error {
	pending := session.GetPendingThreads(manifest)
	if len(pending) == 0 {
		fmt.Println("No threads to collect")
		return nil
	}

	fmt.Printf("Collecting %d threads\n", len(pending))

	for i, threadState := range pending {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fmt.Printf("  [%d/%d] %s\n", i+1, len(pending), truncate(threadState.Title, 50))

		// Fetch full thread
		thread, err := o.searcher.GetThread(ctx, threadState.Permalink, 100)
		if err != nil {
			fmt.Printf("    Warning: failed to fetch: %v\n", err)
			session.UpdateThreadStatus(manifest, threadState.PostID, "failed")
			continue
		}

		// Save thread JSON
		threadPath := filepath.Join(sessionDir, fmt.Sprintf("thread_%s.json", threadState.PostID))
		threadData, err := json.MarshalIndent(thread, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling thread: %w", err)
		}
		if err := os.WriteFile(threadPath, threadData, 0644); err != nil {
			return fmt.Errorf("writing thread: %w", err)
		}

		// Update status
		now := time.Now()
		idx := session.FindThreadIndex(manifest, threadState.PostID)
		if idx >= 0 {
			manifest.Threads[idx].Status = "collected"
			manifest.Threads[idx].CollectedAt = &now
		}
	}

	if err := session.SaveManifest(sessionDir, manifest); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

// extract runs Claude extraction on collected threads
func (o *DefaultOrchestrator) extract(ctx context.Context, config RunConfig, manifest *types.Manifest, sessionDir string) (int, error) {
	if o.extractor == nil {
		// Create default extractor
		o.extractor = agent.NewClaudeExtractor("prompts")
	}

	collected := session.GetCollectedThreads(manifest)
	if len(collected) == 0 {
		fmt.Println("No threads to extract")
		return 0, nil
	}

	fmt.Printf("Extracting %d threads\n", len(collected))
	processed := 0

	for i, threadState := range collected {
		if ctx.Err() != nil {
			return processed, ctx.Err()
		}

		fmt.Printf("  [%d/%d] %s\n", i+1, len(collected), truncate(threadState.Title, 50))

		// Load thread JSON
		threadPath := filepath.Join(sessionDir, fmt.Sprintf("thread_%s.json", threadState.PostID))
		threadData, err := os.ReadFile(threadPath)
		if err != nil {
			fmt.Printf("    Warning: failed to read thread: %v\n", err)
			session.UpdateThreadStatus(manifest, threadState.PostID, "failed")
			continue
		}

		var thread types.Thread
		if err := json.Unmarshal(threadData, &thread); err != nil {
			fmt.Printf("    Warning: failed to parse thread: %v\n", err)
			session.UpdateThreadStatus(manifest, threadState.PostID, "failed")
			continue
		}

		// Run extraction
		result, err := o.extractor.ExtractFields(ctx, &thread, config.Form)
		if err != nil {
			fmt.Printf("    Warning: extraction failed: %v\n", err)
			idx := session.FindThreadIndex(manifest, threadState.PostID)
			if idx >= 0 {
				manifest.Threads[idx].Status = "failed"
				manifest.Threads[idx].Error = err.Error()
			}
			continue
		}

		// Update manifest with results
		session.UpdateThreadFields(manifest, threadState.PostID, result.Fields)
		processed++

		// Save manifest after each extraction
		if err := session.SaveManifest(sessionDir, manifest); err != nil {
			return processed, fmt.Errorf("saving manifest: %w", err)
		}

		fmt.Printf("    Extracted %d fields\n", len(result.Fields))
	}

	return processed, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
