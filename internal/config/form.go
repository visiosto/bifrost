// Copyright 2025 Visiosto oy
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Form body content types.
const (
	ContentJSON ContentType = iota
)

// Form field types.
const (
	FieldBool FieldType = iota
	FieldString
)

var (
	errUnknownContentType = errors.New("unknown form content type")
	errUnknownField       = errors.New("unknown form field type")
)

// ContentType is the type of the request body that the form uses.
type ContentType int

// FieldType is the type of a form field.
type FieldType int //nolint:recvcheck // no need to have pointer receiver for all functions

// FormField is the configuration for a single form field.
type FormField struct {
	DisplayName string    `json:"displayName"`
	Type        FieldType `json:"type"`
	Min         int       `json:"min"`
	Max         int       `json:"max"`
	Required    bool      `json:"required"`
}

// UnmarshalJSON implements [encoding/json.Unmarshaler].
func (t *ContentType) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("failed to unmarshal form content type: %w", err)
	}

	return t.parse(s)
}

func (t *ContentType) parse(s string) error {
	switch strings.ToLower(s) {
	case "json":
		*t = ContentJSON
	default:
		return fmt.Errorf("%w: %s", errUnknownContentType, s)
	}

	return nil
}

// UnmarshalJSON implements [encoding/json.Unmarshaler].
func (t *FieldType) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("failed to unmarshal form field type: %w", err)
	}

	return t.parse(s)
}

func (t FieldType) String() string {
	switch t {
	case FieldBool:
		return "bool"
	case FieldString:
		return "string"
	default:
		return "invalid-type"
	}
}

func (t *FieldType) parse(s string) error {
	switch strings.ToLower(s) {
	case "bool", "boolean":
		*t = FieldBool
	case "string":
		*t = FieldString
	default:
		return fmt.Errorf("%w: %s", errUnknownField, s)
	}

	return nil
}
