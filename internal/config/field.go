package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Form field types.
const (
	FieldBool FieldType = iota
	FieldString
)

var errUnknownField = errors.New("unknown form field type")

// FieldType is the type of a form field.
type FieldType int

// UnmarshalJSON implements [encoding/json.Unmarshaler].
func (t *FieldType) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("failed to unmarshal form field type: %w", err)
	}

	return t.parse(s)
}

func (t *FieldType) parse(s string) error {
	switch strings.ToLower(s) {
	case "bool", "boolean":
		*t = FieldBool
	case "string":
		*t = FieldString
	default:
		return errUnknownField
	}

	return nil
}
