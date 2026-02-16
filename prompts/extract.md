You are extracting structured information from a Reddit thread.

## Form: {{.FormTitle}}
{{.FormDescription}}

## Thread
Title: {{.ThreadTitle}}
Subreddit: r/{{.Subreddit}}
Author: u/{{.Author}}
Score: {{.Score}}

### Post Content
{{.PostContent}}

### Comments
{{.Comments}}

## Fields to Extract
{{range .Fields}}
- **{{.ID}}** ({{.Type}}): {{.Question}}
{{end}}

## Instructions

This thread may contain **multiple distinct recommendations or items**. Extract each one as a separate entry. Each entry should represent a single, specific item (e.g., one destination, one product, one recommendation) with its own complete set of fields.

**CRITICAL**: Do NOT combine multiple items into a single entry. If a thread discusses 5 different destinations, return 5 separate entries — one per destination. Each entry must have exactly one primary item.

For each entry, extract every field listed above. For each field provide:
1. The extracted value (or null if not found for this entry)
2. Confidence score (0.0-1.0)
3. Evidence: quote the relevant text, including the comment_id from the `[comment_id:xxx]` tag preceding the comment

### Confidence Guidelines
- **0.9-1.0**: Explicit, clear statement with multiple supporting comments
- **0.7-0.9**: Clear recommendation with some supporting comments
- **0.5-0.7**: Single mention or implied recommendation
- **0.3-0.5**: Weak or ambiguous mention
- **0.0-0.3**: Not found or contradictory information

### Value Types
- **string**: Extract text value
- **number**: Extract numeric value
- **boolean**: true/false based on thread content
- **array**: Extract multiple values as a JSON array

### Entry Guidelines
- Extract at most **20 entries** per thread, prioritizing those with the most discussion and highest confidence
- Only include entries where there is meaningful information (at least the primary/required field has a value)
- If a commenter mentions a place/item only in passing without detail, you may still include it but with lower confidence
- Entries with more discussion and supporting comments should have higher confidence

Respond ONLY with valid JSON in this format:
```json
{
  "entries": [
    {
      "fields": [
        {
          "id": "field_id",
          "value": "extracted value or null",
          "confidence": 0.85,
          "evidence": [
            {
              "text": "quote from thread",
              "author": "username",
              "comment_id": "the comment_id from the [comment_id:xxx] tag"
            }
          ]
        }
      ]
    }
  ]
}
```
