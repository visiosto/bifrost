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

// Package config defines the Bifröst configuration at the related utilities.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var errConfig = errors.New("invalid config")

// Config is the program representation of the config file and the option
// overrides from the command line.
type Config struct {
	// ListenAddr is the address the program's server should bind to.
	ListenAddr string     `json:"listenAddress"`
	Sites      []Site     `json:"sites"`
	LogLevel   slog.Level `json:"logLevel"` // defaults to 0 which is info
	RateLimit  RateLimit  `json:"rateLimit"`

	// MaxBodyBytes is the default maximum size of the message body in bytes.
	MaxBodyBytes int64 `json:"maxBodyBytes"`

	// DebugHeaders controls whether to print the request headers to the log.
	// This should not be turned on except when truly debugging.
	DebugHeaders bool `json:"debugHeaders"`
}

// RateLimit is the global rate limit config.
type RateLimit struct {
	PerIPSiteMinute int `json:"perIpSiteMinute"`
}

// Site is the config for a site registered to Bifröst.
type Site struct {
	ID             string   `json:"id"`
	Token          string   `json:"token"`
	AllowedOrigins []string `json:"allowedOrigins"`
	Forms          []Form   `json:"forms"`
}

// Form is the config of a form in a site.
type Form struct {
	ID                  string               `json:"id"`
	Token               string               `json:"token"`
	Fields              map[string]FormField `json:"fields"`
	SMTPNotifiers       []*SMTPNotifier      `json:"smtp"`
	ContentType         ContentType          `json:"contentType"`
	AccessControlMaxAge int                  `json:"accessControlMaxAge"`
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
	Port           int    `json:"port"`
}

// Load loads the config from the config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file at %q: %w", path, err)
	}

	var cfg Config

	dec := json.NewDecoder(bytes.NewReader(data))

	dec.DisallowUnknownFields()

	err = dec.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.ListenAddr == "" {
		return fmt.Errorf("%w: empty listenAddress", errConfig)
	}

	if c.MaxBodyBytes <= 0 {
		return fmt.Errorf("%w: maxBytes must be greater than zero", errConfig)
	}

	if c.RateLimit.PerIPSiteMinute <= 0 {
		return fmt.Errorf("%w: global rate limit perIpSiteMinute must be greater than zero", errConfig)
	}

	seenIDs := map[string]struct{}{}

	for _, site := range c.Sites {
		if site.ID == "" {
			return fmt.Errorf("%w: empty site ID", errConfig)
		}

		if site.ID == "_" {
			return fmt.Errorf("%w: use of reserved site ID %q", errConfig, "_")
		}

		if _, ok := seenIDs[site.ID]; ok {
			return fmt.Errorf("%w: duplicate site ID %q", errConfig, site.ID)
		}

		seenIDs[site.ID] = struct{}{}

		if site.Token == "" {
			return fmt.Errorf("%w: empty site token", errConfig)
		}

		if len(site.AllowedOrigins) == 0 {
			return fmt.Errorf("%w: no allowed origins for site %q", errConfig, site.ID)
		}

		for _, form := range site.Forms {
			err := form.validate()
			if err != nil {
				return err
			}
		}
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
	}

	return nil
}
