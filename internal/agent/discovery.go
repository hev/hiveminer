package agent

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"

	claude "go-claude"

	"hiveminer/pkg/types"
)

// ClaudeDiscoverer uses Claude CLI to agentically discover subreddits
type ClaudeDiscoverer struct {
	runner  Runner
	prompts fs.FS
	model   string
	logger  claude.EventHandler
}

// NewClaudeDiscoverer creates a new Claude-based subreddit discoverer
func NewClaudeDiscoverer(runner Runner, prompts fs.FS, model string, logger claude.EventHandler) *ClaudeDiscoverer {
	return &ClaudeDiscoverer{runner: runner, prompts: prompts, model: model, logger: logger}
}

type discoveryResponse struct {
	Subreddits []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"subreddits"`
}

var subredditRefRegex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])r/([a-z0-9_]{2,21})\b`)

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

	opts := []claude.RunOption{
		claude.WithAllowedTools(fmt.Sprintf("Bash(%s *)", executable)),
		claude.WithDisallowedTools("WebSearch", "WebFetch"),
		claude.WithMaxTurns(15),
		claude.WithModel(d.model),
	}
	if d.logger != nil {
		opts = append(opts, claude.WithEventHandler(d.logger))
	}
	result, err := d.runner.Run(ctx, prompt, opts...)
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	names, err := d.parseResponse(result.Text)
	if err != nil {
		// Fast fallback: extract subreddit names directly from freeform text.
		heuristic := extractSubredditNames(result.Text)
		if len(heuristic) > 0 {
			return heuristic, nil
		}

		// Retry: make a single non-agentic call to extract subreddit names from the response
		fmt.Println("  JSON extraction failed, retrying with formatting call...")
		names, err = d.retryFormat(ctx, result.Text)
		if err != nil {
			return nil, fmt.Errorf("retry format also failed: %w", err)
		}
	}

	return names, nil
}

// retryFormat makes a single non-agentic Claude call to extract subreddit names
// from the agent's response text into structured JSON.
func (d *ClaudeDiscoverer) retryFormat(ctx context.Context, responseText string) ([]string, error) {
	prompt := fmt.Sprintf(`Extract all subreddit names mentioned in the following text and return them as JSON.

Text:
%s

Return ONLY this JSON (no other text):
{"subreddits": [{"name": "subredditname", "reason": "why it was mentioned"}]}

Do not include the r/ prefix in names. Order from most relevant to least relevant.`, responseText)

	result, err := d.runner.Run(ctx, prompt,
		claude.WithModel(d.model),
		claude.WithMaxTurns(1),
	)
	if err != nil {
		return nil, fmt.Errorf("retry format call: %w", err)
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
	if err := claude.ExtractJSON(response, &parsed); err == nil {
		names := normalizeSubredditNames(extractNamesFromResponse(parsed))
		if len(names) > 0 {
			return names, nil
		}
	}

	var objectStringList struct {
		Subreddits []string `json:"subreddits"`
	}
	if err := claude.ExtractJSON(response, &objectStringList); err == nil {
		names := normalizeSubredditNames(objectStringList.Subreddits)
		if len(names) > 0 {
			return names, nil
		}
	}

	var listObjects []struct {
		Name string `json:"name"`
	}
	if err := claude.ExtractJSONArray(response, &listObjects); err == nil {
		raw := make([]string, 0, len(listObjects))
		for _, v := range listObjects {
			raw = append(raw, v.Name)
		}
		names := normalizeSubredditNames(raw)
		if len(names) > 0 {
			return names, nil
		}
	}

	var listStrings []string
	if err := claude.ExtractJSONArray(response, &listStrings); err == nil {
		names := normalizeSubredditNames(listStrings)
		if len(names) > 0 {
			return names, nil
		}
	}

	names := extractSubredditNames(response)
	if len(names) == 0 {
		return nil, fmt.Errorf("no subreddits in response")
	}

	return names, nil
}

func extractNamesFromResponse(resp discoveryResponse) []string {
	names := make([]string, 0, len(resp.Subreddits))
	for _, s := range resp.Subreddits {
		names = append(names, s.Name)
	}
	return names
}

func extractSubredditNames(text string) []string {
	matches := subredditRefRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	raw := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			raw = append(raw, m[1])
		}
	}
	return normalizeSubredditNames(raw)
}

func normalizeSubredditNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		clean := normalizeSubredditName(name)
		if clean == "" || seen[strings.ToLower(clean)] {
			continue
		}
		seen[strings.ToLower(clean)] = true
		out = append(out, clean)
	}
	return out
}

func normalizeSubredditName(name string) string {
	s := strings.TrimSpace(name)
	s = strings.TrimPrefix(strings.ToLower(s), "r/")
	s = strings.Trim(s, " \t\r\n\"'`.,;:!?()[]{}")
	if len(s) < 2 || len(s) > 21 {
		return ""
	}
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		return ""
	}
	return s
}
