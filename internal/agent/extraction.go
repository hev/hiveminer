package agent

import (
	"context"
	"fmt"
	"io"
	"io/fs"

	claude "go-claude"

	"hiveminer/pkg/types"
)

// ClaudeExtractor implements Extractor using the Claude CLI
type ClaudeExtractor struct {
	runner  Runner
	prompts fs.FS
	model   string
	logger  claude.EventHandler
}

// NewClaudeExtractor creates a new Claude CLI extractor
func NewClaudeExtractor(runner Runner, prompts fs.FS, model string, logger claude.EventHandler) *ClaudeExtractor {
	return &ClaudeExtractor{
		runner:  runner,
		prompts: prompts,
		model:   model,
		logger:  logger,
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

	// Build run options
	opts := []claude.RunOption{
		claude.WithModel(c.model),
		claude.WithMaxOutputTokens(64000),
	}
	if c.logger != nil {
		opts = append(opts, claude.WithEventHandler(c.logger))
	}
	if output != nil {
		opts = append(opts, claude.WithOutputStream(output))
	}

	// Call Claude CLI
	result, err := c.runner.Run(ctx, prompt, opts...)
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	// Parse the response
	parsed, err := c.parseResponse(result.Text, form)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Build comment links from evidence
	populateLinks(parsed, thread.Post.Permalink)

	return parsed, nil
}

// renderPrompt renders the extraction prompt template
func (c *ClaudeExtractor) renderPrompt(thread *types.Thread, form *types.Form) (string, error) {
	pt, err := claude.LoadPromptTemplate(c.prompts, "extract.md", nil)
	if err != nil {
		return "", fmt.Errorf("loading prompt template: %w", err)
	}

	// Format comments
	var comments string
	for _, comment := range flattenComments(thread.Comments) {
		comments += fmt.Sprintf("[comment_id:%s][%d points] u/%s:\n%s\n\n", comment.ID, comment.Score, comment.Author, comment.Body)
	}

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
		Comments:        comments,
		Fields:          form.Fields,
	}

	return pt.Render(data)
}

// parseResponse parses Claude's JSON response into extraction results
func (c *ClaudeExtractor) parseResponse(response string, form *types.Form) (*types.ExtractionResult, error) {
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

	if err := claude.ExtractJSON(response, &parsed); err != nil {
		return nil, fmt.Errorf("extracting JSON: %w", err)
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

// populateLinks builds Reddit comment permalink arrays on each field and entry
// from the comment_ids found in evidence.
func populateLinks(result *types.ExtractionResult, postPermalink string) {
	if postPermalink == "" {
		return
	}
	// Ensure trailing slash
	if postPermalink[len(postPermalink)-1] != '/' {
		postPermalink += "/"
	}

	for i := range result.Entries {
		seen := map[string]bool{}
		for j := range result.Entries[i].Fields {
			fieldSeen := map[string]bool{}
			for _, ev := range result.Entries[i].Fields[j].Evidence {
				cid := ev.CommentID
				if cid == "" || cid == "post_content" {
					continue
				}
				link := postPermalink + cid + "/"
				if !fieldSeen[link] {
					fieldSeen[link] = true
					result.Entries[i].Fields[j].Links = append(result.Entries[i].Fields[j].Links, link)
				}
				if !seen[link] {
					seen[link] = true
				}
			}
		}
		// Entry-level deduped links
		for link := range seen {
			result.Entries[i].Links = append(result.Entries[i].Links, link)
		}
	}
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
