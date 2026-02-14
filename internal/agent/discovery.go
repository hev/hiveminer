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

// ClaudeDiscoverer uses Claude CLI to agentically discover subreddits
type ClaudeDiscoverer struct {
	promptDir string
}

// NewClaudeDiscoverer creates a new Claude-based subreddit discoverer
func NewClaudeDiscoverer(promptDir string) *ClaudeDiscoverer {
	return &ClaudeDiscoverer{promptDir: promptDir}
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

	response, err := d.callClaude(ctx, prompt, executable)
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

func (d *ClaudeDiscoverer) callClaude(ctx context.Context, prompt string, executable string) (string, error) {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--allowedTools", fmt.Sprintf("Bash(%s *)", executable),
		"--max-turns", "15",
		"--model", "sonnet",
	}

	cmd := exec.CommandContext(ctx, "claude", args...)

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
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
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
			responseBuilder.Reset()
			responseBuilder.WriteString(event.Result)
		}
	}
	fmt.Print(colorReset + "\n")

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
