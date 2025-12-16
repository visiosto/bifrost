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
