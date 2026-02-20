# hiveminer

Extract structured data from Reddit using cascading retrieval — a multi-phase pipeline where AI agent swarms discover, evaluate, extract, and rank information from community discussions.

Define what you're looking for with a form, point it at Reddit, and get back ranked, deduplicated results with confidence scores and source links.

### Prerequisites

- Go 1.25+
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex) installed and authenticated

## Quick Start

```bash
# Build
make build

# Install to $GOPATH/bin
make install

# Run an extraction
hiveminer run --form forms/family-vacation.json

# Run with Codex backend
hiveminer run --form forms/family-vacation.json --codex
```

Results are printed inline and saved to `./output/`.

### Creating a Form

A form defines what to extract. It's a JSON file with a title, description, search hints, and fields:

```json
{
  "title": "Family Vacation Destinations",
  "description": "Compare family-friendly vacation destinations recommended on Reddit",
  "search_hints": ["best family vacation", "where to travel with kids"],
  "fields": [
    {
      "id": "destination",
      "type": "string",
      "question": "What destination is being recommended?",
      "required": true
    },
    {
      "id": "best_season",
      "type": "string",
      "question": "What time of year is recommended for visiting?"
    },
    {
      "id": "kid_activities",
      "type": "array",
      "question": "What kid-friendly activities are mentioned?"
    }
  ]
}
```

Field types: `string`, `number`, `boolean`, `array`. Fields marked `required` are weighted more heavily in ranking. The `search_hints` at both form and field level guide thread discovery queries. See `forms/` for more examples.

## Key Concepts

### Cascading Retrieval

Hiveminer uses **cascading retrieval** — each phase narrows a broad search into structured, ranked results. Rather than a single query-and-parse step, the pipeline progressively filters signal from noise across multiple AI-driven stages:

```
Millions of Reddit posts
        ↓  Phase 0: Subreddit Discovery
    5–10 target subreddits
        ↓  Phase 1: Thread Discovery
   ~60 candidate threads
        ↓  Phase 2: Thread Evaluation
   ~20 extractable threads
        ↓  Phase 3: Field Extraction
   ~30–50 raw entries
        ↓  Phase 4: Entry Ranking
    Top ranked results
```

Each phase acts as a filter. Discovery casts a wide net, evaluation discards noise, extraction structures what remains, and ranking surfaces the best results. The cascade means expensive operations (extraction, LLM quality assessment) only run on content that's already survived cheaper filters.

### Context Management

Each phase of the pipeline gets a **focused context window** — only the information it needs to do its job well. This is deliberate: rather than dumping everything into a single massive prompt, hiveminer scopes each agent's context to maximize signal density.

**Discovery phases** (0 and 1) receive the form title, description, and search hints. The agent uses these to devise varied search queries, but never sees raw thread content — it only needs to find where relevant discussions live.

**Evaluation** (phase 2) receives the form's field definitions alongside a single thread. The agent fetches and reads the thread content through tool use, deciding whether it contains extractable data. Each evaluator sees exactly one thread, keeping the decision focused.

**Extraction** (phase 3) is where context allocation matters most. Each extractor receives:
- The form's field definitions (what to look for)
- A single thread's full content: post body, author, score, and a flattened comment tree with `[comment_id:xxx]` tags on every comment

This one-thread-per-agent design means the model's full attention goes toward extracting fields from that thread's discussion. There's no cross-thread confusion, no competing signals from unrelated conversations. The comment ID tags create an evidence chain — extracted values link back to specific comments and authors.

**Ranking** (phase 4) sees only the extracted field values and algorithmic scores — not the original thread content. The ranker doesn't re-read threads; it assesses quality and diversity across the already-structured entries.

This scoped approach means each agent operates with high signal-to-noise in its context, and the pipeline can use different models per phase (heavier models for discovery and evaluation where planning matters, lighter models for extraction and ranking where the task is more constrained).

### Agent Swarms

Phases 2 and 3 run as parallel agent swarms — a configurable pool of workers (default 10, max 50) that process threads concurrently. Discovery streams threads into a buffered work channel, and workers pick them up as they become available across multiple discovery rounds. Evaluation and extraction happen while discovery is still finding new threads, minimizing total pipeline time.

## How It Works

### The Pipeline

```
 Form + Query
      |
      v
 Phase 0: Subreddit Discovery ──── find where the conversation lives
      |
      v
 Phase 1: Thread Discovery ──────── find the best threads across subreddits
      |
      v
 Phase 2: Thread Evaluation ─────── keep or skip (parallel agent swarm)
      |
      v
 Phase 3: Field Extraction ──────── extract structured entries (parallel agent swarm)
      |
      v
 Phase 4: Entry Ranking ─────────── score, deduplicate, and rank
      |
      v
 Ranked Results
```

