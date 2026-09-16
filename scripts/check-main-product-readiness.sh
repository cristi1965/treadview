#!/usr/bin/env bash
set -euo pipefail

base_url="${BASE_URL:-http://127.0.0.1:8765}"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/tradingagents-readiness-check.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT INT TERM

failures=0

check_json() {
  local label="$1" path="$2" expression="$3" response status
  response="$work_dir/${label}.json"
  status="$(curl -sS --max-time 8 -o "$response" -w '%{http_code}' "$base_url$path" || true)"
  if [[ "$status" != "200" ]]; then
    echo "FAIL $label: HTTP $status"
    jq -r '.checks[]? | select(.status != "ok") | "  - \(.id): \(.detail)"' "$response" 2>/dev/null || true
    failures=$((failures + 1))
    return
  fi
  if ! jq -e "$expression" "$response" >/dev/null 2>&1; then
    echo "FAIL $label: response contract mismatch"
    failures=$((failures + 1))
    return
  fi
  echo "PASS $label"
}

check_json "health" "/api/health" '.liveness == "ok"'
check_json "full-system" "/api/readiness?profile=full" '.status == "ok" and .dataStatus == "ok"'
check_json "historical-research" "/api/readiness?profile=historical-research" '.ready == true and .profile == "historical-research"'
check_json "paper-engine" "/api/readiness?profile=paper-engine" '.ready == true and .profile == "paper-engine"'

live_response="$work_dir/live-trading-data.json"
live_status="$(curl -sS --max-time 15 -o "$live_response" -w '%{http_code}' "$base_url/api/readiness?profile=live-trading-data" || true)"
if [[ "$live_status" == "200" ]] && jq -e '.ready == true' "$live_response" >/dev/null 2>&1; then
  echo "PASS live-trading-data"
else
  echo "FAIL live-trading-data: HTTP $live_status"
  jq -r '.checks[]? | select(.status != "ok") | "  - \(.id): \(.detail)"' "$live_response" 2>/dev/null || true
  failures=$((failures + 1))
fi

if (( failures > 0 )); then
  echo "MAIN PRODUCT NOT READY: $failures mandatory gate(s) failed"
  exit 1
fi
echo "FULL SYSTEM READY: liveness + full data + historical research + Paper engine + live trading data"
