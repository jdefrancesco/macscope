# Launch agents and daemons

```sh
macscope agents
macscope daemons
macscope daemons --json
macscope agents --full
macscope agents --dir testdata/launchd --json
```

These read-only commands inventory installed launchd configuration. Daemons are loaded in the system domain at boot; agents are loaded for user login sessions. Default paths include `/System/Library/LaunchAgents` or `LaunchDaemons`, `/Library/LaunchAgents` or `LaunchDaemons`, and the current user's `~/Library/LaunchAgents` for agents. Other users' home directories are not scanned.

Rows show label, explicit RunAtLoad, KeepAlive (including conditional dictionaries), plist Disabled, executable, and source plist. RunAtLoad requests execution when loaded. KeepAlive can also cause startup execution; conditional dictionaries depend on their keys. Jobs without either may launch on demand or other triggers. Disabled is a plist default, not effective runtime state: launchctl overrides can change it. Installed configuration does not prove a job loaded or ran on this boot.

XML plists are parsed directly; binary plists are converted in memory using `/usr/bin/plutil -convert xml1 -o -` without modifying files. No sudo is normally needed. Unreadable files and parse failures appear as collection errors. Missing directories are skipped. Default inventory requires macOS; explicit `--dir` supports offline XML analysis on other platforms. Repeat `--dir` to replace default paths with multiple directories.

Human output redacts usernames in `/Users/` paths unless `--full` is supplied. JSON preserves raw fields and reports an empty jobs array when nothing is found. Use `macscope persist` for evidence-based persistence scoring.
