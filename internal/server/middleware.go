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

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/visiosto/bifrost/internal/config"
)

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeySite
)

const siteTokenHeader = "X-Bifrost-Token" // #nosec G101 -- This is a false positive

type ctxKey int

type responseWriter struct {
	http.ResponseWriter

	status int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func withMiddleware(h http.Handler, cfg *config.Config, l *fixedWindowLimiter, paths map[string]pathInfo) http.Handler {
	h = rateLimit(h, l)
	h = verifyToken(h, paths)
	h = corsByPath(h, paths)
	h = pathContext(h, paths)

	if cfg.DebugHeaders {
		h = debugHeaders(h)
	}

	h = accessLogger(h)
	h = requestID(h)
	h = http.MaxBytesHandler(h, cfg.MaxBodyBytes)
	h = recoverer(h)

	return h
}

func recoverer(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		h.ServeHTTP(w, r)
	})
}

func requestID(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b [16]byte

		_, err := rand.Read(b[:])
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to assign a request ID")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		id := hex.EncodeToString(b[:])
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func accessLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 0}

		h.ServeHTTP(rw, r)

		reqID, ok := r.Context().Value(ctxKeyRequestID).(string)
		if !ok {
			slog.ErrorContext(r.Context(), "request_id is not a string")

			reqID = "unknown"
		}

		slog.InfoContext(
			r.Context(),
			"HTTP request",
			"method",
			r.Method,
			"path",
			r.URL.Path,
			"status",
			rw.status,
			"duration_ms",
			time.Since(start).Milliseconds(),
			"remote_ip",
			remoteIP(r),
			"request_id",
			reqID,
		)
	})
}

func debugHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, ok := r.Context().Value(ctxKeyRequestID).(string)
		if !ok {
			slog.ErrorContext(r.Context(), "request_id is not a string")

			reqID = "unknown"
		}

		slog.DebugContext(
			r.Context(),
			"request headers",
			"method",
			r.Method,
			"path",
			r.URL.Path,
			"request_id",
			reqID,
			"header",
			r.Header,
		)
		h.ServeHTTP(w, r)
	})
}

func pathContext(h http.Handler, paths map[string]pathInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := paths[r.URL.Path]
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)

			return
		}

		if info.site == "" {
			slog.WarnContext(r.Context(), "failed to assign site to context", "path", r.URL.Path)
			h.ServeHTTP(w, r)

			return
		}

		slog.DebugContext(r.Context(), "assigning site", "site", info.site, "path", r.URL.Path)

		ctx := context.WithValue(r.Context(), ctxKeySite, info.site)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func corsByPath(h http.Handler, paths map[string]pathInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// TODO: Come back to this and clean up the if statement after we have
		// had some real-world requests. We should see out what kind of shape to
		// expect from the legitimate paths we get.
		if path == "" || path == "/" || !strings.HasPrefix(path, "/") {
			http.Error(w, "Not Found", http.StatusNotFound)

			return
		}

		info, ok := paths[path]
		if !ok {
			http.Error(w, "Forbidden", http.StatusForbidden)

			return
		}

		wildcard := slices.Contains(info.allowedOrigins, "*")

		origin := r.Header.Get("Origin")
		if origin == "" && !wildcard {
			http.Error(w, "Forbidden", http.StatusForbidden)

			return
		}

		if !wildcard && !slices.Contains(info.allowedOrigins, origin) {
			http.Error(w, "Forbidden", http.StatusForbidden)

			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")

		h.ServeHTTP(w, r)
	})
}

func verifyToken(h http.Handler, paths map[string]pathInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			h.ServeHTTP(w, r)

			return
		}

		path := r.URL.Path
		// TODO: Come back to this and clean up the if statement after we have
		// had some real-world requests. We should see out what kind of shape to
		// expect from the legitimate paths we get.
		if path == "" || path == "/" || !strings.HasPrefix(path, "/") {
			http.Error(w, "Not Found", http.StatusNotFound)

			return
		}

		info, ok := paths[path]
		if !ok {
			http.Error(w, "Forbidden", http.StatusForbidden)

			return
		}

		if info.token == "" {
			h.ServeHTTP(w, r)

			return
		}

		token := r.Header.Get(siteTokenHeader)
		if token == "" {
			w.Header().Set("WWW-Authenticate", siteTokenHeader)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		if token != info.token {
			w.Header().Set("WWW-Authenticate", siteTokenHeader)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		h.ServeHTTP(w, r)
	})
}

func rateLimit(h http.Handler, l *fixedWindowLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site, ok := r.Context().Value(ctxKeySite).(string)
		if !ok {
			slog.ErrorContext(r.Context(), "failed to get site from context")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		key := site + "|" + remoteIP(r)
		if !l.allow(key) {
			slog.WarnContext(r.Context(), "rate limit exceeded", "key", key)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)

			return
		}

		h.ServeHTTP(w, r)
	})
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
