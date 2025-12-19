# Copyright 2025 Visiosto oy
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.POSIX:
.SUFFIXES:

VERSION =
BIFROST_VERSION = 0.1.0

GOFLAGS =

ADDLICENSE_VERSION = 1.2.0
GCI_VERSION = 0.13.7
GO_LICENSES_VERSION = 2.0.1
GOFUMPT_VERSION = 0.9.2
GOLANGCI_LINT_VERSION = 2.7.2
GOLINES_VERSION = 0.13.0

ALLOWED_LICENSES = Apache-2.0,BSD-2-Clause,BSD-3-Clause,MIT
COPYRIGHT_HOLDER = Visiosto oy
LICENSE = apache
ADDLICENSE_PATTERNS = *.go internal

GO_MODULE = github.com/visiosto/bifrost

RM = rm -f

# Default target
all: build

# CODE QUALITY & CHECKS

audit: FORCE bin/golangci-lint license-check test lint
	go mod tidy -diff
	go mod verify

license-check: FORCE bin/go-licenses
	go mod verify
	go mod download
	./bin/go-licenses check --include_tests $(GO_MODULE)/... --allowed_licenses="$(ALLOWED_LICENSES)"

lint: FORCE bin/addlicense bin/golangci-lint
	./bin/addlicense -check -c "$(COPYRIGHT_HOLDER)" -l "$(LICENSE)" $(ADDLICENSE_PATTERNS)
	./bin/golangci-lint config verify
	./bin/golangci-lint run

test: FORCE
	go test $(GOFLAGS) ./...

# DEVELOPMENT & BUILDING

tidy: FORCE bin/addlicense bin/gci bin/gofumpt bin/golines
	./bin/addlicense -v -c "$(COPYRIGHT_HOLDER)" -l "$(LICENSE)" $(ADDLICENSE_PATTERNS)
	go mod tidy -v
	./bin/gci write .
	./bin/golines -m 120 -t 4 --no-chain-split-dots --no-reformat-tags -w .
	./bin/gofumpt -extra -l -w .

build: FORCE
	@version="$(VERSION)"; \
	revision=""; \
	\
	if [ -n "$${version}" ]; then \
		untrimmed="$$(git describe --always --abbrev=40 --dirty 2>/dev/null)"; \
		revision="$$(echo "$${untrimmed}" | tr -d ' \n\r')"; \
		revision="$${revision%-dirty}"; \
	else \
		if git describe --match 'v*.*.*' --tags >/dev/null 2>&1; then \
			untrimmed="$$(git describe --match 'v*.*.*' --tags 2>/dev/null)"; \
			git_describe="$$(echo "$${untrimmed}" | tr -d ' \n\r')"; \
			hyphens="$$(printf '%s' "$${git_describe}" | tr -dc '-' | wc -c)"; \
			\
			if [ "$${hyphens}" -eq 0 ]; then \
				if [ "$${git_describe}" != "v$(BIFROST_VERSION)" ]; then \
					echo "git tag does not match the version number" >&2; \
					exit 1; \
				fi; \
				\
				untrimmed_revision="$$(git rev-parse "v$(BIFROST_VERSION)^{commit}" 2>/dev/null)"; \
				revision="$$(echo "$${untrimmed_revision}" | tr -d ' \n\r')"; \
				version="$(BIFROST_VERSION)"; \
			else \
				if [ "$${hyphens}" -eq 2 ]; then \
					old_ifs="$$IFS"; \
					IFS="-"; \
					\
					set -- $${git_describe}; \
					\
					tagged_ancestor="$$1"; \
					commit_height="$$2"; \
					commit_id="$$3"; \
					IFS="$${old_ifs}"; \
					\
					if [ "$(BIFROST_VERSION)" = "$${tagged_ancestor}" ]; then \
						echo "version number in the Makefile \"$(BIFROST_VERSION)\" must be greater than tagged version \"$${tagged_ancestor}\"" >&2; \
						exit 1; \
					fi; \
					\
					if [ -z "$${commit_id}" ]; then \
						echo "unexpected \`git describe\` output: $${git_describe}" >&2; \
						exit 1; \
					fi; \
					\
					case "$${commit_id}" in \
						g*) \
							revision="$$(printf '%s' "$${commit_id#g}" | tr -d ' \n\r')"; \
							;; \
						*) \
							echo "unexpected \`git describe\` output: $${git_describe}" >&2; \
							exit 1; \
							;; \
					esac; \
					\
					version="$(BIFROST_VERSION)-dev.$${commit_height}+$${revision}"; \
				else \
					echo "unexpected \`git describe\` output: $${git_describe}" >&2; \
					exit 1; \
				fi; \
			fi; \
		else \
			untrimmed="$$(git describe --always --abbrev=40 --dirty 2>/dev/null)"; \
			revision="$$(echo "$${untrimmed}" | tr -d ' \n\r')"; \
			\
			case "$${revision}" in \
				*-dirty) \
					revision="$${revision%-dirty}"; \
					build_time="$$(date -u +%Y%m%d%H%M%S)"; \
					;; \
				*) \
					revision="$${revision%-dirty}"; \
					build_time="$$(TZ=UTC0 git show -s --date=format-local:%Y%m%d%H%M%S --format=%cd "$${revision}" 2>/dev/null)"; \
					;; \
			esac; \
			version="$(BIFROST_VERSION)-dev.$${build_time}+$${revision}"; \
		fi; \
	fi; \
	\
	if [ -z "$${version}" ]; then \
		echo "failed to create a version string"; \
		exit 1; \
	fi; \
	\
	if [ -z "$${revision}" ]; then \
		echo "failed to create parse the built revision"; \
		exit 1; \
	fi; \
	\
	ldflags="-X $(GO_MODULE)/internal/version.BuildVersion=$${version} -X $(GO_MODULE)/internal/version.Revision=$${revision}"; \
	\
	go build $(GOFLAGS) -ldflags "$${ldflags}" -o bifrost$$(go env GOEXE) .

