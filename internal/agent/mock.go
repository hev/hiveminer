package agent

import (
	"context"

	"threadminer/pkg/types"
)

// MockRanker implements Ranker for testing
type MockRanker struct {
	Results []RankOutput
	Err     error
}

// NewMockRanker creates a new mock ranker
func NewMockRanker() *MockRanker {
	return &MockRanker{}
}

// RankEntries returns mock ranking results with basic algorithmic scores
func (m *MockRanker) RankEntries(ctx context.Context, form *types.Form, entries []RankInput) ([]RankOutput, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Results != nil {
		return m.Results, nil
	}

	// Return basic scores
	outputs := make([]RankOutput, len(entries))
	for i, input := range entries {
		outputs[i] = RankOutput{
			ThreadPostID: input.ThreadPostID,
			EntryIndex:   input.EntryIndex,
			AlgoScore:    50,
			FinalScore:   50,
		}
	}
	return outputs, nil
}

// MockExtractor implements Extractor for testing
type MockExtractor struct {
	Results map[string]*types.ExtractionResult
	Err     error
}

// NewMockExtractor creates a new mock extractor
func NewMockExtractor() *MockExtractor {
	return &MockExtractor{
		Results: make(map[string]*types.ExtractionResult),
	}
}

// ExtractFields returns mock extraction results
func (m *MockExtractor) ExtractFields(ctx context.Context, thread *types.Thread, form *types.Form) (*types.ExtractionResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	if result, ok := m.Results[thread.Post.ID]; ok {
		return result, nil
	}

	// Return empty result with one entry containing all fields
	fields := make([]types.FieldValue, len(form.Fields))
	for i, field := range form.Fields {
		fields[i] = types.FieldValue{
			ID:         field.ID,
			Value:      nil,
			Confidence: 0,
		}
	}

	result := &types.ExtractionResult{
		Entries: []types.Entry{{Fields: fields}},
	}

	return result, nil
}
