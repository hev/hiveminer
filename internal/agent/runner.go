package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// RunOpts configures a Claude CLI invocation
type RunOpts struct {
	AllowedTools []string
	MaxTurns     int
	Model        string    // default "sonnet"
	Output       io.Writer // nil = stdout
}

// ClaudeRunner executes the Claude CLI and streams output
type ClaudeRunner struct{}

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

// Run executes the Claude CLI with the given prompt and options.
// It streams assistant text to stdout in subdued color and returns the final result.
func (r *ClaudeRunner) Run(ctx context.Context, prompt string, opts RunOpts) (string, error) {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
	}

	for _, tool := range opts.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}

	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
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

	// Determine output destination
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	// Stream Claude output in subdued color
	fmt.Fprint(out, colorDim)
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
					fmt.Fprint(out, block.Text)
				}
			}
		} else if event.Type == "result" && event.Result != "" {
			responseBuilder.Reset()
			responseBuilder.WriteString(event.Result)
		}
	}
	fmt.Fprint(out, colorReset+"\n")

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
