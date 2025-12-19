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

// Package version provides build and version information of the current binary.
package version

import "github.com/anttikivi/semver"

// Version is the parsed version object of the current build. It is created in
// [init].
var Version *semver.Version //nolint:gochecknoglobals // version must be global

// Build information populated at build time.
//
//nolint:gochecknoglobals // build information must be global
var (
	BuildVersion string
	Revision     string
)

func init() { //nolint:gochecknoinits // version must be populated when the module is first used
	Version = semver.MustParse(BuildVersion)
}
