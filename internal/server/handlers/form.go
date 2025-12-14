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
	"net/http"

	"github.com/visiosto/bifrost/internal/config"
)

// FormPreflight is the handler for the `OPTIONS` method of form endpoints.
func FormPreflight() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "600")
		// Note: Both 200 OK and 204 No Content are permitted status codes, but
		// some browsers incorrectly believe 204 No Content applies to
		// the resource and do not send a subsequent request to fetch it.
		w.WriteHeader(http.StatusOK)
	})
}

// SubmitForm returns a [http.Handler] for a form endpoint.
func SubmitForm(_ *config.Site, _ *config.Form) http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
	})
}
