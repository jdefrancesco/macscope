#!/bin/zsh
# macscope.zsh
# Modern macOS introspection MVP
#
# Usage:
#   ./macscope.zsh macho <path>
#   ./macscope.zsh attach <pid>
#   ./macscope.zsh persist
#   ./macscope.zsh tcc [--last 30m|--watch]
#   ./macscope.zsh vpn ["VPN Name"]
#   ./macscope.zsh es [--last 30m|--watch]
#   ./macscope.zsh proc <pid|name>
#
# This MVP avoids EndpointSecurity entitlements and uses built-in tooling:
# log, codesign, spctl, xattr, ps, launchctl, scutil, ifconfig, netstat,
# lsof, otool, nm, strings, vmmap, sample, fs_usage.

set -u
set -o pipefail

SELF="${0:t}"
CMD="${1:-help}"
[[ $# -gt 0 ]] && shift

# ---------- formatting ----------

hr() { print -r -- "================================================================================"; }
section() { hr; print -r -- "[*] $1"; hr; }
sub() { print -r -- "--- $1 ---"; }
note() { print -r -- "[+] $1"; }
warn() { print -r -- "[!] $1"; }
err() { print -r -- "[-] $1" >&2; }

have() { command -v "$1" >/dev/null 2>&1; }
need() {
  if ! have "$1"; then
    err "missing required command: $1"
    return 1
  fi
}

run() {
  local title="$1"
  shift
  sub "$title"
  "$@" 2>&1 || true
  echo
}

usage() {
  cat <<'EOF'
macscope.zsh - modern macOS introspection MVP

Commands:
  macho <path>              Inspect binary/app: signature, Gatekeeper, Mach-O, xattrs, strings
  attach <pid>              Explain likely LLDB attach failures using logs/signing/dev-group checks
  persist                   Hunt launchd persistence and suspicious launch items
  tcc [--last 30m]          Show recent TCC/privacy denials
  tcc --watch               Live-watch TCC/privacy denials
  vpn [VPN Name]            Inspect VPN state, utun interfaces, DNS/proxy/routes/recent logs
  es [--last 30m]           Show EndpointSecurity-related denials/logs
  es --watch                Live-watch EndpointSecurity-related logs
  proc <pid|name>           Process triage: ps, signing, lsof, vmmap/sample suggestions
  help                      Show this help

Examples:
  ./macscope.zsh macho /Applications/Foo.app
  ./macscope.zsh attach 1234
  ./macscope.zsh persist
  ./macscope.zsh tcc --watch
  ./macscope.zsh vpn "Work VPN"
  ./macscope.zsh es --last 1h
EOF
}

# ---------- common helpers ----------

resolve_app_exec() {
  local target="$1"
  if [[ -d "$target" && "$target" == *.app ]]; then
    local info="$target/Contents/Info.plist"
    local exe=""
    if [[ -f "$info" ]]; then
      exe="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$info" 2>/dev/null || true)"
      if [[ -n "$exe" && -f "$target/Contents/MacOS/$exe" ]]; then
        print -r -- "$target/Contents/MacOS/$exe"
        return 0
      fi
    fi
  fi
  print -r -- "$target"
}

codesign_summary() {
  local path="$1"
  run "codesign details" codesign -dvvv --entitlements :- "$path"
  run "codesign verify" codesign --verify --deep --strict --verbose=4 "$path"
  run "Gatekeeper assessment" spctl --assess --type execute --verbose=4 "$path"
  run "xattrs" xattr -l "$path"
}

path_sha256() {
  local path="$1"
  if have shasum; then
    shasum -a 256 "$path" 2>/dev/null | awk '{print $1}'
  fi
}

team_id_for_path() {
  local path="$1"
  codesign -dv --verbose=4 "$path" 2>&1 | awk -F= '/^TeamIdentifier=/{print $2}'
}

is_probably_pid() {
  [[ "$1" == <-> ]]
}

pid_path() {
  local pid="$1"
  ps -p "$pid" -o command= 2>/dev/null | awk '{print $1}'
}

proc_name_from_pid() {
  local pid="$1"
  ps -p "$pid" -o comm= 2>/dev/null | xargs basename 2>/dev/null
}

# ---------- command: macho ----------

cmd_macho() {
  local target="${1:-}"
  if [[ -z "$target" ]]; then
    err "usage: $SELF macho <path>"
    return 1
  fi
  if [[ ! -e "$target" ]]; then
    err "not found: $target"
    return 1
  fi

  local bin
  bin="$(resolve_app_exec "$target")"

  section "Target"
  print -r -- "input:  $target"
  print -r -- "binary: $bin"
  print -r -- "sha256: $(path_sha256 "$bin")"
  echo

  section "Identity and architecture"
  run "file" file "$bin"
  have lipo && run "lipo -info" lipo -info "$bin"
  have otool && run "otool -hv" otool -hv "$bin"

  section "Signature / policy"
  codesign_summary "$bin"

  if [[ -d "$target" && "$target" == *.app && -f "$target/Contents/Info.plist" ]]; then
    section "Bundle metadata"
    run "Info.plist" /usr/libexec/PlistBuddy -c 'Print' "$target/Contents/Info.plist"
  fi

  section "Linked libraries"
  have otool && run "otool -L" otool -L "$bin"

  section "Symbols snapshot"
  if have nm; then
    sub "nm -m first 120 lines"
    nm -m "$bin" 2>&1 | head -n 120 || true
    echo
  else
    warn "nm unavailable"
  fi

  section "Interesting strings"
  if have strings; then
    strings -a "$bin" 2>/dev/null \
      | grep -Ei 'https?://|socket|launchd|LaunchAgent|LaunchDaemon|DYLD_|@rpath|/tmp/|/private/|xpc|mach|endpointsecurity|tccd|amfid|syspolicyd|plist|screen|camera|microphone|keychain|vpn|utun' \
      | head -n 120 || true
  else
    warn "strings unavailable"
  fi
}

# ---------- command: attach ----------

cmd_attach() {
  local pid="${1:-}"
  if [[ -z "$pid" || ! "$pid" == <-> ]]; then
    err "usage: $SELF attach <pid>"
    return 1
  fi

  local ppath pname
  ppath="$(pid_path "$pid")"
  pname="$(proc_name_from_pid "$pid")"

  section "Target process"
  run "ps" ps -p "$pid" -o pid,ppid,user,group,lstart,etime,stat,command
  print -r -- "process name: ${pname:-unknown}"
  print -r -- "process path: ${ppath:-unknown}"
  echo

  section "Debugger prerequisites"
  run "developer group membership" dseditgroup -o checkmember -m "$(whoami)" _developer
  run "current user id/groups" id

  if [[ -n "$ppath" && -e "$ppath" ]]; then
    section "Target signature / policy"
    codesign_summary "$ppath"
  else
    warn "could not resolve executable path for codesign/spctl"
  fi

  section "Recent attach-relevant logs"
  log show --last 30m --style syslog --info --debug \
    --predicate 'eventMessage CONTAINS[c] "task_for_pid" OR eventMessage CONTAINS[c] "debug" OR process == "amfid" OR process == "tccd" OR process == "taskgated" OR process == "syspolicyd"' \
    2>/dev/null | tail -n 250 || true

  section "Interpretation checklist"
  cat <<'EOF'
Look for:
- taskgated / task_for_pid denial
- tccd denial involving Developer Tools, Debugging, Automation, or Full Disk Access
- amfid / syspolicyd signature or policy rejection
- target protected by platform policy, SIP, hardened runtime, or library validation
- debugger user not in _developer group
EOF
}

# ---------- command: persist ----------

print_plist_summary() {
  local plist="$1"
  local label program run keep args
  label="$(/usr/libexec/PlistBuddy -c 'Print :Label' "$plist" 2>/dev/null || true)"
  program="$(/usr/libexec/PlistBuddy -c 'Print :Program' "$plist" 2>/dev/null || true)"
  run="$(/usr/libexec/PlistBuddy -c 'Print :RunAtLoad' "$plist" 2>/dev/null || true)"
  keep="$(/usr/libexec/PlistBuddy -c 'Print :KeepAlive' "$plist" 2>/dev/null || true)"
  args="$(/usr/libexec/PlistBuddy -c 'Print :ProgramArguments' "$plist" 2>/dev/null | tr '\n' ' ' || true)"

  [[ -z "$program" && -n "$args" ]] && program="$(print -r -- "$args" | awk '{print $2}')"

  local score=0 reasons=()
  [[ "$program" == /tmp/* || "$program" == /private/tmp/* || "$program" == "$HOME"/* ]] && { score=$((score+20)); reasons+=("user-writable/transient program path"); }
  [[ "$run" == "true" ]] && { score=$((score+5)); reasons+=("RunAtLoad"); }
  [[ "$keep" == "true" ]] && { score=$((score+5)); reasons+=("KeepAlive"); }

  printf "%-5s %-45s %s\n" "$score" "${label:-<no label>}" "$plist"
  [[ -n "$program" ]] && print -r -- "      Program: $program"
  [[ -n "$args" ]] && print -r -- "      Args: $args"
  [[ ${#reasons[@]} -gt 0 ]] && print -r -- "      Reasons: ${(j:, :)reasons}"
}

cmd_persist() {
  section "Launchd persistence files"
  local dirs=(
    "/Library/LaunchAgents"
    "/Library/LaunchDaemons"
    "$HOME/Library/LaunchAgents"
  )

  print -r -- "SCORE LABEL                                         PLIST"
  for d in "$dirs[@]"; do
    [[ -d "$d" ]] || continue
    for plist in "$d"/*.plist(N); do
      print_plist_summary "$plist"
    done
  done
  echo

  section "launchctl loaded services - system"
  launchctl print system 2>/dev/null | grep -Ei 'label|program|programarguments|path|state =' | head -n 300 || true
  echo

  section "launchctl loaded services - gui/$(id -u)"
  launchctl print "gui/$(id -u)" 2>/dev/null | grep -Ei 'label|program|programarguments|path|state =' | head -n 300 || true
  echo

  section "Login items"
  osascript -e 'tell application "System Events" to get the name of every login item' 2>/dev/null || true
  echo

  section "Suspicious grep across launch paths"
  grep -RniE 'RunAtLoad|KeepAlive|ProgramArguments|/tmp/|/private/tmp|/var/folders|curl|osascript|python|perl|ruby|sh -c|zsh -c|bash -c' \
    /Library/LaunchAgents /Library/LaunchDaemons "$HOME/Library/LaunchAgents" \
    2>/dev/null | head -n 300 || true
}

# ---------- command: tcc ----------

cmd_tcc() {
  local mode="show"
  local last="30m"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --watch) mode="watch"; shift ;;
      --last) last="${2:-30m}"; shift 2 ;;
      *) err "unknown tcc arg: $1"; return 1 ;;
    esac
  done

  local pred='process == "tccd" OR subsystem CONTAINS[c] "TCC" OR eventMessage CONTAINS[c] "kTCC" OR eventMessage CONTAINS[c] "deny"'
  if [[ "$mode" == "watch" ]]; then
    log stream --style syslog --info --debug --predicate "$pred"
  else
    section "TCC/privacy logs last $last"
    log show --last "$last" --style syslog --info --debug --predicate "$pred" 2>/dev/null | tail -n 300 || true
  fi
}

# ---------- command: es ----------

cmd_es() {
  local mode="show"
  local last="30m"
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --watch) mode="watch"; shift ;;
      --last) last="${2:-30m}"; shift 2 ;;
      *) err "unknown es arg: $1"; return 1 ;;
    esac
  done

  local pred='eventMessage CONTAINS[c] "EndpointSecurity" OR eventMessage CONTAINS[c] "/dev/es" OR eventMessage CONTAINS[c] "com.apple.developer.endpoint-security.client"'
  if [[ "$mode" == "watch" ]]; then
    section "EndpointSecurity live logs"
    log stream --style syslog --info --debug --predicate "$pred"
  else
    section "EndpointSecurity logs last $last"
    log show --last "$last" --style syslog --info --debug --predicate "$pred" 2>/dev/null | tail -n 300 || true
    section "Hint: catch /dev/es access live"
    print -r -- "sudo fs_usage -w | grep -i '/dev/es'"
  fi
}

# ---------- command: vpn ----------

cmd_vpn() {
  local vpn_name="${1:-}"

  section "Configured VPN services"
  scutil --nc list 2>&1 || true
  echo

  if [[ -n "$vpn_name" ]]; then
    section "VPN status: $vpn_name"
    scutil --nc status "$vpn_name" 2>&1 || true
    echo
  fi

  section "DNS"
  scutil --dns 2>&1 | head -n 250 || true
  echo

  section "Proxy"
  scutil --proxy 2>&1 || true
  echo

  section "Interfaces: utun and primary"
  ifconfig 2>&1 | awk '/^[a-z0-9]+: flags=/{iface=$1} /utun|inet |status:/{print iface, $0}' | head -n 250 || true
  echo

  section "Routes"
  run "default route" route -n get default
  run "route table utun/default" sh -c 'netstat -rn | grep -E "default|utun|UGSc|UGS" | head -n 120'

  section "Interface counters"
  netstat -i 2>&1 | grep -E 'Name|utun|en[0-9]' || true
  echo

  section "Recent VPN/utun logs"
  log show --last 60m --style syslog --info --debug \
    --predicate 'subsystem CONTAINS[c] "vpn" OR process CONTAINS[c] "vpn" OR eventMessage CONTAINS[c] "utun" OR eventMessage CONTAINS[c] "IPSec" OR eventMessage CONTAINS[c] "IKE" OR eventMessage CONTAINS[c] "disconnect"' \
    2>/dev/null | tail -n 300 || true
  echo

  section "Sleep/wake correlation"
  pmset -g log 2>/dev/null | grep -Ei 'sleep|wake|darkwake|tcpkeepalive|network' | tail -n 100 || true
}

# ---------- command: proc ----------

cmd_proc() {
  local q="${1:-}"
  if [[ -z "$q" ]]; then
    err "usage: $SELF proc <pid|name>"
    return 1
  fi

  local pid="$q"
  if ! is_probably_pid "$q"; then
    pid="$(pgrep -n -x "$q" 2>/dev/null || pgrep -n -f "$q" 2>/dev/null || true)"
  fi

  if [[ -z "$pid" ]]; then
    err "no matching process: $q"
    return 1
  fi

  local ppath pname
  ppath="$(pid_path "$pid")"
  pname="$(proc_name_from_pid "$pid")"

  section "Process"
  run "ps" ps -p "$pid" -o pid,ppid,user,group,lstart,etime,stat,command
  print -r -- "name: ${pname:-unknown}"
  print -r -- "path: ${ppath:-unknown}"
  echo

  if [[ -n "$ppath" && -e "$ppath" ]]; then
    section "Signature / policy"
    codesign_summary "$ppath"
  fi

  section "Open files / network"
  have lsof && run "lsof" lsof -n -P -p "$pid"
  have lsof && run "lsof network" lsof -i -n -P -a -p "$pid"

  section "Memory map hint"
  print -r -- "vmmap $pid | less"
  print -r -- "sample $pid 5 -file sample.$pid.txt"
  print -r -- "sudo fs_usage -w -p $pid"
}

# ---------- dispatch ----------

case "$CMD" in
  help|-h|--help) usage ;;
  macho) cmd_macho "$@" ;;
  attach) cmd_attach "$@" ;;
  persist) cmd_persist "$@" ;;
  tcc) cmd_tcc "$@" ;;
  es) cmd_es "$@" ;;
  vpn) cmd_vpn "$@" ;;
  proc) cmd_proc "$@" ;;
  *) err "unknown command: $CMD"; usage; exit 1 ;;
esac

