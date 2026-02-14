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
For each field, provide:
1. The extracted value (or null if not found)
2. Confidence score (0.0-1.0)
3. Evidence: quote the relevant text

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

Respond ONLY with valid JSON in this format:
```json
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
          "comment_id": "optional"
        }
      ]
    }
  ]
}
```
