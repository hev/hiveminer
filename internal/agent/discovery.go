package agent

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	claude "go-claude"

	"threadminer/pkg/types"
)

// ClaudeDiscoverer uses Claude CLI to agentically discover subreddits
type ClaudeDiscoverer struct {
	runner  Runner
	prompts fs.FS
	model   string
}

// NewClaudeDiscoverer creates a new Claude-based subreddit discoverer
func NewClaudeDiscoverer(runner Runner, prompts fs.FS, model string) *ClaudeDiscoverer {
	return &ClaudeDiscoverer{runner: runner, prompts: prompts, model: model}
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

	result, err := d.runner.Run(ctx, prompt,
		claude.WithAllowedTools(fmt.Sprintf("Bash(%s *)", executable)),
		claude.WithMaxTurns(15),
		claude.WithModel(d.model),
	)
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	return d.parseResponse(result.Text)
}

func (d *ClaudeDiscoverer) renderPrompt(form *types.Form, query string, executable string) (string, error) {
	pt, err := claude.LoadPromptTemplate(d.prompts, "discover_subreddits.md", nil)
	if err != nil {
		return "", fmt.Errorf("loading template: %w", err)
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

	return pt.Render(data)
}

func (d *ClaudeDiscoverer) parseResponse(response string) ([]string, error) {
	var parsed discoveryResponse
	if err := claude.ExtractJSON(response, &parsed); err != nil {
		return nil, fmt.Errorf("extracting JSON: %w", err)
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
