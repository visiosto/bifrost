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
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	texttemplate "text/template"

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
	{{$fields := .fields}}
	{{$payload := .payload}}
	{{$hidden := .hidden}}
	{{if (gt (len .order) 0) -}}
		{{range $key := .order -}}
			{{$field := index $fields $key -}}
			<h2>{{if (eq $field.DisplayName "")}}{{$key}}{{else}}{{$field.DisplayName}}{{end}}</h2>
			<p style="font-size: 14px; line-height: 24px; margin: 16px 0">
				{{- index $payload $key -}}
			</p>
		{{end -}}
	{{else -}}
		{{range $key, $value := $payload -}}
			{{$hide := false}}
			{{range $k := $hidden}}{{if (eq $k $key)}}{{$hide = true}}{{end}}{{end}}
			{{if $hide}}{{continue}}{{end}}
			{{$field := index $fields $key -}}
			<h2>{{if (eq $field.DisplayName "")}}{{$key}}{{else}}{{$field.DisplayName}}{{end}}</h2>
			<p style="font-size: 14px; line-height: 24px; margin: 16px 0">
				{{- $value -}}
			</p>
		{{end -}}
	{{end -}}
</body>
</html>
`

type payloadError struct {
	field   string
	message string
}

type smtpTemplate struct {
	subject *texttemplate.Template
	intro   *texttemplate.Template
	html    *template.Template
	cfg     *config.SMTPNotifier
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
	smtpTmpls, err := createSMTPTemplates(form)
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

		// TODO: Add request ID to the logs.
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

		err = sendSMTPNotifications(w, r, form, smtpTmpls, payload)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		w.WriteHeader(http.StatusOK)

		_, err = w.Write([]byte("accepted"))
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}
	}), nil
}

//nolint:cyclop,gocognit // let's keep this as one function
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
			if cfg.Type != config.FieldBool {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q has invalid type bool, expected %s", k, cfg.Type.String()),
				}
			}
		case string:
			if cfg.Type != config.FieldString {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q has invalid type string, expected %s", k, cfg.Type.String()),
				}
			}
		default:
			return &payloadError{
				field:   k,
				message: fmt.Sprintf("field %q has invalid type %T", k, v),
			}
		}
	}

	for k, v := range form.Fields { //nolint:varnamelen // basic names for loopvars
		_, ok := seenKeys[k]
		if !ok {
			if v.Required {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("missing required field %q", k),
				}
			}

			continue
		}

		val := payload[k]

		switch v.Type {
		case config.FieldBool:
			b, ok := val.(bool)
			if !ok {
				panic(fmt.Sprintf("field %q should have been a bool but it is %T", k, val))
			}

			if v.Required && !b {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q is required but its value is false", k),
				}
			}
		case config.FieldString:
			s, ok := val.(string)
			if !ok {
				panic(fmt.Sprintf("field %q should have been a string but it is %T", k, val))
			}

			if v.Required && s == "" {
				return &payloadError{
					field:   k,
					message: fmt.Sprintf("field %q is required but its value is empty", k),
				}
			}

			if (len(s) < v.Min || len(s) > v.Max) && v.Max != 0 {
				return &payloadError{
					field: k,
					message: fmt.Sprintf(
						"field %q must be between %d and %d characters but it is %d characters",
						k,
						v.Min,
						v.Max,
						len(s),
					),
				}
			}
		default:
			panic(fmt.Sprintf("invalid form field type: %d", v.Type))
		}
	}

	return nil
}

func createSMTPTemplates(form *config.Form) ([]smtpTemplate, error) {
	result := make([]smtpTemplate, len(form.SMTPNotifiers))

	for i, notifier := range form.SMTPNotifiers {
		subjTmpl, err := texttemplate.New("subject").Parse(notifier.Subject)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subject template: %w", err)
		}

		var introTmpl *texttemplate.Template

		introTmpl, err = texttemplate.New("intro").Parse(notifier.Intro)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intro template: %w", err)
		}

		var tmpl *template.Template

		tmpl, err = template.New("html").Parse(htmlTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse HTML template: %w", err)
		}

		result[i] = smtpTemplate{
			subject: subjTmpl,
			intro:   introTmpl,
			html:    tmpl,
			cfg:     notifier,
		}
	}

	return result, nil
}

func sendSMTPNotifications(
	w http.ResponseWriter,
	r *http.Request,
	form *config.Form,
	tmpls []smtpTemplate,
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

		err = tmpl.html.Execute(os.Stdout, data)
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
	}

	return nil
}
