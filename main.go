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

/*
Bifröst is a tool.
*/
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/visiosto/bifrost/internal/config"
	"github.com/visiosto/bifrost/internal/version"
)

//go:embed VERSION
var versionFile string

func init() { //nolint:gochecknoinits // initializes the version information
	version.Init(strings.Trim(versionFile, " \t\n"))
}

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "version" {
			_, err := fmt.Fprintf(os.Stdout, "bifrost version %s\n", version.Version())
			if err != nil {
				log.Fatal(err)
			}

			return
		}
	}

	cfgPath := flag.String("config", "/etc/bifrost.json", "path to the config file")

	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	_ = cfg
}
