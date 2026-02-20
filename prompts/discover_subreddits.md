You are finding the best subreddits for extracting data about: **{{.FormTitle}}**

## Context
{{.FormDescription}}

Search hints: {{.SearchHints}}
User query: {{.Query}}

## Tool
You have access to `{{.Executable}}` — a Reddit CLI tool. Use it to search Reddit and identify which subreddits contain the most relevant discussions.

**IMPORTANT: Do NOT use web search. Only use the provided CLI tool.**

Commands:
- `{{.Executable}} search "query" -l 10` — search all of Reddit
- `{{.Executable}} search "query" -r subredditname -l 10` — search within a specific subreddit

## Strategy
1. Start with 2-3 broad searches across all of Reddit using different phrasings of the query
2. Look at which subreddits appear in the results — note the ones that show up repeatedly
3. For the most promising subreddits, run a targeted search within them to verify they have relevant content
4. Select the best 5-10 subreddits based on relevance and activity

## What makes a good subreddit
- Posts are directly relevant to the topic (not tangentially related)
- Has active discussion threads with comments (not just link dumps)
- Community focuses on recommendations, reviews, or comparisons
- Appears multiple times across different search queries

## Output
After your research, respond with ONLY this JSON (no other text):
```json
{
  "subreddits": [
    {"name": "subredditname", "reason": "why this subreddit is relevant"}
  ]
}
```

Order from most relevant to least relevant. Do not include the r/ prefix in names.
