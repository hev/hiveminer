package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"


	"threadminer/pkg/types"
)

// ClaudeExtractor implements Extractor using the Claude CLI
type ClaudeExtractor struct {
	promptDir string
}

// NewClaudeExtractor creates a new Claude CLI extractor
func NewClaudeExtractor(promptDir string) *ClaudeExtractor {
	return &ClaudeExtractor{
		promptDir: promptDir,
	}
}

// ExtractFields extracts all form fields from a thread using Claude
func (c *ClaudeExtractor) ExtractFields(ctx context.Context, thread *types.Thread, form *types.Form) (*types.ExtractionResult, error) {
	// Render the extraction prompt
	prompt, err := c.renderPrompt(thread, form)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	// Call Claude CLI
	response, err := c.callClaude(ctx, prompt)
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
		commentsBuilder.WriteString(fmt.Sprintf("[%d points] u/%s:\n%s\n\n", comment.Score, comment.Author, comment.Body))
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

// callClaude executes the Claude CLI with the given prompt
func (c *ClaudeExtractor) callClaude(ctx context.Context, prompt string) (string, error) {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
	}

	cmd := exec.CommandContext(ctx, "claude", args...)

	// Capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting claude: %w", err)
	}

	// Stream Claude output in subdued color
	fmt.Print(colorDim)
	var responseBuilder strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		var event streamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Type == "assistant" && event.Message != nil {
			for _, block := range event.Message.Content {
				if block.Type == "text" {
					fmt.Print(block.Text)
				}
			}
		} else if event.Type == "result" && event.Result != "" {
			// Use only the result event for the final response to avoid duplication
			responseBuilder.Reset()
			responseBuilder.WriteString(event.Result)
		}
	}
	fmt.Print(colorReset + "\n")

	// Read any stderr
	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(stderr)

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("claude exited with error: %w, stderr: %s", err, stderrBuf.String())
	}

	return responseBuilder.String(), nil
}

// streamEvent represents a line from Claude's stream-json output
type streamEvent struct {
	Type    string         `json:"type"`
	Message *streamMessage `json:"message,omitempty"`
	Result  string         `json:"result,omitempty"`
}

type streamMessage struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
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
		Fields []struct {
			ID         string     `json:"id"`
			Value      any        `json:"value"`
			Confidence float64    `json:"confidence"`
			Evidence   []evidence `json:"evidence"`
		} `json:"fields"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w, json: %s", err, jsonStr)
	}

	result := &types.ExtractionResult{
		Fields: make([]types.FieldValue, 0, len(parsed.Fields)),
	}

	for _, f := range parsed.Fields {
		ev := make([]types.Evidence, len(f.Evidence))
		for i, e := range f.Evidence {
			ev[i] = types.Evidence{
				Text:      e.Text,
				CommentID: e.CommentID,
				Author:    e.Author,
			}
		}

		result.Fields = append(result.Fields, types.FieldValue{
			ID:         f.ID,
			Value:      f.Value,
			Confidence: f.Confidence,
			Evidence:   ev,
		})
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
