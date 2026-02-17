package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	claude "go-claude"

	"hiveminer/pkg/types"
)

// ClaudeEvaluator uses Claude CLI to evaluate individual thread relevance
type ClaudeEvaluator struct {
	runner  Runner
	prompts fs.FS
	model   string
	logger  claude.EventHandler
}

// NewClaudeEvaluator creates a new Claude-based thread evaluator
func NewClaudeEvaluator(runner Runner, prompts fs.FS, model string, logger claude.EventHandler) *ClaudeEvaluator {
	return &ClaudeEvaluator{runner: runner, prompts: prompts, model: model, logger: logger}
}

// evalFileResult is the JSON structure the agent writes to the eval output file
type evalFileResult struct {
	PostID           string `json:"post_id"`
	Verdict          string `json:"verdict"`
	Reason           string `json:"reason"`
	EstimatedEntries int    `json:"estimated_entries"`
	ThreadSaved      bool   `json:"thread_saved"`
}

// EvaluateThread uses Claude to fetch, read, and evaluate a single thread
func (e *ClaudeEvaluator) EvaluateThread(ctx context.Context, form *types.Form, thread types.ThreadState, sessionDir string) (*EvalResult, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("getting executable path: %w", err)
	}

	evalPath := filepath.Join(sessionDir, fmt.Sprintf("eval_%s.json", thread.PostID))
	threadPath := filepath.Join(sessionDir, fmt.Sprintf("thread_%s.json", thread.PostID))

	prompt, err := e.renderPrompt(form, thread, executable, evalPath, threadPath)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	opts := []claude.RunOption{
		claude.WithAllowedTools(
			fmt.Sprintf("Bash(%s *)", executable),
			fmt.Sprintf("Bash(* > %s)", threadPath),
			fmt.Sprintf("Write(%s/*)", sessionDir),
		),
		claude.WithMaxTurns(10),
		claude.WithModel(e.model),
	}
	if e.logger != nil {
		opts = append(opts, claude.WithEventHandler(e.logger))
	}
	_, err = e.runner.Run(ctx, prompt, opts...)
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	// Parse the evaluation output file
	return e.parseEvalFile(evalPath)
}

func (e *ClaudeEvaluator) renderPrompt(form *types.Form, thread types.ThreadState, executable string, evalPath string, threadPath string) (string, error) {
	pt, err := claude.LoadPromptTemplate(e.prompts, "evaluate_thread.md", nil)
	if err != nil {
		return "", fmt.Errorf("loading template: %w", err)
	}

	data := struct {
		FormTitle       string
		FormDescription string
		Fields          []types.Field
		ThreadTitle     string
		Permalink       string
		PostID          string
		Executable      string
		EvalPath        string
		ThreadPath      string
	}{
		FormTitle:       form.Title,
		FormDescription: form.Description,
		Fields:          form.Fields,
		ThreadTitle:     thread.Title,
		Permalink:       thread.Permalink,
		PostID:          thread.PostID,
		Executable:      executable,
		EvalPath:        evalPath,
		ThreadPath:      threadPath,
	}

	return pt.Render(data)
}

func (e *ClaudeEvaluator) parseEvalFile(path string) (*EvalResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading eval result: %w", err)
	}

	var fileResult evalFileResult
	if err := json.Unmarshal(data, &fileResult); err != nil {
		return nil, fmt.Errorf("parsing eval result: %w", err)
	}

	return &EvalResult{
		PostID:           fileResult.PostID,
		Verdict:          fileResult.Verdict,
		Reason:           fileResult.Reason,
		EstimatedEntries: fileResult.EstimatedEntries,
	}, nil
}
