# Known Shortcomings

Last updated: 2026-02-17

This file tracks known issues that should be addressed in future sessions.

## Recently Fixed

- Phase 0 subreddit discovery now has robust fallback parsing for non-JSON/freeform LLM responses (`internal/agent/discovery.go`).
- CLI now supports `--json` for `search`, `ls`, and `thread`, aligning runtime behavior with agent prompt instructions (`cmd/hiveminer/cmd/search.go`).

## Open Items

1. Ranking penalties are overwritten in Claude assessment
- File: `internal/agent/ranking.go`
- Issue: Claude penalty assignment replaces previously applied diversity/thread penalties instead of accumulating all penalties.
- Impact: Final ranking can ignore duplicate/thread-saturation penalties.
- Suggested fix: Add Claude penalties to existing `Penalty` and recalculate `FinalScore` from total penalty.

2. Worker early-exit can stall producer under load
- File: `internal/orchestrator/orchestrator.go`
- Issue: Workers return permanently once target extracted count is reached, while producer can continue sending to `workCh`.
- Impact: Potential blocking when channel fills and workers are gone.
- Suggested fix: Keep workers draining channel, or coordinate shutdown via cancellation/close semantics before further sends.

3. Round progress uses dequeued count instead of completed count
- File: `internal/orchestrator/orchestrator.go`
- Issue: Round completion checks `done` when items are picked up, not when extraction completes.
- Impact: Retry-round logic can advance too early.
- Suggested fix: Track a separate completion counter incremented after success/fail terminal status updates.

4. Background manifest save errors are ignored
- File: `internal/orchestrator/orchestrator.go`
- Issue: Periodic and final saver goroutine calls to `session.SaveManifest` drop errors.
- Impact: Silent durability failures.
- Suggested fix: Capture saver errors and surface them through a shared error channel/atomic state.

5. `runs ls` order does not match stated sort intent
- File: `cmd/hiveminer/cmd/runs.go`
- Issue: Data is sorted newest-first but printed in reverse order.
- Impact: UX confusion when scanning recent runs.
- Suggested fix: Iterate in forward order after current sort, or sort ascending intentionally.

6. No automated tests yet
- Scope: repository-wide.
- Issue: `go test ./...` compiles but has no test files.
- Impact: High regression risk.
- Suggested fix: Add unit tests for ranking accumulation, discovery parsing fallback, and orchestrator channel/round behavior.
