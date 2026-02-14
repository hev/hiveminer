You are evaluating whether a Reddit thread contains useful data for extraction.

## Form: {{.FormTitle}}
{{.FormDescription}}

### Fields to extract
{{- range .Fields}}
- **{{.ID}}** ({{.Type}}): {{.Question}}
{{- end}}

## Thread to evaluate
Title: {{.ThreadTitle}}
Permalink: {{.Permalink}}

## Instructions

1. Fetch the thread using: `{{.Executable}} thread {{.Permalink}} --json -l 100`
2. Read through the post content and comments
3. Evaluate whether this thread contains information relevant to the form fields above
4. Consider:
   - Does the thread discuss topics related to the form fields?
   - Are there specific recommendations, reviews, or comparisons?
   - Do comments contain substantive discussion (not just jokes/memes)?
   - Could you extract at least one meaningful entry from this thread?

## Decision

- **keep**: Thread contains extractable data for the form fields
- **skip**: Thread is off-topic, too shallow, or doesn't contain relevant information

## Output

If the verdict is **keep**, first save the full thread JSON to: `{{.ThreadPath}}`
You can do this by piping the command output: `{{.Executable}} thread {{.Permalink}} --json -l 100 > {{.ThreadPath}}`

Then write your evaluation to: `{{.EvalPath}}`

```json
{
  "post_id": "{{.PostID}}",
  "verdict": "keep or skip",
  "reason": "Brief explanation of your decision",
  "estimated_entries": 3,
  "thread_saved": true
}
```

Set `thread_saved` to true only if you wrote the thread JSON file. Set `estimated_entries` to your estimate of how many distinct items could be extracted (0 if skipping).
