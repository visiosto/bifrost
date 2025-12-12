.POSIX:
.SUFFIXES:

GO = go
GOFLAGS =

VERSION =
OUTPUT =

TOOLFLAGS =

ADDLICENSE_VERSION = 1.2.0
DELVE_VERSION = 1.25.2
GCI_VERSION = 0.13.7
GO_LICENSES_VERSION = 2.0.1
GOFUMPT_VERSION = 0.9.2
GOLANGCI_LINT_VERSION = 2.7.2
GOLINES_VERSION = 0.13.0

ALLOWED_LICENSES = Apache-2.0,BSD-2-Clause,BSD-3-Clause,MIT
COPYRIGHT_HOLDER = Visiosto oy
LICENSE = apache
ADDLICENSE_PATTERNS = *.go

GO_MODULE = github.com/visiosto/bifrost

RM = rm -f

# Default target.
all: bifrost

# CODE QUALITY & CHECKS

audit: license-check test lint
	golangci-lint config verify
	"$(GO)" mod tidy -diff
	"$(GO)" mod verify

license-check: go-licenses
	"$(GO)" mod verify
	"$(GO)" mod download
	go-licenses check --include_tests $(GO_MODULE)/... --allowed_licenses="$(ALLOWED_LICENSES)"

lint: addlicense golangci-lint
	addlicense -check -c "$(COPYRIGHT_HOLDER)" -l "$(LICENSE)" $(ADDLICENSE_PATTERNS)
	golangci-lint run

test: FORCE
	"$(GO)" test $(GOFLAGS) ./...

# DEVELOPMENT & BUILDING

tidy: addlicense gci gofumpt golines
	addlicense -v -c "$(COPYRIGHT_HOLDER)" -l "$(LICENSE)" $(ADDLICENSE_PATTERNS)
	"$(GO)" mod tidy -v
	gci write .
	golines -m 120 -t 4 --no-chain-split-dots --no-reformat-tags -w .
	gofumpt -extra -l -w .

bifrost: FORCE buildtask
	@./buildtask $@

build: bifrost

clean: FORCE
	@exe=""; \
	\
	case "$$("$(GO)" env GOOS)" in \
		windows) exe=".exe";; \
	esac; \
	\
	output="$(OUTPUT)"; \
	\
	if [ -z "$${output}" ]; then \
		output="bifrost$${exe}"; \
	fi; \
	\
	$(RM) "$${output}"
	@$(RM) -r bin

# TOOL HELPERS

addlicense delve gci go-licenses gofumpt golangci-lint golines: FORCE installer
	@./installer $@ $(TOOLFLAGS)

buildtask: scripts/buildtask/script.go
	"$(GO)" build -o $@ -tags script $<

installer: scripts/installer/script.go
	"$(GO)" build -o $@ -tags script $<

# SPECIAL TARGET

FORCE: ;