**Phase 0 — Subreddit Discovery.** Given a search query, an agent searches Reddit with multiple phrasings to identify which subreddits contain the most relevant discussions. Skipped if subreddits are provided explicitly via `--subreddits`.

**Phase 1 — Thread Discovery.** An agent searches target subreddits with varied queries derived from form-level and field-level search hints, browses top/hot listings, and selects the most promising threads based on comment count, title relevance, and discussion quality. Discovery runs in up to 3 rounds, streaming threads to workers as they're found.

**Phase 2 — Thread Evaluation.** An agent swarm evaluates threads in parallel. Each agent fetches a thread, reads its content, and makes a keep/skip decision based on whether the thread contains extractable data for the form's fields. This filters out off-topic, shallow, or link-only threads before the more expensive extraction phase.

**Phase 3 — Field Extraction.** Another agent swarm processes kept threads in parallel. Each agent extracts multiple entries per thread — one per distinct recommendation, product, destination, or whatever the form defines. Every field value includes a confidence score (0–1) and evidence quotes linking back to specific comments and authors.

**Phase 4 — Entry Ranking.** All extracted entries are scored through a hybrid algorithmic + LLM approach.

### Ranking

Entries are ranked by a composite score combining algorithmic signals with LLM-based quality assessment.

**Algorithmic score (0–100):**

| Signal | Weight | How it's calculated |
|--------|--------|-------------------|
| Confidence | 40% | Average confidence across extracted fields |
| Completeness | 25% | Filled fields ratio (required fields weighted 2x) |
| Thread upvotes | 20% | Log-scaled, caps at ~1000 |
| Comment count | 15% | Log-scaled, caps at ~500 |

**Penalties:**

- **Diversity penalty.** Entries are grouped by their primary field value using normalized string matching. Duplicates are penalized: -15 for the second-best, -25 for third, up to -50 for redundant copies. This prevents "Walt Disney World" from appearing five times because five threads mentioned it.
- **Thread saturation penalty.** When multiple entries come from the same thread, all but the best are penalized (-5 to -30). One thread shouldn't dominate results.
- **LLM quality assessment.** Claude reviews entries and applies penalties for spam, jokes, outdated info, off-topic content, and low-effort mentions (-10 to -50).

Final score: `max(0, algorithmic_score + penalties)`

## CLI Reference

```bash
# Run extraction
hiveminer run --form <path> [flags]

  -q, --query           Search query (inferred from form if omitted)
  -r, --subreddits      Comma-separated subreddit list (skips Phase 0)
  -l, --limit           Target number of entries (default: 20)
  -o, --output          Output directory (default: ./output)
      --workers         Concurrent extraction workers (default: 10, max: 50)
      --sort            Subreddit sort: hot, new, top, rising (default: hot)
      --discovery-model Model for discovery phases (default: opus)
      --eval-model      Model for evaluation (default: opus)
      --extract-model   Model for extraction (default: haiku)
      --rank-model      Model for ranking (default: haiku)
      --codex           Use Codex backend instead of Claude
  -v, --verbose         Show full agent logs

# Run with Codex backend
hiveminer run --form forms/family-vacation.json --codex

# View past runs
hiveminer runs ls [-o ./output]
hiveminer runs show <run-id> [-n 10]

# Debug: search Reddit directly
hiveminer search "query" [-r subreddit]
hiveminer ls <subreddit> [-s hot]
hiveminer thread <permalink>
```

### Backends

Hiveminer supports two backends: **Claude** (default) and **Codex**.

```bash
# Claude (default)
hiveminer run --form forms/family-vacation.json

# Codex
hiveminer run --form forms/family-vacation.json --codex
```

With `--codex`, model defaults switch automatically:

| Phase | Claude default | Codex default |
|-------|---------------|---------------|
| Discovery (0+1) | opus | codex CLI default |
| Evaluation (2) | opus | codex CLI default |
| Extraction (3) | haiku | gpt-5.1-codex-mini |
| Ranking (4) | haiku | gpt-5.1-codex-mini |

You can override any model explicitly regardless of backend: `--extract-model gpt-5.1-codex-mini`.

Codex does not support agentic options (`WithMaxTurns`, `WithAllowedTools`, `WithDisallowedTools`, `WithMaxOutputTokens`), so these are automatically omitted when using the codex backend.

### Session Resumption

Each run creates a session directory under `./output/`. Running the same query again resumes from where it left off — discovered subreddits, collected threads, and completed extractions are reused. Only missing phases are re-run.
