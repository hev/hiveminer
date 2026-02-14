package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"threadminer/pkg/types"
)

// ClaudeDiscoverer uses Claude CLI to agentically discover subreddits
type ClaudeDiscoverer struct {
	promptDir string
	model     string
	runner    ClaudeRunner
}

// NewClaudeDiscoverer creates a new Claude-based subreddit discoverer
func NewClaudeDiscoverer(promptDir, model string) *ClaudeDiscoverer {
	return &ClaudeDiscoverer{promptDir: promptDir, model: model}
}

type discoveryResponse struct {
	Subreddits []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"subreddits"`
}

// DiscoverSubreddits uses Claude to search Reddit and identify the best subreddits
func (d *ClaudeDiscoverer) DiscoverSubreddits(ctx context.Context, form *types.Form, query string) ([]string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("getting executable path: %w", err)
	}

	prompt, err := d.renderPrompt(form, query, executable)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	response, err := d.runner.Run(ctx, prompt, RunOpts{
		AllowedTools: []string{fmt.Sprintf("Bash(%s *)", executable)},
		MaxTurns:     15,
		Model:        d.model,
	})
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	return d.parseResponse(response)
}

func (d *ClaudeDiscoverer) renderPrompt(form *types.Form, query string, executable string) (string, error) {
	tmplPath := filepath.Join(d.promptDir, "discover_subreddits.md")
	tmplData, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("discover").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	data := struct {
		FormTitle       string
		FormDescription string
		SearchHints     string
		Query           string
		Executable      string
	}{
		FormTitle:       form.Title,
		FormDescription: form.Description,
		SearchHints:     strings.Join(form.SearchHints, ", "),
		Query:           query,
		Executable:      executable,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

func (d *ClaudeDiscoverer) parseResponse(response string) ([]string, error) {
	// Strip markdown code fences that LLMs sometimes wrap around JSON
	response = stripCodeFences(response)

	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var parsed discoveryResponse
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w, json: %s", err, jsonStr)
	}

	if len(parsed.Subreddits) == 0 {
		return nil, fmt.Errorf("no subreddits in response")
	}

	names := make([]string, len(parsed.Subreddits))
	for i, s := range parsed.Subreddits {
		names[i] = s.Name
	}

	return names, nil
}
