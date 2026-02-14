package schema

import (
	"threadminer/pkg/types"
)

// FieldType constants for validation
const (
	FieldTypeString  = types.FieldTypeString
	FieldTypeNumber  = types.FieldTypeNumber
	FieldTypeBoolean = types.FieldTypeBoolean
	FieldTypeArray   = types.FieldTypeArray
)

// ValidFieldTypes is the set of valid field types
var ValidFieldTypes = map[types.FieldType]bool{
	FieldTypeString:  true,
	FieldTypeNumber:  true,
	FieldTypeBoolean: true,
	FieldTypeArray:   true,
}

// IsValidFieldType checks if a field type is valid
func IsValidFieldType(t types.FieldType) bool {
	return ValidFieldTypes[t]
}
