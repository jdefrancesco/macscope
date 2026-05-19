# macscope

macscope is a macOS introspection and triage toolkit written in Go. It is being ported from the `macscope.zsh` proof of concept into a production-quality CLI that keeps collection read-only by default and favors explainable output.

Current status:

- `macscope.zsh` remains the live proof-of-concept implementation.
- `cmd/macscope` contains the Go CLI skeleton and stable command routing.
- Feature commands are recognized in Go, then implemented incrementally by milestone.

## Build And Run

```sh
go run ./cmd/macscope help
go run ./cmd/macscope version
go test ./...
go vet ./...
```

The first live collectors will wrap native macOS tools with `exec.CommandContext`. Commands must capture stdout and stderr separately, use timeouts, and avoid shell interpolation with untrusted values.

## Command Shape

```text
macscope macho <path>
macscope proc <pid-or-name>
macscope attach <pid>
macscope persist
macscope tcc --last 30m
macscope tcc --watch
macscope es --last 30m
macscope vpn [vpn-name]
macscope panic --last
macscope panic --file <panic-file>
macscope panic --since 48h
macscope panic --json
macscope timeline --pid <pid>
```

During the port, recognized-but-unimplemented Go commands point back to the zsh fallback:

```sh
./macscope.zsh macho /bin/ls
./macscope.zsh tcc --last 30m
```

## Safety Model

macscope is a defensive diagnostics tool. Default behavior should be read-only, evidence-driven, and explicit about privileged or invasive actions. It should not disable macOS protections, alter privacy databases, install persistence, hide evidence, or submit local data over the network without an explicit opt-in feature.

## Documentation

Examples live under `docs/examples/`. When adding a command, document:

- what the command does
- which native macOS tools it invokes
- what permissions may be required
- how to read the output
- limitations and expected failure modes
