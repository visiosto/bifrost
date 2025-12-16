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

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	texttemplate "text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/visiosto/bifrost/internal/config"
)

//
//nolint:lll
const htmlTemplate = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html lang="{{.lang}}">
<head>
<meta charset="utf-8" />
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
<title>{{- .subject -}}</title>
</head>
<body>
	<h1>{{.subject}}</h1>
	{{if (ne .intro "") -}}
		<p style="font-size: 14px; line-height: 24px; margin: 16px 0">
			{{- .intro -}}
		</p>
	{{end -}}
	{{$fields := .fields -}}
	{{$objs := .objs -}}
	{{$payload := .payload -}}
	{{$hidden := .hidden -}}
	{{if (gt (len .order) 0) -}}
		{{range $key := .order -}}
			{{$field := index $fields $key -}}
			<h2>{{if (eq $field.DisplayName "")}}{{$key}}{{else}}{{$field.DisplayName}}{{end}}</h2>
			{{if (IsObj $key)}}
				{{$lines := index $objs $key -}}
				<ul style="font-size: 14px; line-height: 24px; margin: 16px 0">
					{{range $line := $lines}}
						<li>{{- $line -}}</li>
					{{end}}
				</ul>
			{{ else -}}
				<p style="font-size: 14px; line-height: 24px; margin: 16px 0">
					{{- index $payload $key -}}
				</p>
			{{end -}}
		{{end -}}
	{{else -}}
		{{range $key, $value := $payload -}}
			{{$hide := false}}
			{{range $k := $hidden}}{{if (eq $k $key)}}{{$hide = true}}{{end}}{{end}}
			{{if $hide}}{{continue}}{{end}}
			{{$field := index $fields $key -}}
			<h2>{{if (eq $field.DisplayName "")}}{{$key}}{{else}}{{$field.DisplayName}}{{end}}</h2>
			{{if (IsObj $key)}}
				{{$lines := index $objs $key -}}
				<ul style="font-size: 14px; line-height: 24px; margin: 16px 0">
					{{range $line := $lines}}
						<li>{{- $line -}}</li>
					{{end}}
				</ul>
			{{ else -}}
				<p style="font-size: 14px; line-height: 24px; margin: 16px 0">
					{{- $value -}}
				</p>
			{{end -}}
		{{end -}}
	{{end -}}
