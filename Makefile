.POSIX:
.SUFFIXES:

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
all: bifrost

# CODE QUALITY & CHECKS

audit: FORCE bin/golangci-lint license-check test lint
	./bin/golangci-lint config verify
	go mod tidy -diff
	go mod verify

license-check: FORCE bin/go-licenses
	go mod verify
	go mod download
	./bin/go-licenses check --include_tests $(GO_MODULE)/... --allowed_licenses="$(ALLOWED_LICENSES)"

lint: FORCE bin/addlicense bin/golangci-lint
	./bin/addlicense -check -c "$(COPYRIGHT_HOLDER)" -l "$(LICENSE)" $(ADDLICENSE_PATTERNS)
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
	go build $(GOFLAGS) -o bifrost$$(go env GOEXE) .

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
