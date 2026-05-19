GO ?= go
BIN ?= macscope
CMD := ./cmd/macscope

.PHONY: help all build run fmt test vet smoke check clean

help:
	@printf '%s\n' 'Targets:'
	@printf '  %-10s %s\n' 'build' 'Build the macscope binary.'
	@printf '  %-10s %s\n' 'run' 'Run macscope help through go run.'
	@printf '  %-10s %s\n' 'fmt' 'Format Go files.'
	@printf '  %-10s %s\n' 'test' 'Run Go tests, including command smoke tests.'
	@printf '  %-10s %s\n' 'vet' 'Run go vet.'
	@printf '  %-10s %s\n' 'smoke' 'Run command-level smoke tests.'
	@printf '  %-10s %s\n' 'check' 'Run fmt, test, and vet.'
	@printf '  %-10s %s\n' 'clean' 'Remove local build artifacts.'

all: check build

build:
	$(GO) build -o $(BIN) $(CMD)

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

clean:
	rm -f $(BIN)
