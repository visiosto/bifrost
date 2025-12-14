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

// Package server contains the HTTP server of Bifröst.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/visiosto/bifrost/internal/config"
	"github.com/visiosto/bifrost/internal/server/handlers"
)

const apiPrefix = "/v1"

// Server contains the HTTP server and the configured modules.
type Server struct {
	HTTPServer *http.Server
}

type pathInfo struct {
	site           string
	allowedOrigins []string
}

// New allocates and returns a new Server.
func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	limiter, err := newFixedWindowLimiter(ctx, cfg.RateLimit.PerIPSiteMinute, time.Minute)
	if err != nil {
		return nil, err
	}

	// Map the allowed origins and sites to the created paths.
	paths := make(map[string]pathInfo)
	mux := http.NewServeMux()

	paths["/health"] = pathInfo{site: "_", allowedOrigins: []string{"*"}}

	mux.Handle("/health", handlers.Health())

	for _, site := range cfg.Sites {
		slog.DebugContext(ctx, "registering handlers for site", "site", site.ID)

		for _, form := range site.Forms {
			path := apiPrefix + "/forms/" + site.ID + "/" + form.ID

			slog.DebugContext(ctx, "registering handler for form", "site", site.ID, "form", form.ID, "path", path)

			paths[path] = pathInfo{site: site.ID, allowedOrigins: site.AllowedOrigins}

			mux.Handle("POST "+path, handlers.SubmitForm(&site, &form))
			mux.Handle("OPTIONS "+path, handlers.FormPreflight())
		}
	}

	handler := withMiddleware(mux, cfg, limiter, paths)
	httpServer := &http.Server{ //nolint:exhaustruct // use defaults
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadTimeout:       5 * time.Second,  //nolint:mnd
		WriteTimeout:      10 * time.Second, //nolint:mnd
		IdleTimeout:       10 * time.Second, //nolint:mnd
		ReadHeaderTimeout: 2 * time.Second,  //nolint:mnd
	}

	return &Server{
		HTTPServer: httpServer,
	}, nil
}

// Run runs the server.
func (s *Server) Run() error {
	err := s.HTTPServer.ListenAndServe()
	if err != nil {
		return fmt.Errorf("unexpected error in server: %w", err)
	}

	return nil
}

// Shutdown tries to shut down the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.HTTPServer.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("failed to shut down the server: %w", err)
	}

	return nil
}
