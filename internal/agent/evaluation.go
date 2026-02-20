package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	rack "go-rack"

	"hiveminer/pkg/types"
)

// ClaudeEvaluator uses Claude CLI to evaluate individual thread relevance
type ClaudeEvaluator struct {
	runner  Runner
	prompts fs.FS
	model   string
	logger  rack.EventHandler
}

// NewClaudeEvaluator creates a new Claude-based thread evaluator
func NewClaudeEvaluator(runner Runner, prompts fs.FS, model string, logger rack.EventHandler) *ClaudeEvaluator {
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

	opts := []rack.RunOption{
		rack.WithAllowedTools(
			fmt.Sprintf("Bash(%s *)", executable),
			fmt.Sprintf("Bash(* > %s)", threadPath),
			fmt.Sprintf("Write(%s/*)", sessionDir),
		),
		rack.WithDisallowedTools("WebSearch", "WebFetch"),
		rack.WithMaxTurns(10),
		rack.WithModel(e.model),
	}
	if e.logger != nil {
		opts = append(opts, rack.WithEventHandler(e.logger))
	}
	var lastErr error
	const maxAttempts = 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_ = os.Remove(evalPath)
		_ = os.Remove(threadPath)

		_, err = e.runner.Run(ctx, prompt, opts...)
		if err != nil {
			lastErr = fmt.Errorf("calling claude (attempt %d/%d): %w", attempt, maxAttempts, err)
			if attempt < maxAttempts {
				continue
			}
			return nil, lastErr
		}

		// Parse the evaluation output file.
		result, parseErr := e.parseEvalFile(evalPath)
		if parseErr != nil {
			lastErr = fmt.Errorf("reading eval output (attempt %d/%d): %w", attempt, maxAttempts, parseErr)
			if attempt < maxAttempts {
				continue
			}
			return nil, lastErr
		}

		if result.Verdict == "keep" {
			if !result.ThreadSaved {
				lastErr = fmt.Errorf("keep verdict without saved thread payload (attempt %d/%d)", attempt, maxAttempts)
				if attempt < maxAttempts {
					continue
				}
				return nil, lastErr
			}
			if validateErr := validateThreadFile(threadPath, thread.PostID); validateErr != nil {
				lastErr = fmt.Errorf("invalid saved thread payload (attempt %d/%d): %w", attempt, maxAttempts, validateErr)
				if attempt < maxAttempts {
					continue
				}
				return nil, lastErr
			}
		}

		return result, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("evaluation failed without a specific error")
}

func (e *ClaudeEvaluator) renderPrompt(form *types.Form, thread types.ThreadState, executable string, evalPath string, threadPath string) (string, error) {
	pt, err := rack.LoadPromptTemplate(e.prompts, "evaluate_thread.md", nil)
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
		ThreadSaved:      fileResult.ThreadSaved,
	}, nil
}

func validateThreadFile(path string, expectedPostID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading thread payload: %w", err)
	}

	var thread types.Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		return fmt.Errorf("parsing thread payload JSON: %w", err)
	}
	if thread.Post.ID == "" || thread.Post.Permalink == "" {
		return fmt.Errorf("missing post id/permalink in thread payload")
	}
	if expectedPostID != "" && thread.Post.ID != expectedPostID {
		return fmt.Errorf("thread post id mismatch: expected %s, got %s", expectedPostID, thread.Post.ID)
	}
	return nil
}