clean: FORCE
	rm -f bifrost$$(go env GOEXE)
	rm -rf bin

# TOOL INSTALLS

bin/addlicense: bin/vendor/addlicense-$(ADDLICENSE_VERSION)
	ln -sf vendor/addlicense-$(ADDLICENSE_VERSION) $@
bin/vendor/addlicense-$(ADDLICENSE_VERSION):
	mkdir -p bin/vendor
	GOBIN="$(PWD)/bin/vendor" go install github.com/google/addlicense@v$(ADDLICENSE_VERSION)
	mv bin/vendor/addlicense $@

bin/gci: bin/vendor/gci-$(GCI_VERSION)
	ln -sf vendor/gci-$(GCI_VERSION) $@
bin/vendor/gci-$(GCI_VERSION):
	mkdir -p bin/vendor
	GOBIN="$(PWD)/bin/vendor" go install github.com/daixiang0/gci@v$(GCI_VERSION)
	mv bin/vendor/gci $@

bin/go-licenses: bin/vendor/go-licenses-$(GO_LICENSES_VERSION)
	ln -sf vendor/go-licenses-$(GO_LICENSES_VERSION) $@
bin/vendor/go-licenses-$(GO_LICENSES_VERSION):
	mkdir -p bin/vendor
	GOBIN="$(PWD)/bin/vendor" go install github.com/google/go-licenses/v2@v$(GO_LICENSES_VERSION)
	mv bin/vendor/go-licenses $@

bin/gofumpt: bin/vendor/gofumpt-$(GOFUMPT_VERSION)
	ln -sf vendor/gofumpt-$(GOFUMPT_VERSION) $@
bin/vendor/gofumpt-$(GOFUMPT_VERSION):
	mkdir -p bin/vendor
	GOBIN="$(PWD)/bin/vendor" go install mvdan.cc/gofumpt@v$(GOFUMPT_VERSION)
	mv bin/vendor/gofumpt $@

bin/golangci-lint: bin/vendor/golangci-lint-$(GOLANGCI_LINT_VERSION)
	ln -sf vendor/golangci-lint-$(GOLANGCI_LINT_VERSION) $@
bin/vendor/golangci-lint-$(GOLANGCI_LINT_VERSION):
	mkdir -p bin/vendor
	GOBIN="$(PWD)/bin/vendor" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION)
	mv bin/vendor/golangci-lint $@

bin/golines: bin/vendor/golines-$(GOLINES_VERSION)
	ln -sf vendor/golines-$(GOLINES_VERSION) $@
bin/vendor/golines-$(GOLINES_VERSION):
	mkdir -p bin/vendor
	GOBIN="$(PWD)/bin/vendor" go install github.com/segmentio/golines@v$(GOLINES_VERSION)
	mv bin/vendor/golines $@

# SPECIAL TARGET

FORCE: ;
