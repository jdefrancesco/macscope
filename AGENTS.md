AGENTS.md

Project: macscope

macscope is a modern macOS introspection and triage toolkit written in Go. It is intended to replace the practical parts of old DTrace-style workflows with supported macOS facilities and safe wrappers around native tools.

Primary focus areas:

* Apple Silicon reverse-engineering support
* macOS process, binary, signing, and policy triage
* LLDB attach failure investigation
* TCC, AMFI, syspolicyd, taskgated, and EndpointSecurity denial analysis
* VPN and network-drop debugging
* launchd persistence hunting
* panic/watchdog/kernel reboot triage
* future EndpointSecurity-backed event collection

This repository is moving from a zsh proof of concept to a production-quality Go CLI.

⸻

Development Priorities

When working in this repo, prioritize in this order:

1. Correctness and safety
2. Clear, explainable output
3. Small composable packages
4. Testable parsing logic
5. macOS-native behavior
6. Minimal dependencies
7. Fast command startup
8. Clean JSON output for automation

Do not optimize prematurely. Prefer readable, boring Go over clever abstractions.

⸻

Security and Safety Rules

macscope is a defensive diagnostics and introspection tool.

Do not add code that:

* bypasses macOS security protections
* disables SIP, AMFI, TCC, Gatekeeper, or quarantine
* hides processes, files, logs, persistence, or network activity
* tampers with panic logs or diagnostic evidence
* modifies user privacy databases such as TCC.db
* installs persistence automatically
* exfiltrates logs, hardware identifiers, panic files, or user data
* runs destructive cleanup without an explicit user flag

Safe behavior:

* read-only collection by default
* clear warnings before privileged commands
* no network submission unless explicitly implemented behind an opt-in flag
* redact sensitive identifiers in normal output where reasonable
* keep raw evidence available with --full or --json

⸻

Repository Layout

Use this target layout unless the repository already differs:

cmd/
  macscope/
    main.go
internal/
  cli/
  collect/
  output/
  process/
  codesign/
  gatekeeper/
  macho/
  launchd/
  tcc/
  endpointsecurity/
  vpn/
  panic/
  logquery/
  pmset/
  systemextensions/
testdata/
  panic/
  log/
  launchd/
  macho/
docs/
  design/
  examples/

Package responsibilities:

* cmd/macscope: command wiring only
* internal/cli: argument parsing and command dispatch
* internal/collect: command execution helpers and shared collection primitives
* internal/output: table, text, and JSON renderers
* internal/process: ps, lsof, PID, PPID, process-path helpers
* internal/codesign: codesign parsing and signature model
* internal/gatekeeper: spctl parsing and assessment model
* internal/macho: Mach-O, architecture, otool, lipo, nm, and strings helpers
* internal/launchd: LaunchAgent/LaunchDaemon parsing and scoring
* internal/tcc: TCC and privacy-denial log parsing
* internal/endpointsecurity: ES denial and future ES agent integration points
* internal/vpn: VPN, scutil, DNS, proxy, route, and utun checks
* internal/panic: panic log parser, watchdog classifier, and correlation engine
* internal/logquery: safe wrappers around log show and log stream
* internal/pmset: sleep/wake/shutdown correlation helpers
* internal/systemextensions: systemextensionsctl and extension inventory

⸻

CLI Shape

The desired CLI is:

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

Prefer stable command names over changing the interface repeatedly.

⸻

Go Standards

Use idiomatic Go.

Required:

* gofmt all Go files
* meaningful package names
* small functions
* explicit error returns
* table-driven tests for parsers
* context-aware command execution where possible
* no panics for normal error paths
* no global mutable state unless unavoidable
* no shell interpolation with untrusted values

Prefer standard library first.

Allowed dependencies should be minimal and justified. If adding a dependency, explain why the standard library is insufficient.

Recommended libraries only if needed:

* spf13/cobra for CLI command structure
* mattn/go-isatty or similar only if terminal detection becomes necessary

Avoid heavy frameworks.

⸻

Command Execution Rules

Many features wrap native macOS tools. Use exec.CommandContext, not shell strings.

Good:

exec.CommandContext(ctx, "log", "show", "--last", since, "--style", "syslog", "--predicate", predicate)

Bad:

exec.Command("sh", "-c", "log show ... " + userInput)

When invoking macOS tools:

* capture stdout and stderr separately when useful
* include command timeout support
* return structured errors
* preserve raw output in debug mode
* avoid requiring sudo unless the command explicitly needs it

⸻

Output Requirements

Every command should support human-readable output. Commands that return structured findings should also support JSON.

Human output should be concise and evidence-driven:

Panic Type:
  WATCHDOG_TIMEOUT
Summary:
  watchdogd failed to check in for 92 seconds.
Likely Causes:
  BOOT_IOKIT_STALL         confidence=0.81
  EXTERNAL_DISPLAY_DOCK    confidence=0.72
Evidence:
  - panic string contains watchdog timeout
  - IOKit Boot phase detected
  - AppleDisplayCrossbar found near panic window

JSON output should be stable and machine-readable:

{
  "panic_type": "WATCHDOG_TIMEOUT",
  "watchdog_timeout_sec": 92,
  "suspected_causes": [
    {
      "category": "BOOT_IOKIT_STALL",
      "confidence": 0.81,
      "evidence": ["IOKit Boot phase detected"]
    }
  ]
}

Never emit invalid JSON from --json commands.

⸻

Panic / Watchdog Triage Requirements

The panic module should parse logs such as:

/Library/Logs/DiagnosticReports/*.panic
/Library/Logs/DiagnosticReports/panic-full*.panic

It must detect:

* kernel panic string
* watchdog timeout
* timeout duration
* CPU number if present
* caller address if present
* macOS version if present
* boot session UUID if present
* embedded panic log metadata
* iBoot / pre-OS log markers
* IOKit Boot installer phase
* SOCD presence or absence
* panic report path

Important regexes:

panic\(cpu ([0-9]+).*\): (.*)
watchdog timeout: no checkins from watchdogd in ([0-9]+) seconds
bootsessionuuid: ([A-Fa-f0-9\-]+)
osversion: ([^,]+)
Current Phase = "([^"]+)"
Saved type '210.*' report .* at (\/Library\/Logs\/DiagnosticReports\/[^ ]+\.panic)

Watchdog classification:

* If panic string contains watchdog timeout, set PanicType = WATCHDOG_TIMEOUT.
* If watchdogd missed checkins, explain that the system likely stalled hard enough for the watchdog to force a panic/reboot.
* If IOKit Boot appears nearby, add BOOT_IOKIT_STALL as a suspected cause.
* If display/dock indicators appear nearby, add EXTERNAL_DISPLAY_OR_DOCK as a suspected cause.

Display/dock indicators:

AppleDisplayCrossbar
AppleATCDPINAdapterPort
DisplayPort
Thunderbolt
IOGPU
AGX
USB-C

Software-update indicators:

Installer Progress
softwareupdated
IOKit Boot
PanicMedic

⸻

Log Query Rules

Use unified logging through log show and log stream wrappers.

Useful predicates:

TCC:

process == "tccd" OR subsystem CONTAINS[c] "TCC" OR eventMessage CONTAINS[c] "kTCC" OR eventMessage CONTAINS[c] "deny"

LLDB attach failures:

eventMessage CONTAINS[c] "task_for_pid" OR eventMessage CONTAINS[c] "debug" OR process == "amfid" OR process == "tccd" OR process == "taskgated" OR process == "syspolicyd"

EndpointSecurity:

eventMessage CONTAINS[c] "EndpointSecurity" OR eventMessage CONTAINS[c] "/dev/es" OR eventMessage CONTAINS[c] "com.apple.developer.endpoint-security.client"

VPN:

subsystem CONTAINS[c] "vpn" OR process CONTAINS[c] "vpn" OR eventMessage CONTAINS[c] "utun" OR eventMessage CONTAINS[c] "IPSec" OR eventMessage CONTAINS[c] "IKE" OR eventMessage CONTAINS[c] "disconnect"

Panic/watchdog:

eventMessage CONTAINS[c] "DumpPanic" OR eventMessage CONTAINS[c] "panic" OR eventMessage CONTAINS[c] "watchdog" OR eventMessage CONTAINS[c] "IOKit Boot"

Keep predicates centralized in code so they are easy to audit and update.

⸻

Triage Scoring Rules

Scoring must be explainable. Never output a score without evidence.

Example categories:

UNSIGNED_BINARY
INVALID_SIGNATURE
GATEKEEPER_REJECTED
QUARANTINE_PRESENT
USER_WRITABLE_PERSISTENCE
TCC_DENIAL
ENDPOINTSECURITY_DENIAL
WATCHDOG_TIMEOUT
BOOT_IOKIT_STALL
EXTERNAL_DISPLAY_OR_DOCK
NETWORK_EXTENSION_STALL
SECURITY_SOFTWARE_INTERFERENCE

Each finding should contain:

* category
* severity
* confidence
* evidence strings
* source command or source file

⸻

Testing Requirements

Before considering work complete, run:

go test ./...
go vet ./...
gofmt -w .

Add tests for:

* panic parser
* watchdog timeout extraction
* codesign output parser
* spctl output parser
* launchd plist parser
* TCC log parser
* VPN log parser

Use testdata/ fixtures. Do not require a real panic log or live macOS system for unit tests.

Prefer table-driven tests:

func TestParseWatchdogTimeout(t *testing.T) {
    tests := []struct {
        name string
        input string
        want int
        ok bool
    }{...}
}

⸻

Platform Rules

This is a macOS-focused tool.

* It may compile on non-macOS where practical, but macOS behavior is primary.
* Use build tags for macOS-specific collectors.
* Commands that require macOS should return a clear unsupported-platform error on Linux/Windows.
* Do not fake macOS command output in production code.

⸻

EndpointSecurity Roadmap

The first Go version should not require EndpointSecurity entitlements.

Initial implementation should wrap built-in macOS tools only.

Future EndpointSecurity work should be split into:

agent/
  SystemExtension/
  Shared/
internal/endpointsecurity/
  client protocol
  event model
  JSONL decoder

The Go CLI should communicate with a future ES agent through a local, explicit IPC boundary such as:

* Unix domain socket
* XPC bridge
* JSONL file during development

Do not mix privileged ES code directly into the normal CLI path.

⸻

Privacy / Redaction Rules

Some macOS logs contain hostnames, usernames, serial-like identifiers, paths, ECIDs, panic UUIDs, and device metadata.

Default human output should redact obvious sensitive values when they are not necessary.

Examples:

* serial numbers
* ECID values
* full home directory paths when not needed
* user names in paths
* VPN endpoint hostnames if not requested with --full

--json may include raw fields, but avoid accidental secrets. --full can preserve more evidence.

⸻

Documentation Expectations

When adding a command, update:

* README.md
* command help text
* at least one example under docs/examples/
* tests or fixtures where parsing is involved

Documentation should explain:

* what the command does
* what native macOS tools it invokes
* what permissions may be required
* what the output means
* limitations

⸻

Git / Change Discipline

Keep changes small and reviewable.

For each task:

1. Inspect existing structure first.
2. Make the smallest coherent change.
3. Add tests for parsing or scoring logic.
4. Run formatting and tests.
5. Summarize what changed and what was not completed.

Do not rewrite the entire project unless explicitly asked.

⸻

Implementation Order

Suggested milestone order:

Milestone 1: Go CLI skeleton

* macscope help
* command routing
* shared command execution helper
* text and JSON output interfaces

Milestone 2: macho

* file identity
* architecture
* codesign
* spctl
* xattrs
* linked libraries

Milestone 3: panic

* parse latest panic file
* detect watchdog timeout
* render text + JSON
* add test fixtures

Milestone 4: attach

* process lookup
* developer group check
* signing checks
* attach-relevant log correlation

Milestone 5: persist

* LaunchAgents / LaunchDaemons
* plist parser
* suspicion scoring

Milestone 6: tcc, es, vpn

* log predicates
* structured parsers
* human summaries

Milestone 7: timeline and correlation

* normalize events
* SQLite or JSONL store
* cross-command timeline views

⸻

Definition of Done

A change is done when:

* it builds on macOS
* go test ./... passes
* go vet ./... passes
* Go files are formatted
* parsing logic has tests
* command output is understandable
* JSON output is valid when applicable
* errors are actionable
* README/help text is updated for user-facing commands

⸻

Tone and UX

macscope should feel like a calm senior macOS debugging assistant.

Good output:

Panic Type: WATCHDOG_TIMEOUT
Summary: watchdogd failed to check in for 92 seconds, indicating a system-wide stall.
Evidence: panic-full-2026-05-19-142007.0003.panic
Next checks: external dock/display, software update state, third-party system extensions.

Avoid vague alarmist language.

Do not say something is malware, compromised, or hardware failure unless there is direct evidence. Use terms like:

* suspicious
* likely
* possible
* confidence
* evidence
* next check
