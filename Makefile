GO ?= go
BIN ?= macscope
CMD := ./cmd/macscope
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
DATADIR ?= $(PREFIX)/share
BASH_COMPLETION_DIR ?= $(DATADIR)/bash-completion/completions
ZSH_COMPLETION_DIR ?= $(DATADIR)/zsh/site-functions
FISH_COMPLETION_DIR ?= $(DATADIR)/fish/vendor_completions.d
MAN1DIR ?= $(DATADIR)/man/man1
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || printf dev)
BUILD_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
DIST_DIR ?= dist
DIST_NAME := $(BIN)_$(VERSION)_$(GOOS)_$(GOARCH)
LDFLAGS ?= -s -w -X github.com/jdefrancesco/macscope/internal/cli.version=$(VERSION) -X github.com/jdefrancesco/macscope/internal/cli.buildCommit=$(BUILD_COMMIT) -X github.com/jdefrancesco/macscope/internal/cli.buildDate=$(BUILD_DATE)

.PHONY: help all build run fmt test vet smoke check install uninstall install-completions uninstall-completions install-man uninstall-man dist release clean

help:
	@printf '%s\n' 'Targets:'
	@printf '  %-22s %s\n' 'build' 'Build the macscope binary.'
	@printf '  %-22s %s\n' 'run' 'Run macscope help through go run.'
	@printf '  %-22s %s\n' 'fmt' 'Format Go files.'
	@printf '  %-22s %s\n' 'test' 'Run Go tests, including command smoke tests.'
	@printf '  %-22s %s\n' 'vet' 'Run go vet.'
	@printf '  %-22s %s\n' 'smoke' 'Run command-level smoke tests.'
	@printf '  %-22s %s\n' 'check' 'Run fmt, test, and vet.'
	@printf '  %-22s %s\n' 'install' 'Install the binary under PREFIX.'
	@printf '  %-22s %s\n' 'uninstall' 'Remove the installed binary under PREFIX.'
	@printf '  %-22s %s\n' 'install-completions' 'Install bash, zsh, and fish completions.'
	@printf '  %-22s %s\n' 'uninstall-completions' 'Remove installed completions.'
	@printf '  %-22s %s\n' 'install-man' 'Install the macscope(1) manual page.'
	@printf '  %-22s %s\n' 'uninstall-man' 'Remove the installed manual page.'
	@printf '  %-22s %s\n' 'dist' 'Build a local release archive under dist/.'
	@printf '  %-22s %s\n' 'release' 'Run checks and build release artifacts.'
	@printf '  %-22s %s\n' 'clean' 'Remove local build artifacts.'

all: check build

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)

run:
	$(GO) run $(CMD) help

fmt:
	gofmt -w .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

smoke:
	$(GO) test ./cmd/macscope -run Smoke -count=1

check: fmt test vet

install: build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 0755 "$(BIN)" "$(DESTDIR)$(BINDIR)/$(BIN)"

uninstall:
	rm -f "$(DESTDIR)$(BINDIR)/$(BIN)"

install-completions:
	install -d "$(DESTDIR)$(BASH_COMPLETION_DIR)"
	install -d "$(DESTDIR)$(ZSH_COMPLETION_DIR)"
	install -d "$(DESTDIR)$(FISH_COMPLETION_DIR)"
	$(GO) run $(CMD) completion bash > "$(DESTDIR)$(BASH_COMPLETION_DIR)/$(BIN)"
	$(GO) run $(CMD) completion zsh > "$(DESTDIR)$(ZSH_COMPLETION_DIR)/_$(BIN)"
	$(GO) run $(CMD) completion fish > "$(DESTDIR)$(FISH_COMPLETION_DIR)/$(BIN).fish"

uninstall-completions:
	rm -f "$(DESTDIR)$(BASH_COMPLETION_DIR)/$(BIN)"
	rm -f "$(DESTDIR)$(ZSH_COMPLETION_DIR)/_$(BIN)"
	rm -f "$(DESTDIR)$(FISH_COMPLETION_DIR)/$(BIN).fish"

install-man:
	install -d "$(DESTDIR)$(MAN1DIR)"
	install -m 0644 docs/man/$(BIN).1 "$(DESTDIR)$(MAN1DIR)/$(BIN).1"

uninstall-man:
	rm -f "$(DESTDIR)$(MAN1DIR)/$(BIN).1"

dist:
	rm -rf "$(DIST_DIR)/$(DIST_NAME)"
	mkdir -p "$(DIST_DIR)/$(DIST_NAME)/completions"
	mkdir -p "$(DIST_DIR)/$(DIST_NAME)/man/man1"
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -ldflags "$(LDFLAGS)" -o "$(DIST_DIR)/$(DIST_NAME)/$(BIN)" $(CMD)
	cp README.md "$(DIST_DIR)/$(DIST_NAME)/README.md"
	cp -R docs "$(DIST_DIR)/$(DIST_NAME)/docs"
	cp docs/man/$(BIN).1 "$(DIST_DIR)/$(DIST_NAME)/man/man1/$(BIN).1"
	$(GO) run $(CMD) completion bash > "$(DIST_DIR)/$(DIST_NAME)/completions/$(BIN).bash"
	$(GO) run $(CMD) completion zsh > "$(DIST_DIR)/$(DIST_NAME)/completions/_$(BIN)"
	$(GO) run $(CMD) completion fish > "$(DIST_DIR)/$(DIST_NAME)/completions/$(BIN).fish"
	tar -C "$(DIST_DIR)" -czf "$(DIST_DIR)/$(DIST_NAME).tar.gz" "$(DIST_NAME)"
	shasum -a 256 "$(DIST_DIR)/$(DIST_NAME).tar.gz" > "$(DIST_DIR)/$(DIST_NAME).tar.gz.sha256"

release: check dist

clean:
	rm -f $(BIN)
