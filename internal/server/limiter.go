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
	"errors"
	"fmt"
	"sync"
	"time"
)

var errLimiterConfig = errors.New("invalid limiter configuration")

type fixedWindowLimiter struct {
	buckets map[string]*bucket
	window  time.Duration
	limit   int
	mu      sync.Mutex
}

type bucket struct {
	resetAt time.Time
	count   int
}

func newFixedWindowLimiter(limit int, window time.Duration) (*fixedWindowLimiter, error) {
	if limit < 0 {
		return nil, fmt.Errorf("%w: negative limit", errLimiterConfig)
	}

	if window < 0 {
		return nil, fmt.Errorf("%w: negative window", errLimiterConfig)
	}

	return &fixedWindowLimiter{
		mu:      sync.Mutex{},
		limit:   limit,
		window:  window,
		buckets: map[string]*bucket{},
	}, nil
}

func (l *fixedWindowLimiter) allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		l.buckets[key] = &bucket{
			count:   1,
			resetAt: now.Add(l.window),
		}

		return true
	}

	if b.count >= l.limit {
		return false
	}

	b.count++

	return true
}
