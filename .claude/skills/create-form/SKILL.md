---
name: create-form
description: Create hiveminer extraction forms interactively. Use when the user wants to research, find, compare, or rank something on Reddit using hiveminer. Triggers on phrases like "find the best X", "research Y on Reddit", "compare Z", or "create a form for".
argument-hint: "[topic]"
---

# Interactive Form Builder

You are helping the user design a hiveminer extraction form. Threadminer mines Reddit threads to extract and compare structured data — the form defines *what* to extract.

Your job is to have a **conversation** to understand what the user cares about, then generate a form. Don't ask the user to list fields — ask about their goals and preferences, then translate those into fields yourself.

## If arguments were provided

The user's topic is: $ARGUMENTS

Skip asking what they want to research. Go straight to unpacking their criteria.

## Step 1: Understand the Goal

If no topic was provided, ask what they want to find or research on Reddit.

Listen for the **subject** (pokemon, laptops, cities) and the **intent** (ranking, comparison, decision-making, recommendation).

## Step 2: Unpack the Criteria

This is the core step. The user said "best" or "recommended" — but what does that mean to them?

Use `AskUserQuestion` with `multiSelect: true` to explore what dimensions matter. Generate 3-4 options based on what typically matters for the subject.

Example for "best pokemon":
- Competitive viability (tier ranking, win rate, meta relevance)
- Design & aesthetics (looks, shiny forms, fan favorites)
- In-game usability (story mode, ease of obtaining, versatility)
- Nostalgia & community sentiment (beloved by fans, iconic status)

**Then drill deeper** on what they selected with follow-up questions. Keep using `AskUserQuestion` — multi-select options are faster than open-ended text.

## Step 3: Determine the Primary Entity

Every form needs a `required: true` field that identifies the primary thing being extracted. If it's not obvious from context, ask using `AskUserQuestion`:

"What's the main thing you want to extract — one entry per ___?"

## Step 4: Propose Fields

Based on the conversation, propose 5-9 fields as a readable table:

| Field | Type | What it captures |
|-------|------|-----------------|
| **pokemon_name** | string | Which pokemon (required) |
| **tier** | string | Competitive tier |
| **strengths** | array | What makes it good |
| ... | ... | ... |

Ask: "Want to add, remove, or change anything?"

Let the user iterate until they're happy.

## Step 5: Generate and Save

Generate the form JSON and save to `forms/<slug>.json`.

### Form JSON rules

- **title**: Short noun phrase (e.g., "Competitive Pokemon", not "Best Pokemon Research Form")
- **description**: One sentence on what's being extracted and compared
- **search_hints**: 4-6 Reddit search terms using casual language and common phrasings
- **fields**: Each field needs:
  - `id`: snake_case identifier
  - `type`: `string`, `number`, `boolean`, or `array`
  - `question`: Specific extraction prompt telling Claude what to look for in thread comments
  - `search_hints` (optional): Keywords that help find relevant comments
  - `required: true` on the primary identifier field only
  - `internal: true` on metadata fields hidden from display (like `mention_count`)
- Use `array` for lists (pros, cons, features). Use `string` for summaries or specific values. Use `number` only for truly numeric data.

### Reference: existing form examples

See `forms/android-phones.json` and `forms/family-vacation.json` for the expected format. Read these if you need to check the structure.

## Step 6: Suggest a Run Command

After saving, suggest how to run it:

```
hiveminer run --form forms/<slug>.json
```

If the conversation revealed specific subreddits or search terms, include those with `-q` and `-r` flags.

## Key Principles

- **Don't ask the user to list fields.** Ask about goals, translate into fields yourself.
- **Use `AskUserQuestion` liberally.** Multi-select options are faster than open-ended text.
- **Be opinionated.** Propose a reasonable default, let the user adjust.
- **Think about what Reddit has.** Focus on opinions, experiences, comparisons — not official specs or stats.
- **Keep it conversational.** Planning with a friend, not filling out a form about forms.
