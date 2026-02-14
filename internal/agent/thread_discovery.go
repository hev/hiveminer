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

// ClaudeThreadDiscoverer uses Claude CLI to agentically discover relevant threads
type ClaudeThreadDiscoverer struct {
	promptDir string
	model     string
	runner    ClaudeRunner
}

// NewClaudeThreadDiscoverer creates a new Claude-based thread discoverer
func NewClaudeThreadDiscoverer(promptDir, model string) *ClaudeThreadDiscoverer {
	return &ClaudeThreadDiscoverer{promptDir: promptDir, model: model}
}

// discoveryResult is the JSON structure the agent writes to the output file
type discoveryResult struct {
	Posts []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Permalink   string `json:"permalink"`
		Subreddit   string `json:"subreddit"`
		Score       int    `json:"score"`
		NumComments int    `json:"num_comments"`
		Reason      string `json:"reason"`
	} `json:"posts"`
	SearchLog []struct {
		Query     string `json:"query"`
		Subreddit string `json:"subreddit"`
		Results   int    `json:"results"`
	} `json:"search_log"`
}

// DiscoverThreads uses Claude to search Reddit and identify the most relevant threads
func (d *ClaudeThreadDiscoverer) DiscoverThreads(ctx context.Context, form *types.Form, query string, subreddits []string, limit int, sessionDir string) ([]types.Post, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("getting executable path: %w", err)
	}

	outputPath := filepath.Join(sessionDir, "discovery_results.json")

	prompt, err := d.renderPrompt(form, query, subreddits, limit, executable, outputPath)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	_, err = d.runner.Run(ctx, prompt, RunOpts{
		AllowedTools: []string{
			fmt.Sprintf("Bash(%s *)", executable),
			fmt.Sprintf("Write(%s/*)", sessionDir),
		},
		MaxTurns: 25,
		Model:    d.model,
	})
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	// Parse the output file
	return d.parseOutputFile(outputPath)
}

func (d *ClaudeThreadDiscoverer) renderPrompt(form *types.Form, query string, subreddits []string, limit int, executable string, outputPath string) (string, error) {
	tmplPath := filepath.Join(d.promptDir, "discover_threads.md")
	tmplData, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	funcMap := template.FuncMap{
		"joinHints": func(hints []string) string {
			return strings.Join(hints, ", ")
		},
	}

	tmpl, err := template.New("discover_threads").Funcs(funcMap).Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	data := struct {
		FormTitle       string
		FormDescription string
		SearchHints     string
		Fields          []types.Field
		Query           string
		Subreddits      string
		TargetCount     int
		Executable      string
		OutputPath      string
	}{
		FormTitle:       form.Title,
		FormDescription: form.Description,
		SearchHints:     strings.Join(form.SearchHints, ", "),
		Fields:          form.Fields,
		Query:           query,
		Subreddits:      strings.Join(subreddits, ", "),
		TargetCount:     limit,
		Executable:      executable,
		OutputPath:      outputPath,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

func (d *ClaudeThreadDiscoverer) parseOutputFile(path string) ([]types.Post, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading discovery results: %w", err)
	}

	var result discoveryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing discovery results: %w", err)
	}

	if len(result.Posts) == 0 {
		return nil, fmt.Errorf("no threads found in discovery results")
	}

	// Log search activity
	for _, entry := range result.SearchLog {
		fmt.Printf("  Searched r/%s for '%s': %d results\n", entry.Subreddit, entry.Query, entry.Results)
	}

	posts := make([]types.Post, len(result.Posts))
	for i, p := range result.Posts {
		posts[i] = types.Post{
			ID:          p.ID,
			Title:       p.Title,
			Permalink:   p.Permalink,
			Subreddit:   p.Subreddit,
			Score:       p.Score,
			NumComments: p.NumComments,
		}
	}

	return posts, nil
}
