package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"threadminer/pkg/types"
)

const manifestFile = "manifest.json"

// NewManifest creates a new empty manifest
func NewManifest(formRef types.FormRef, query string, subreddits []string) *types.Manifest {
	now := time.Now()
	return &types.Manifest{
		Version:    1,
		Form:       formRef,
		Query:      query,
		Subreddits: subreddits,
		Threads:    []types.ThreadState{},
		Runs:       []types.RunLog{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// LoadManifest loads a manifest from a session directory
func LoadManifest(dir string) (*types.Manifest, error) {
	path := filepath.Join(dir, manifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No manifest yet
		}
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest types.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &manifest, nil
}

// SaveManifest saves a manifest to a session directory
func SaveManifest(dir string, manifest *types.Manifest) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}

	manifest.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	path := filepath.Join(dir, manifestFile)
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming manifest: %w", err)
	}

	return nil
}

// FindThread finds a thread by post ID in the manifest
func FindThread(manifest *types.Manifest, postID string) *types.ThreadState {
	for i := range manifest.Threads {
		if manifest.Threads[i].PostID == postID {
			return &manifest.Threads[i]
		}
	}
	return nil
}

// FindThreadIndex finds the index of a thread by post ID
func FindThreadIndex(manifest *types.Manifest, postID string) int {
	for i := range manifest.Threads {
		if manifest.Threads[i].PostID == postID {
			return i
		}
	}
	return -1
}

// AddThread adds a new thread to the manifest
func AddThread(manifest *types.Manifest, thread types.ThreadState) {
	manifest.Threads = append(manifest.Threads, thread)
	manifest.UpdatedAt = time.Now()
}

// UpdateThreadStatus updates the status of a thread
func UpdateThreadStatus(manifest *types.Manifest, postID, status string) bool {
	for i := range manifest.Threads {
		if manifest.Threads[i].PostID == postID {
			manifest.Threads[i].Status = status
			manifest.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// UpdateThreadEntries updates the extracted entries for a thread
func UpdateThreadEntries(manifest *types.Manifest, postID string, entries []types.Entry) bool {
	for i := range manifest.Threads {
		if manifest.Threads[i].PostID == postID {
			now := time.Now()
			manifest.Threads[i].Entries = entries
			manifest.Threads[i].Status = "extracted"
			manifest.Threads[i].ExtractedAt = &now
			manifest.UpdatedAt = now
			return true
		}
	}
	return false
}

// CountByStatus counts threads by status
func CountByStatus(manifest *types.Manifest) map[string]int {
	counts := map[string]int{
		"pending":   0,
		"collected": 0,
		"extracted": 0,
		"failed":    0,
		"skipped":   0,
	}
	for _, t := range manifest.Threads {
		counts[t.Status]++
	}
	return counts
}

// GetPendingThreads returns threads that haven't been collected yet
func GetPendingThreads(manifest *types.Manifest) []types.ThreadState {
	var pending []types.ThreadState
	for _, t := range manifest.Threads {
		if t.Status == "pending" {
			pending = append(pending, t)
		}
	}
	return pending
}

// GetCollectedThreads returns threads that have been collected but not extracted
func GetCollectedThreads(manifest *types.Manifest) []types.ThreadState {
	var collected []types.ThreadState
	for _, t := range manifest.Threads {
		if t.Status == "collected" {
			collected = append(collected, t)
		}
	}
	return collected
}

// StartRun creates a new run log entry
func StartRun(manifest *types.Manifest, invocationID string) {
	manifest.Runs = append(manifest.Runs, types.RunLog{
		InvocationID: invocationID,
		StartedAt:    time.Now(),
		Status:       "running",
	})
	manifest.UpdatedAt = time.Now()
}

// CompleteRun marks the current run as complete
func CompleteRun(manifest *types.Manifest, status string, threadsProcessed int) {
	if len(manifest.Runs) == 0 {
		return
	}
	idx := len(manifest.Runs) - 1
	manifest.Runs[idx].CompletedAt = time.Now()
	manifest.Runs[idx].Status = status
	manifest.Runs[idx].ThreadsProcessed = threadsProcessed
	manifest.UpdatedAt = time.Now()
}
