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
	FormContentTypeJSON FormContentType = iota
)

// Form field types.
const (
	FormFieldBool FormFieldType = iota
	FormFieldInt
	FormFieldString
	FormFieldObjects
)

var (
	errUnknownContentType = errors.New("unknown form content type")
	errUnknownField       = errors.New("unknown form field type")
)

// FormContentType is the type of the request body that the form uses.
type FormContentType int

// FormFieldType is the type of a form field.
type FormFieldType int //nolint:recvcheck // no need to have pointer receiver for all functions

// Form is the config of a form in a site.
type Form struct {
	ID                  string               `json:"id"`
	Token               string               `json:"token"`
	Fields              map[string]FormField `json:"fields"`
	SMTPNotifiers       []*SMTPNotifier      `json:"smtp"`
	ContentType         FormContentType      `json:"contentType"`
	AccessControlMaxAge int                  `json:"accessControlMaxAge"`
}

// FormField is the configuration for a single form field.
type FormField struct {
	// Shape is the shape of the objects in the array in the field if the type
	// of the field [FormFieldObjects]. The keys are the keys of the objects and
	// the values are the types. There is no further validation for the values
	// for now, and further "objects" are not permitted.
	//
	// TODO: Add validation to make sure that:
	//   1) objects have this,
	//   2) others do not have this
	//   3) objects include the template for printing the value row
	Shape       map[string]FormFieldType `json:"shape"`
	DisplayName string                   `json:"displayName"`

	// DisplayTemplate is a text template that is parsed and executed to display
	// each element of objects.
	DisplayTemplate string        `json:"displayTemplate"`
	Type            FormFieldType `json:"type"`
	Min             int           `json:"min"`
	Max             int           `json:"max"`
	Required        bool          `json:"required"`
}

// SMTPNotifier is the config for a SMTP form notifier.
type SMTPNotifier struct {
	From string `json:"from"`
	To   string `json:"to"`
	Lang string `json:"lang"`

	// Subject is a text template that will be used as the subject of
	// the notification email.
	Subject string `json:"subject"`

	// Intro is a text template that will be used as an intro in
	// the notification email before the form fields.
	Intro          string `json:"intro"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	UsernameEnvVar string `json:"usernameEnv"`
	PasswordEnvVar string `json:"passwordEnv"`
	Host           string `json:"host"`

	// FieldOrder is the order in which the non-hidden form fields should be
	// output to the SMTP notification. If FieldOrder is given, it must contain
	// all of the non-hidden fields.
	FieldOrder []string `json:"fieldOrder"`

	// HiddenFields defines the fields that should not be included in this
	// notification. It must contain all of the fields that are not contained in
	// FieldOrder.
	HiddenFields []string `json:"hiddenFields"`
	Port         int      `json:"port"`
}

// UnmarshalJSON implements [encoding/json.Unmarshaler].
func (t *FormContentType) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("failed to unmarshal form content type: %w", err)
	}

	return t.parse(s)
}

func (t *FormContentType) parse(s string) error {
	switch strings.ToLower(s) {
	case "json":
		*t = FormContentTypeJSON
	default:
		return fmt.Errorf("%w: %s", errUnknownContentType, s)
	}

	return nil
}

// UnmarshalJSON implements [encoding/json.Unmarshaler].
func (t *FormFieldType) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("failed to unmarshal form field type: %w", err)
	}

	return t.parse(s)
}

func (t FormFieldType) String() string {
	switch t {
	case FormFieldBool:
		return "bool"
	case FormFieldInt:
		return "int"
	case FormFieldString:
		return "string"
	case FormFieldObjects:
		return "objects"
	default:
		return "invalid-type"
	}
}

func (t *FormFieldType) parse(s string) error {
	switch strings.ToLower(s) {
	case "bool", "boolean":
		*t = FormFieldBool
	case "int", "number":
		*t = FormFieldInt
	case "string":
		*t = FormFieldString
	case "objects":
		*t = FormFieldObjects
	default:
		return fmt.Errorf("%w: %s", errUnknownField, s)
	}

	return nil
}

func (f *Form) validate() error {
	// TODO: By default, we do not require the form token.
	if f.ID == "" {
		return fmt.Errorf("%w: empty form ID", errConfig)
	}

	if f.AccessControlMaxAge < 0 {
		return fmt.Errorf("%w: accessControlMaxAge must be at least 0", errConfig)
	}

	for _, field := range f.Fields {
		if field.Min < 0 {
			return fmt.Errorf("%w: min field length must be greater than zero", errConfig)
		}

		if field.Max < field.Min {
			return fmt.Errorf("%w: max field length must be greater then the min length", errConfig)
		}
	}

	err := f.validateSMTPNotifiers()
	if err != nil {
		return err
	}

	return nil
}

func (f *Form) validateSMTPNotifiers() error {
	for _, smtp := range f.SMTPNotifiers {
		if smtp.From == "" {
			return fmt.Errorf("%w: empty From address", errConfig)
		}

		if smtp.To == "" {
			return fmt.Errorf("%w: empty To address", errConfig)
		}

		if smtp.Lang == "" {
			return fmt.Errorf("%w: empty language for SMTP form notification", errConfig)
		}

		if smtp.Subject == "" {
			return fmt.Errorf("%w: empty subject for SMTP form notification", errConfig)
		}

		if smtp.Host == "" {
			return fmt.Errorf("%w: empty SMTP host", errConfig)
		}

		if smtp.Port <= 0 {
			return fmt.Errorf("%w: invalid SMTP port %d", errConfig, smtp.Port)
		}

		if smtp.Username == "" && smtp.UsernameEnvVar == "" {
			return fmt.Errorf("%w: no SMTP username or environment variable name provided", errConfig)
		}

		if smtp.Password == "" && smtp.PasswordEnvVar == "" {
			return fmt.Errorf("%w: no SMTP password or environment variable name provided", errConfig)
		}

		err := f.validateSMTPNotifierFields(smtp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *Form) validateSMTPNotifierFields(smtp *SMTPNotifier) error {
	seenFields := map[string]struct{}{}

	for _, name := range smtp.HiddenFields {
		if _, ok := f.Fields[name]; !ok {
			return fmt.Errorf(
				"%w: unknown field name %q in hidden SMTP notification fields of form %q",
				errConfig,
				name,
				f.ID,
			)
		}

		seenFields[name] = struct{}{}
	}

	if smtp.FieldOrder != nil {
		for _, name := range smtp.FieldOrder {
			if _, ok := f.Fields[name]; !ok {
				return fmt.Errorf(
					"%w: unknown field name %q in SMTP notification field order of form %q",
					errConfig,
					name,
					f.ID,
				)
			}

			if _, ok := seenFields[name]; ok {
				return fmt.Errorf(
					"%w: field %q in both field order and the hidden fields of SMTP notifier of form %q",
					errConfig,
					name,
					f.ID,
				)
			}

			seenFields[name] = struct{}{}
		}

		// If the field order is given, we need to make sure that all of
		// the fields are either in the order or hidden to avoid strange
		// decisions later.
		for name := range f.Fields {
			if _, ok := seenFields[name]; !ok {
				return fmt.Errorf(
					"%w: field %q missing in the field order and the hidden fields of SMTP notifier of form %q",
					errConfig,
					name,
					f.ID,
				)
			}
		}
	}

	return nil
}
