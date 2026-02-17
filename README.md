# hiveminer

Extract structured data from Reddit using cascading retrieval — a multi-phase pipeline where AI agent swarms discover, evaluate, extract, and rank information from community discussions.

Define what you're looking for with a form, point it at Reddit, and get back ranked, deduplicated results with confidence scores and source links.

## Quick Start

```bash
# Build
make build

# Install to $GOPATH/bin
make install

# Run an extraction
hiveminer run --form forms/family-vacation.json -q "best family vacation destinations"
```

Results are printed inline and saved to `./output/`.

## How It Works

Hiveminer uses **cascading retrieval** — a technique where each phase narrows and refines a broad search into structured, ranked results. Rather than a single query-and-parse step, the pipeline progressively filters signal from noise across multiple AI-driven stages.

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

**Phase 0 — Subreddit Discovery.** Given a search query, an agent searches Reddit with multiple phrasings to identify which subreddits contain the most relevant discussions. Skipped if subreddits are provided explicitly.

**Phase 1 — Thread Discovery.** An agent searches target subreddits with varied queries, browses top/hot listings, and selects the most promising threads based on comment count, title relevance, and discussion quality. Discovery runs in up to 3 rounds, streaming threads to workers as they're found.

**Phase 2 — Thread Evaluation.** An **agent swarm** evaluates threads in parallel. Each agent fetches a thread, reads its content, and makes a keep/skip decision based on whether the thread contains extractable data for the form's fields. This filters out off-topic, shallow, or link-only threads before the more expensive extraction phase.

**Phase 3 — Field Extraction.** Another **agent swarm** processes kept threads in parallel. Each agent extracts multiple entries per thread — one per distinct recommendation, product, destination, or whatever the form defines. Every field value includes a confidence score (0–1) and evidence quotes linking back to specific comments and authors.

**Phase 4 — Entry Ranking.** All extracted entries are scored through a hybrid algorithmic + LLM approach:

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
- **LLM quality assessment.** Claude reviews entries flagged for potential issues and applies penalties for spam, jokes, outdated info, off-topic content, and low-effort mentions (-10 to -50).

Final score: `max(0, algorithmic_score + penalties)`

### Agent Swarms

Phases 2 and 3 run as parallel agent swarms — a configurable pool of workers (default 10, max 50) that process threads concurrently. Discovery feeds threads into a buffered work channel, and workers pick them up continuously across multiple discovery rounds. This means evaluation and extraction happen while discovery is still finding new threads, minimizing total pipeline time.

## Forms

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

Field types: `string`, `number`, `boolean`, `array`. See `forms/` for examples.

## CLI Reference

```bash
# Run extraction
hiveminer run --form <path> [flags]

  -q, --query           Search query (inferred from form if omitted)
  -r, --subreddits      Comma-separated subreddit list
  -l, --limit           Target number of entries (default: 20)
  -o, --output          Output directory (default: ./output)
      --workers         Concurrent extraction workers (default: 10)
      --sort            Subreddit sort: hot, new, top, rising (default: hot)
      --discovery-model Model for discovery phases (default: opus)
      --eval-model      Model for evaluation (default: opus)
      --extract-model   Model for extraction (default: haiku)
      --rank-model      Model for ranking (default: haiku)
  -v, --verbose         Show full agent logs

# View past runs
hiveminer runs ls [-o ./output]
hiveminer runs show <run-id> [-n 10]
```
