package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"threadminer/pkg/types"
)

// ClaudeExtractor implements Extractor using the Claude CLI
type ClaudeExtractor struct {
	promptDir string
	model     string
	runner    ClaudeRunner
}

// NewClaudeExtractor creates a new Claude CLI extractor
func NewClaudeExtractor(promptDir, model string) *ClaudeExtractor {
	return &ClaudeExtractor{
		promptDir: promptDir,
		model:     model,
	}
}

// ExtractFields extracts all form fields from a thread using Claude
func (c *ClaudeExtractor) ExtractFields(ctx context.Context, thread *types.Thread, form *types.Form) (*types.ExtractionResult, error) {
	return c.ExtractFieldsWithOutput(ctx, thread, form, nil)
}

// ExtractFieldsWithOutput extracts fields, directing streaming LLM output to the given writer.
// If output is nil, streaming goes to stdout.
func (c *ClaudeExtractor) ExtractFieldsWithOutput(ctx context.Context, thread *types.Thread, form *types.Form, output io.Writer) (*types.ExtractionResult, error) {
	// Render the extraction prompt
	prompt, err := c.renderPrompt(thread, form)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	// Call Claude CLI
	response, err := c.runner.Run(ctx, prompt, RunOpts{
		Model:  c.model,
		Output: output,
	})
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	// Parse the response
	result, err := c.parseResponse(response, form)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return result, nil
}

// renderPrompt renders the extraction prompt template
func (c *ClaudeExtractor) renderPrompt(thread *types.Thread, form *types.Form) (string, error) {
	// Load template
	tmplPath := filepath.Join(c.promptDir, "extract.md")
	tmplData, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("reading prompt template: %w", err)
	}

	tmpl, err := template.New("extract").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("parsing prompt template: %w", err)
	}

	// Format comments
	var commentsBuilder strings.Builder
	for _, comment := range flattenComments(thread.Comments) {
		commentsBuilder.WriteString(fmt.Sprintf("[comment_id:%s][%d points] u/%s:\n%s\n\n", comment.ID, comment.Score, comment.Author, comment.Body))
	}

	// Build template data
	data := struct {
		FormTitle       string
		FormDescription string
		ThreadTitle     string
		Subreddit       string
		Author          string
		Score           int
		PostContent     string
		Comments        string
		Fields          []types.Field
	}{
		FormTitle:       form.Title,
		FormDescription: form.Description,
		ThreadTitle:     thread.Post.Title,
		Subreddit:       thread.Post.Subreddit,
		Author:          thread.Post.Author,
		Score:           thread.Post.Score,
		PostContent:     thread.Post.Selftext,
		Comments:        commentsBuilder.String(),
		Fields:          form.Fields,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// parseResponse parses Claude's JSON response into extraction results
func (c *ClaudeExtractor) parseResponse(response string, form *types.Form) (*types.ExtractionResult, error) {
	// Strip markdown code fences that LLMs sometimes wrap around JSON
	response = stripCodeFences(response)

	// Find JSON in the response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var parsed struct {
		Entries []struct {
			Fields []struct {
				ID         string     `json:"id"`
				Value      any        `json:"value"`
				Confidence float64    `json:"confidence"`
				Evidence   []evidence `json:"evidence"`
			} `json:"fields"`
		} `json:"entries"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w, json: %s", err, jsonStr)
	}

	result := &types.ExtractionResult{
		Entries: make([]types.Entry, 0, len(parsed.Entries)),
	}

	for _, entry := range parsed.Entries {
		fields := make([]types.FieldValue, 0, len(entry.Fields))
		for _, f := range entry.Fields {
			ev := make([]types.Evidence, len(f.Evidence))
			for i, e := range f.Evidence {
				ev[i] = types.Evidence{
					Text:      e.Text,
					CommentID: e.CommentID,
					Author:    e.Author,
				}
			}

			fields = append(fields, types.FieldValue{
				ID:         f.ID,
				Value:      f.Value,
				Confidence: f.Confidence,
				Evidence:   ev,
			})
		}
		result.Entries = append(result.Entries, types.Entry{Fields: fields})
	}

	return result, nil
}

type evidence struct {
	Text      string `json:"text"`
	CommentID string `json:"comment_id,omitempty"`
	Author    string `json:"author,omitempty"`
}

// flattenComments flattens nested comments into a list
func flattenComments(comments []*types.Comment) []*types.Comment {
	var result []*types.Comment
	for _, c := range comments {
		result = append(result, c)
		if len(c.Replies) > 0 {
			result = append(result, flattenComments(c.Replies)...)
		}
	}
	return result
}
