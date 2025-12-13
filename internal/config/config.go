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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Config is the program representation of the config file and the option
// overrides from the command line.
type Config struct {
	// ListenAddr is the address the program's server should bind to.
	ListenAddr string     `json:"listenAddress"`
	Sites      []Site     `json:"sites"`
	LogLevel   slog.Level `json:"logLevel"` // defaults to 0 which is info

	// MaxBody is the default maximum size of the message body in bytes.
	MaxBody int64 `json:"maxBytes"`
}

// Site is the config for a site registered to Bifröst.
type Site struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	Forms []Form `json:"forms"`
}

// Form is the config of a form in a site.
type Form struct {
	ID string `json:"id"`
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

	err = validate()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate() error {
	return nil
}
