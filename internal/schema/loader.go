package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"threadminer/pkg/types"
)

// LoadForm loads and validates a form from a JSON file
func LoadForm(path string) (*types.Form, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading form file: %w", err)
	}

	var form types.Form
	if err := json.Unmarshal(data, &form); err != nil {
		return nil, fmt.Errorf("parsing form JSON: %w", err)
	}

	if err := Validate(&form); err != nil {
		return nil, fmt.Errorf("validating form: %w", err)
	}

	return &form, nil
}

// Validate validates a form schema
func Validate(form *types.Form) error {
	if form.Title == "" {
		return fmt.Errorf("form title is required")
	}

	if len(form.Fields) == 0 {
		return fmt.Errorf("form must have at least one field")
	}

	seen := make(map[string]bool)
	for i, field := range form.Fields {
		if field.ID == "" {
			return fmt.Errorf("field %d: id is required", i)
		}

		if seen[field.ID] {
			return fmt.Errorf("duplicate field id: %s", field.ID)
		}
		seen[field.ID] = true

		if !IsValidFieldType(field.Type) {
			return fmt.Errorf("field %s: invalid type %q", field.ID, field.Type)
		}

		if field.Question == "" {
			return fmt.Errorf("field %s: question is required", field.ID)
		}
	}

	return nil
}

// HashForm computes a hash of the form schema for change detection
func HashForm(form *types.Form) (string, error) {
	data, err := json.Marshal(form)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]), nil
}

// GetFieldIDs returns all field IDs from a form
func GetFieldIDs(form *types.Form) []string {
	ids := make([]string, len(form.Fields))
	for i, f := range form.Fields {
		ids[i] = f.ID
	}
	return ids
}

// GetField finds a field by ID
func GetField(form *types.Form, id string) *types.Field {
	for i := range form.Fields {
		if form.Fields[i].ID == id {
			return &form.Fields[i]
		}
	}
	return nil
}