</body>
</html>
`

const textTemplate = `{{- if (ne .intro "") -}}{{.intro}}{{- end}}
{{$fields := .fields -}}
{{$objs := .objs -}}
{{$payload := .payload -}}
{{$hidden := .hidden -}}
{{if (gt (len .order) 0) -}}
{{range $key := .order -}}
{{$field := index $fields $key -}}
{{if (IsObj $key)}}
{{$lines := index $objs $key -}}
{{if (eq $field.DisplayName "") -}}{{$key}}{{else -}}{{$field.DisplayName}}{{end}}:
{{range $line := $lines}}
  - {{$line -}}
{{end}}
{{ else -}}
{{if (eq $field.DisplayName "") -}}{{$key}}{{else -}}{{$field.DisplayName}}{{end}}: {{index $payload $key}}
{{end -}}
{{end -}}
{{else -}}
{{range $key, $value := $payload -}}
{{$hide := false -}}
{{range $k := $hidden -}}{{if (eq $k $key) -}}{{$hide = true -}}{{end -}}{{end -}}
{{if $hide -}}{{continue -}}{{end -}}
{{$field := index $fields $key -}}
{{if (IsObj $key)}}
{{$lines := index $objs $key -}}
{{if (eq $field.DisplayName "") -}}{{$key}}{{else -}}{{$field.DisplayName}}{{end}}:
{{range $line := $lines}}
  - {{$line -}}
{{end}}
{{ else -}}
{{if (eq $field.DisplayName "") -}}{{$key}}{{else -}}{{$field.DisplayName}}{{end}}: {{$value}}
{{end -}}
{{end -}}
{{end}}
`

type payloadError struct {
	field   string
	message string
}

type sesTemplate struct {
	subject *texttemplate.Template
	intro   *texttemplate.Template
	html    *template.Template
	text    *texttemplate.Template
	cfg     *config.SESNotifier
	objs    map[string]*texttemplate.Template
}

func (e *payloadError) Error() string {
	return e.message
}

// FormPreflight is the handler for the `OPTIONS` method of form endpoints.
func FormPreflight(form *config.Form) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

		allowHeaders := []string{"Content-Type", config.SiteTokenHeader}
		if form.Token != "" {
			allowHeaders = append(allowHeaders, config.FormTokenHeader)
		}

		w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowHeaders, ", "))
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(form.AccessControlMaxAge))

		// NOTE: Both 200 OK and 204 No Content are permitted status codes, but
		// some browsers incorrectly believe 204 No Content applies to
		// the resource and do not send a subsequent request to fetch it.
		w.WriteHeader(http.StatusOK)
	})
}

// SubmitForm returns a [http.Handler] for a form endpoint.
func SubmitForm(site *config.Site, form *config.Form) (http.Handler, error) {
	sesTmpls, err := createSMTPTemplates(form)
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := map[string]any{}
		dec := json.NewDecoder(r.Body)

		err := dec.Decode(&payload)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)

			return
		}

		if dec.More() {
			slog.WarnContext(
				r.Context(),
				"reject request with more than one JSON object",
				"path",
				r.URL.Path,
				"site",
				site.ID,
				"form",
				form.ID,
			)
			http.Error(w, "Bad Request", http.StatusBadRequest)

			return
		}

		// TODO: Remove this.
		slog.DebugContext(
			r.Context(),
			"received form payload",
			"path",
			r.URL.Path,
			"site",
			site.ID,
			"form",
			form.ID,
			"payload",
			payload,
		)

		err = validatePayload(form, payload)
		if err != nil {
			var payloadErr *payloadError
			if errors.As(err, &payloadErr) {
				slog.WarnContext(
					r.Context(),
					"invalid request payload",
					"path",
					r.URL.Path,
					"site",
					site.ID,
					"form",
					form.ID,
					"field",
					payloadErr.field,
					"err",
					err.Error(),
				)
			} else {
				slog.WarnContext(
					r.Context(),
					"invalid request payload",
					"path",
					r.URL.Path,
					"site",
					site.ID,
					"form",
					form.ID,
					"err",
					err.Error(),
				)
			}

			http.Error(w, "Bad Request", http.StatusBadRequest)

			return
		}

		err = handleSESNotifiers(w, r, form, sesTmpls, payload)
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"failed to send SES notifications",
				"path",
				r.URL.Path,
				"site",
				site.ID,
				"form",
				form.ID,
				"err",
				err.Error(),
			)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		w.WriteHeader(http.StatusOK)

		_, err = w.Write([]byte("accepted"))
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"failed write response",
				"path",
				r.URL.Path,
				"site",
				site.ID,
				"form",
				form.ID,
				"err",
				err.Error(),
			)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}
	}), nil
}

//nolint:cyclop,funlen,gocognit,gocyclo,maintidx // let's keep this as one function
func validatePayload(form *config.Form, payload map[string]any) error {
	seenKeys := map[string]struct{}{}

	for k, v := range payload {
		seenKeys[k] = struct{}{}

		cfg, ok := form.Fields[k]
		if !ok {
			return &payloadError{field: k, message: fmt.Sprintf("unknown field %q", k)}
		}

		switch v.(type) {
		case bool:
			if cfg.Type != config.FormFieldBool {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q has invalid type bool, expected %s", k, cfg.Type.String()),
				}
			}
		case float64:
			if cfg.Type != config.FormFieldInt {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q has invalid type number, expected %s", k, cfg.Type.String()),
				}
			}
		case string:
			if cfg.Type != config.FormFieldString {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q has invalid type string, expected %s", k, cfg.Type.String()),
				}
			}
		case []any:
			if cfg.Type != config.FormFieldObjects {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q has invalid type array, expected %s", k, cfg.Type.String()),
				}
			}
		default:
			return &payloadError{
				field:   k,
				message: fmt.Sprintf("field %q has invalid type %T", k, v),
			}
		}
	}

	for k, field := range form.Fields { //nolint:varnamelen // basic names for loopvars
		_, ok := seenKeys[k]
		if !ok {
			if field.Required {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("missing required field %q", k),
				}
			}

			continue
		}

		val := payload[k]

		switch field.Type {
		case config.FormFieldBool:
			b, ok := val.(bool)
			if !ok {
				panic(fmt.Sprintf("field %q should have been a bool but it is %T", k, val))
			}

			if field.Required && !b {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q is required but its value is false", k),
				}
			}
		case config.FormFieldInt:
			f, ok := val.(float64)
			if !ok {
				panic(fmt.Sprintf("field %q should have been a number but it is %T", k, val))
			}

			i := int(f)

			if i < field.Min || i > field.Max {
				return &payloadError{
					field: k,
					message: fmt.Sprintf(
						"field %q must be between %d and %d but it is %d",
						k,
						field.Min,
						field.Max,
						i,
					),
				}
			}

			payload[k] = i
		case config.FormFieldString:
			s, ok := val.(string)
			if !ok {
				panic(fmt.Sprintf("field %q should have been a string but it is %T", k, val))
			}

			if field.Required && s == "" {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q is required but its value is empty", k),
				}
			}

			if (len(s) < field.Min || len(s) > field.Max) && field.Max != 0 {
				return &payloadError{
					field: k,
					message: fmt.Sprintf(
						"field %q must be between %d and %d characters but it is %d characters",
						k,
						field.Min,
						field.Max,
						len(s),
					),
				}
			}
		case config.FormFieldObjects:
			arr, ok := val.([]any)
			if !ok && field.Required {
				panic(fmt.Sprintf("field %q should have been an array but it is %T", k, val))
			}

			if field.Required && len(arr) == 0 {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q is required but its value is empty", k),
				}
			}

			for _, a := range arr {
				obj, ok := a.(map[string]any)
				if !ok {
					return &payloadError{
						field:   k,
						message: fmt.Sprintf("could not cast element of field %q to a map", k),
					}
				}

				seenInObj := map[string]struct{}{}

				for name, value := range obj {
					var shapeType config.FormFieldType

					shapeType, ok = field.Shape[name]
					if !ok {
						return &payloadError{field: k, message: fmt.Sprintf("unknown field %q in field %q", name, k)}
					}

					seenInObj[name] = struct{}{}

					switch shapeType {
					case config.FormFieldBool:
						if _, ok = value.(bool); !ok {
							return &payloadError{
								field: k,
								message: fmt.Sprintf(
									"value %q in field %q should be bool but it is %T",
									name,
									k,
									value,
								),
							}
						}
					case config.FormFieldInt:
						if _, ok = value.(float64); !ok {
							return &payloadError{
								field: k,
								message: fmt.Sprintf(
									"value %q in field %q should be number but it is %T",
									name,
									k,
									value,
								),
							}
						}
					case config.FormFieldString:
						if _, ok = value.(string); !ok {
							return &payloadError{
								field: k,
								message: fmt.Sprintf(
									"value %q in field %q should be string but it is %T",
									name,
									k,
									value,
								),
							}
						}
					case config.FormFieldObjects:
						fallthrough //nolint:gocritic // Just throw the error.
					default:
						panic(fmt.Sprintf("value %q in field %q has invalid configured type", name, k))
					}
				}

				for name := range field.Shape {
					if _, ok = seenInObj[name]; !ok {
						return &payloadError{
							field: k,
							message: fmt.Sprintf(
								"value %q in field %q missing",
								name,
								k,
							),
						}
					}
				}
			}
		default:
			panic(fmt.Sprintf("invalid form field type: %d", field.Type))
		}
	}

	return nil
}

func createSMTPTemplates(form *config.Form) ([]sesTemplate, error) {
	result := make([]sesTemplate, len(form.SESNotifiers))

	for i, notifier := range form.SESNotifiers {
		subjTmpl, err := texttemplate.New("subject").Parse(notifier.Subject)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subject template: %w", err)
		}

		var introTmpl *texttemplate.Template

		introTmpl, err = texttemplate.New("intro").Parse(notifier.Intro)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intro template: %w", err)
		}

		objs := map[string]*texttemplate.Template{}

		for name, field := range form.Fields {
			if field.Type != config.FormFieldObjects {
				continue
			}

			var obj *texttemplate.Template

			obj, err = texttemplate.New(name).Parse(field.DisplayTemplate)
			if err != nil {
				return nil, fmt.Errorf("failed to parse text template for field %q: %w", name, err)
			}

			objs[name] = obj
		}

		var html *template.Template

		html, err = template.New("html").Funcs(template.FuncMap{
			"IsObj": func(name string) bool {
				_, ok := objs[name]

				return ok
			},
		}).Parse(htmlTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse HTML template: %w", err)
		}

		var text *texttemplate.Template

		text, err = texttemplate.New("text").Funcs(texttemplate.FuncMap{
			"IsObj": func(name string) bool {
				_, ok := objs[name]

				return ok
			},
		}).Parse(textTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse text template: %w", err)
		}

		result[i] = sesTemplate{
			subject: subjTmpl,
			intro:   introTmpl,
			html:    html,
			text:    text,
			cfg:     notifier,
			objs:    objs,
		}
	}

	return result, nil
}

func handleSESNotifiers(
	w http.ResponseWriter,
	r *http.Request,
	form *config.Form,
	tmpls []sesTemplate,
	payload map[string]any,
) error {
	for _, tmpl := range tmpls {
		data := map[string]any{}
		data["payload"] = payload
		data["fields"] = form.Fields
		data["lang"] = tmpl.cfg.Lang
		data["order"] = tmpl.cfg.FieldOrder
		data["hidden"] = tmpl.cfg.HiddenFields

		var subjBuf bytes.Buffer

		err := tmpl.subject.Execute(&subjBuf, data)
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"failed to execute subject template",
				"path",
				r.URL.Path,
				"tmpl",
				tmpl.cfg.Subject,
				"err",
				err,
			)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return fmt.Errorf("failed to execute subject template: %w", err)
		}

		data["subject"] = subjBuf.String()

		var introBuf bytes.Buffer

		err = tmpl.intro.Execute(&introBuf, data)
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"failed to execute intro template",
				"path",
				r.URL.Path,
				"tmpl",
				tmpl.cfg.Intro,
				"err",
				err,
			)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return fmt.Errorf("failed to execute intro template: %w", err)
		}

		data["intro"] = introBuf.String()

		objs := map[string][]string{}

		for name, obj := range tmpl.objs {
			objs[name] = make([]string, 0)

			val, ok := payload[name].([]any)
			if !ok && form.Fields[name].Required {
				panic(fmt.Sprintf("field %q has a value that is not an array but %T", name, payload[name]))
			}

			for _, v := range val {
				var buf bytes.Buffer

				err = obj.Execute(&buf, v)
				if err != nil {
					slog.ErrorContext(
						r.Context(),
						"failed to execute object template",
						"path",
						r.URL.Path,
						"field",
						name,
						"err",
						err,
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)

					return fmt.Errorf("failed to execute template for field %q: %w", name, err)
				}

				objs[name] = append(objs[name], buf.String())
			}
		}

		data["objs"] = objs

		var htmlBuf bytes.Buffer

		err = tmpl.html.Execute(&htmlBuf, data)
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"failed to execute HTML template",
				"path",
				r.URL.Path,
				"err",
				err,
			)

			return fmt.Errorf("failed to execute HTML template: %w", err)
		}

		var textBuf bytes.Buffer

		err = tmpl.text.Execute(&textBuf, data)
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"failed to execute text template",
				"path",
				r.URL.Path,
				"err",
				err,
			)

			return fmt.Errorf("failed to execute text template: %w", err)
		}

		err = sendSES(r.Context(), tmpl.cfg, subjBuf.String(), htmlBuf.String(), textBuf.String())
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"failed to send email",
				"path",
				r.URL.Path,
				"err",
				err,
			)

			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	return nil
}

func sendSES(
	ctx context.Context,
	notifier *config.SESNotifier,
	subject string,
	htmlBody string,
	textBody string,
) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(notifier.Region))
	if err != nil {
		return fmt.Errorf("failed to create AWS config: %w", err)
	}

	client := ses.NewFromConfig(cfg)
	input := &ses.SendEmailInput{ //nolint:exhaustruct // use defaults
		Destination: &types.Destination{ //nolint:exhaustruct // use defaults
			ToAddresses: []string{notifier.To},
		},
		Message: &types.Message{
			Body: &types.Body{
				Html: &types.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(htmlBody),
				},
				Text: &types.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(textBody),
				},
			},
			Subject: &types.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(subject),
			},
		},
		Source: aws.String(notifier.From),
	}

	_, err = client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
