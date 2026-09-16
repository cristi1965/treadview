#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backend_dir="$repo_root/app/backend"
service_label="com.stockgod.trading-cockpit"
service_domain="gui/$(id -u)"
support_dir="$HOME/Library/Application Support/TradingAgents/runtime"
binary_path="$support_dir/tradingagents-backend"
log_file="$support_dir/backend.log"
plist_source="$repo_root/scripts/$service_label.plist"
plist_target="$HOME/Library/LaunchAgents/$service_label.plist"
port="8765"
base_url="http://127.0.0.1:$port"

healthy() {
  curl -fsS --max-time 2 "$base_url/api/health" >/dev/null 2>&1
}

service_loaded() {
  launchctl print "$service_domain/$service_label" >/dev/null 2>&1
}

service_pid() {
  launchctl print "$service_domain/$service_label" 2>/dev/null |
    awk -F'= ' '/^[[:space:]]*pid = / {gsub(/[^0-9]/, "", $2); print $2; exit}'
}

status() {
  local pid=""
  if service_loaded; then
    pid="$(service_pid)"
    if [[ -n "$pid" ]] && healthy; then
      echo "RUNNING pid=$pid url=$base_url"
      lsof -nP -a -p "$pid" -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | tail -n +2 || true
      echo "binary=$binary_path"
      echo "cwd=$backend_dir"
      echo "log=$log_file"
      return 0
    fi
    echo "STARTING label=$service_label url=$base_url"
    return 1
  fi
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "UNMANAGED port=$port is already in use" >&2
    lsof -nP -iTCP:"$port" -sTCP:LISTEN >&2
    return 2
  fi
  echo "STOPPED url=$base_url"
  return 1
}

install() {
  local pid
  if service_loaded && healthy; then
    status
    return 0
  fi
  if ! service_loaded && lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    status
    return 2
  fi

  echo "Building current backend source..."
  mkdir -p "$support_dir" "$HOME/Library/LaunchAgents"
  local build_path="$support_dir/.tradingagents-backend.build"
  (cd "$backend_dir" && go build -o "$build_path" .)
  chmod 0755 "$build_path"
  mv -f "$build_path" "$binary_path"
  if [[ -f "$log_file" ]] && [[ "$(stat -f '%z' "$log_file")" -gt 10485760 ]]; then
    mv "$log_file" "$log_file.$(date -u '+%Y%m%dT%H%M%SZ')"
  fi
  cp "$plist_source" "$plist_target"
  plutil -lint "$plist_target" >/dev/null
  launchctl bootout "$service_domain/$service_label" >/dev/null 2>&1 || true
  launchctl bootstrap "$service_domain" "$plist_target"
  launchctl kickstart -k "$service_domain/$service_label"

  for _ in $(seq 1 100); do
    if healthy; then
      status
      return 0
    fi
    sleep 0.1
  done
  pid="$(service_pid)"
  echo "Backend did not become healthy within 10 seconds (pid=${pid:-unknown}). Recent log:" >&2
  tail -60 "$log_file" >&2 2>/dev/null || true
  return 1
}

stop() {
  if service_loaded; then
    launchctl bootout "$service_domain/$service_label"
    echo "STOPPED label=$service_label"
  else
    echo "No managed backend service is loaded."
  fi
}

uninstall() {
  launchctl bootout "$service_domain/$service_label" >/dev/null 2>&1 || true
  rm -f "$plist_target"
  echo "UNINSTALLED label=$service_label"
  echo "Runtime binary and log remain in: $support_dir"
}

case "${1:-status}" in
  start|install) install ;;
  stop) stop ;;
  restart) stop; install ;;
  uninstall) uninstall ;;
  status) status ;;
  logs) tail -n "${LINES:-80}" "$log_file" ;;
  *) echo "Usage: $0 {start|stop|restart|status|logs|uninstall}" >&2; exit 2 ;;
esac
