# macscope

macscope is a macOS introspection and triage toolkit written in Go. It is being ported from the `macscope.zsh` proof of concept into a production-quality CLI that keeps collection read-only by default and favors explainable output.

Current status:

- `macscope.zsh` remains the live proof-of-concept implementation.
- `cmd/macscope` contains the Go CLI skeleton and stable command routing.
- `macscope macho` is implemented in Go.
- `macscope panic` is implemented in Go.
- `macscope proc` and `macscope attach` are implemented in Go.
- Remaining feature commands are recognized in Go, then implemented incrementally by milestone.

## Build And Run

```sh
go run ./cmd/macscope help
go run ./cmd/macscope version
go run ./cmd/macscope macho /bin/ls
go run ./cmd/macscope macho --json /bin/ls
go run ./cmd/macscope panic --file testdata/panic/watchdog.panic
go run ./cmd/macscope proc <pid-or-name>
go run ./cmd/macscope attach <pid>
go test ./...
go vet ./...
```

The same workflows are available through `make`:

```sh
make help
make build
make test
make vet
make check
```

The first live collectors will wrap native macOS tools with `exec.CommandContext`. Commands must capture stdout and stderr separately, use timeouts, and avoid shell interpolation with untrusted values.

## Command Shape

```text
macscope macho [--json] [--full] <path>
macscope proc [--json] <pid-or-name>
macscope attach [--json] [--last 30m] <pid>
macscope persist
macscope tcc --last 30m
macscope tcc --watch
macscope es --last 30m
macscope vpn [vpn-name]
macscope panic --last [--json]
macscope panic --file <panic-file> [--json]
macscope panic --since 48h [--json]
macscope timeline --pid <pid>
```

During the port, recognized-but-unimplemented Go commands point back to the zsh fallback:

```sh
./macscope.zsh tcc --last 30m
```

## macho

`macscope macho [--json] [--full] <path>` inspects a Mach-O binary or `.app` bundle. It resolves the bundle executable, hashes the binary, detects architectures, checks code signing and Gatekeeper policy, lists extended attributes, and records linked libraries.

Native tools used:

- `file`
- `lipo`
- `codesign`
- `spctl`
- `xattr`
- `otool`

Normal output is concise and evidence-driven. `--json` emits machine-readable output. `--full` includes raw command output in the JSON report for audit and debugging.

## panic

`macscope panic --last|--file <path>|--since <duration> [--json]` parses macOS panic reports and classifies watchdog/kernel reboot evidence.

Inputs:

- `--last` reads the newest `*.panic` report under `/Library/Logs/DiagnosticReports`.
- `--file <path>` reads a specific panic report.
- `--since <duration>` reads reports modified within a Go-style duration such as `30m` or `48h`.

The parser extracts panic string, CPU number, caller address, watchdog timeout duration, macOS version, boot session UUID, installer/current phase, saved report path, SOCD marker presence, pre-OS markers, and display/dock/software-update indicators.

Classifications are evidence-based. Watchdog reports can produce likely causes such as `BOOT_IOKIT_STALL` or `EXTERNAL_DISPLAY_OR_DOCK`, each with confidence and evidence strings.

## proc

`macscope proc [--json] <pid-or-name>` resolves a running process with `ps` and `pgrep`, summarizes PID/PPID/user/group/state/path/command, and checks executable signing when a path is available.

The command is read-only. It does not attach to the process.

## attach

`macscope attach [--json] [--last 30m] <pid>` explains likely LLDB attach failures using:

- process identity from `ps`
- developer group membership from `dseditgroup`
- target signing state from `codesign`
- recent attach-relevant unified logs from `log show`

The command does not bypass SIP, AMFI, TCC, hardened runtime, or taskgated policy. It reports evidence and next checks only.

## Safety Model

macscope is a defensive diagnostics tool. Default behavior should be read-only, evidence-driven, and explicit about privileged or invasive actions. It should not disable macOS protections, alter privacy databases, install persistence, hide evidence, or submit local data over the network without an explicit opt-in feature.

## Documentation

Examples live under `docs/examples/`. When adding a command, document:

- what the command does
- which native macOS tools it invokes
- what permissions may be required
- how to read the output
- limitations and expected failure modes
