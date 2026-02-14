package agent

import (
	"context"

	"threadminer/pkg/types"
)

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

	// Return empty result with all fields
	result := &types.ExtractionResult{
		Fields: make([]types.FieldValue, len(form.Fields)),
	}

	for i, field := range form.Fields {
		result.Fields[i] = types.FieldValue{
			ID:         field.ID,
			Value:      nil,
			Confidence: 0,
		}
	}

	return result, nil
}
