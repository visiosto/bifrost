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
Bifröst is a request-to-action backend for a fleet of static websites.
*/
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/visiosto/bifrost/internal/config"
	"github.com/visiosto/bifrost/internal/server"
	"github.com/visiosto/bifrost/internal/version"
)

//go:embed VERSION
var versionFile string

func init() { //nolint:gochecknoinits // initializes the version information
	version.Init(strings.Trim(versionFile, " \t\n"))
}

func main() {
	ctx := context.Background()

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
	logLevelName := flag.String("log-level", "", "log only messages with the given severity or more")

	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	if *logLevelName != "" {
		err = cfg.LogLevel.UnmarshalText([]byte(*logLevelName))
		if err != nil {
			log.Fatal(err)
		}
	}

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{ //nolint:exhaustruct // no need for value
				AddSource: false,
				Level:     cfg.LogLevel,
			}),
		),
	)

	srv := server.New(ctx, cfg)
	errCh := make(chan error, 1)

	go func() {
		slog.InfoContext(ctx, "running the server")

		errCh <- srv.Run()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.InfoContext(ctx, "signal received", "signal", sig.String())
	case err = <-errCh:
		slog.ErrorContext(ctx, "server stopped", "error", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second) //nolint:mnd
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		log.Fatal(err) //nolint:gocritic // we don't care about the cancel just now
	}

	slog.InfoContext(ctx, "shutdown complete")
}
