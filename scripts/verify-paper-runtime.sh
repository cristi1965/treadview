#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
runtime_dir="$(mktemp -d "${TMPDIR:-/tmp}/stockgod-paper-runtime.XXXXXX")"
binary_path="$runtime_dir/tradingagents-backend"
database_dir="$runtime_dir/database"
server_log="$runtime_dir/server.log"
response_file="$runtime_dir/response.json"
admin_token="paper-runtime-acceptance-$RANDOM-$$"
port=""
server_pid=""

cleanup() {
	if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then
		kill "$server_pid" 2>/dev/null || true
		wait "$server_pid" 2>/dev/null || true
	fi
	rm -rf "$runtime_dir"
}
trap cleanup EXIT INT TERM

for _ in $(seq 1 50); do
	candidate=$((20000 + RANDOM % 30000))
	if ! lsof -nP -iTCP:"$candidate" -sTCP:LISTEN >/dev/null 2>&1; then
		port="$candidate"
		break
	fi
done
if [[ -z "$port" ]]; then
	echo "Unable to allocate a local acceptance port" >&2
	exit 1
fi

api_call() {
	local method="$1"
	local path="$2"
	local payload="${3:-}"
	local args=(-sS -o "$response_file" -w "%{http_code}" -X "$method" -H "Authorization: Bearer $admin_token")
	if [[ -n "$payload" ]]; then
		args+=(-H "Content-Type: application/json" --data "$payload")
	fi
	http_status="$(curl "${args[@]}" "http://127.0.0.1:$port$path")"
}

require_status() {
	local expected="$1"
	local label="$2"
	if [[ "$http_status" != "$expected" ]]; then
		echo "$label returned HTTP $http_status, expected $expected" >&2
		jq . "$response_file" >&2 2>/dev/null || true
		exit 1
	fi
}

echo "Building a temporary backend binary for authenticated PAPER-only acceptance."
echo "The explicit fixture uses only a temporary SQLite database and performs no broker, paid LLM, or external write operation."
mkdir -p "$database_dir"
(
	cd "$repo_root/app/backend"
	go build -o "$binary_path" .
)

PORT="$port" \
STOCKGOD_DATABASE_DIR="$database_dir" \
STOCKGOD_ADMIN_TOKEN="$admin_token" \
STOCKGOD_PAPER_RUNTIME_FIXTURE="temporary-local-acceptance-v1" \
STOCKGOD_REPLAY="false" \
STOCKGOD_LIVE_MIRROR="false" \
STOCKGOD_LOCAL_FRONTEND="false" \
TRADINGAGENTS_RESULTS_DIR="$runtime_dir/results" \
TRADINGAGENTS_CACHE_DIR="$runtime_dir/cache" \
TRADINGAGENTS_LLM_PROVIDER="openai_compatible" \
TRADINGAGENTS_LLM_BACKEND_URL="http://127.0.0.1:9/v1" \
OPENAI_COMPATIBLE_API_KEY="paper-runtime-no-network" \
"$binary_path" >"$server_log" 2>&1 &
server_pid=$!

ready="false"
for _ in $(seq 1 100); do
	if curl -fsS "http://127.0.0.1:$port/api/health" >/dev/null 2>&1; then
		ready="true"
		break
	fi
	if ! kill -0 "$server_pid" 2>/dev/null; then
		echo "Temporary backend exited before health check" >&2
		tail -80 "$server_log" >&2
		exit 1
	fi
	sleep 0.1
done
if [[ "$ready" != "true" ]]; then
	echo "Temporary backend did not become healthy" >&2
	tail -80 "$server_log" >&2
	exit 1
fi

baseline_ready="false"
for _ in $(seq 1 50); do
	api_call GET "/api/paper-orders/scheduler"
	require_status 200 "Paper scheduler baseline status"
	if jq -e '.lastResult.dailyBaselinesCreated >= 1' "$response_file" >/dev/null 2>&1; then
		baseline_ready="true"
		break
	fi
	sleep 0.1
done
if [[ "$baseline_ready" != "true" ]]; then
	echo "Paper scanner did not establish the USD day-start baseline" >&2
	jq . "$response_file" >&2 2>/dev/null || true
	exit 1
fi
api_call GET "/api/paper-orders/portfolio-risk"
require_status 200 "Paper day-start portfolio risk"
jq -e '.accountModel == "currency-isolated-paper-subaccounts" and .baseCurrency.status == "unknown" and .baseCurrency.totalEquity == null and (.groups[] | select(.currency == "USD") | .dailyRiskComplete == true and .dayStartEquity == 100000)' "$response_file" >/dev/null

set_quote() {
	local price="$1"
	api_call POST "/api/_test/paper-runtime/quote" "$(jq -nc --argjson price "$price" '{symbol:"NVDA",price:$price}')"
	require_status 200 "fixture quote update"
}

risk_snapshot='{"investableCapital":100000,"maxLoss":300,"maxRiskAmount":1500,"plannedNotional":10100,"maxNotionalAmount":15000,"riskLimitPassed":true}'
gap_payload="$(jq -nc --argjson risk "$risk_snapshot" '{clientOrderId:"runtime-gap-buy-stop",environment:"PAPER",symbol:"NVDA",side:"BUY",market:"US",currency:"USD",orderType:"STOP_MARKET",timeInForce:"GTC",referencePrice:100,entry:null,triggerPrice:101,protectiveStop:98,takeProfit:130,quantity:100,quoteSource:"client-plan",quoteTime:"2026-09-08T15:00:00Z",riskSnapshot:$risk}')"

set_quote 100
api_call POST "/api/paper-orders" "$gap_payload"
require_status 201 "BUY STOP submission"

set_quote 115
api_call POST "/api/paper-orders/runtime-gap-buy-stop/simulated-fill"
require_status 422 "gap BUY fill policy check"
jq -e '.fillRejected == true and .order.status == "ACCEPTED" and .order.reservedCash > 0 and .order.riskSnapshot.fillPolicyCheck.passed == false and (.order.riskSnapshot.fillPolicyCheck.reasons | join(";") | contains("max loss exceeds"))' "$response_file" >/dev/null

set_quote 100
parent_payload="$(jq -nc --argjson risk "$risk_snapshot" '{clientOrderId:"runtime-oco-parent",environment:"PAPER",symbol:"NVDA",side:"BUY",market:"US",currency:"USD",orderType:"LIMIT",timeInForce:"GTC",referencePrice:100,entry:100,triggerPrice:null,protectiveStop:98,takeProfit:106,quantity:5,quoteSource:"client-plan",quoteTime:"2026-09-08T15:00:00Z",riskSnapshot:$risk}')"
api_call POST "/api/paper-orders" "$parent_payload"
require_status 201 "OCO parent submission"
api_call POST "/api/paper-orders/runtime-oco-parent/simulated-fill" '{"fillId":"runtime-parent-partial-1","quantity":2}'
require_status 200 "OCO parent partial fill"
jq -e '.order.status == "ACCEPTED" and .order.fillQty == 2 and .order.remainingQty == 3 and (.order.fills | length) == 1 and .order.fills[0].quote.price == 100 and .order.fills[0].quote.source == "paper-runtime-fixture" and .order.fills[0].quote.observedAt == "2026-09-08T15:00:00Z" and .order.fills[0].quote.providerURL == "paper-runtime-fixture://local/NVDA"' "$response_file" >/dev/null

api_call POST "/api/paper-orders/runtime-oco-parent/simulated-fill" '{"fillId":"runtime-parent-partial-1","quantity":2}'
require_status 200 "OCO parent partial replay"
jq -e '.idempotentReplay == true and .order.fillQty == 2 and (.order.fills | length) == 1' "$response_file" >/dev/null

api_call POST "/api/paper-orders/runtime-oco-parent/simulated-fill" '{"fillId":"runtime-parent-partial-2","quantity":3}'
require_status 200 "OCO parent completing fill"
jq -e '.order.status == "SIMULATED_FILLED" and .order.environment == "PAPER" and .order.fillQty == 5 and .order.remainingQty == 0 and (.order.fills | length) == 2 and (.order.fills | map(.quantity) == [2,3]) and (.disclaimer | contains("No broker order"))' "$response_file" >/dev/null

api_call GET "/api/paper-orders"
require_status 200 "Paper order listing"
stop_id="$(jq -r '.orders[] | select(.clientOrderId | startswith("auto-stop-")) | .clientOrderId' "$response_file" | head -1)"
if [[ -z "$stop_id" ]]; then
	echo "Automatic protective STOP child was not created" >&2
	exit 1
fi

set_quote 97
api_call POST "/api/paper-orders/$stop_id/simulated-fill" '{"fillId":"runtime-stop-partial-1","quantity":2}'
require_status 200 "protective SELL partial fill"
jq -e '.order.status == "ACCEPTED" and .order.side == "SELL" and .order.fillQty == 2 and .order.remainingQty == 3 and (.order.fills | length) == 1 and .order.fillQuote.price == 97 and .order.fillQuote.providerURL == "paper-runtime-fixture://local/NVDA"' "$response_file" >/dev/null
partial_realized_pnl="$(jq -r '.order.fills[0].realizedPnL' "$response_file")"
api_call GET "/api/paper-orders/portfolio-risk"
require_status 200 "partial SELL realized P&L risk snapshot"
jq -e --argjson realized "$partial_realized_pnl" '.groups[] | select(.currency == "USD") | .realizedPnLToday == $realized' "$response_file" >/dev/null

api_call GET "/api/paper-orders"
require_status 200 "partial OCO order listing"
jq -e --arg stop "$stop_id" '([.orders[] | select(.clientOrderId == $stop)][0] | .status == "ACCEPTED" and .fillQty == 2 and .remainingQty == 3) and ([.orders[] | select((.clientOrderId | startswith("auto-take-profit-")) and .status == "ACCEPTED")][0] | .quantity == 3 and .remainingQty == 3) and ([.orders[] | select((.clientOrderId | startswith("auto-stop-")) or (.clientOrderId | startswith("auto-take-profit-"))) | .reservedQty] | add == 3)' "$response_file" >/dev/null

api_call POST "/api/paper-orders/$stop_id/simulated-fill" '{"fillId":"runtime-stop-partial-1","quantity":2}'
require_status 200 "protective SELL partial replay"
jq -e '.idempotentReplay == true and .order.fillQty == 2 and (.order.fills | length) == 1' "$response_file" >/dev/null

api_call POST "/api/paper-orders/$stop_id/simulated-fill" '{"fillId":"runtime-stop-partial-2","quantity":3}'
require_status 200 "protective SELL completion"
jq -e '.order.status == "SIMULATED_FILLED" and .order.fillQty == 5 and .order.remainingQty == 0 and (.order.fills | map(.quantity) == [2,3])' "$response_file" >/dev/null

api_call GET "/api/paper-orders"
require_status 200 "post-OCO order listing"
jq -e '([.orders[] | select(.clientOrderId | startswith("auto-stop-") or startswith("auto-take-profit-")) | .status] | sort) == ["CANCELLED","SIMULATED_FILLED"]' "$response_file" >/dev/null

api_call GET "/api/admin/audit?limit=500"
require_status 200 "audit listing"
jq -e '.events | any(.action == "paper-order.manual-simulated-fill-record" and (.target | startswith("paper-fill:")) and (.payloadHash | length == 64))' "$response_file" >/dev/null

api_call GET "/api/admin/audit/verify"
require_status 200 "audit verification"
jq -e '.valid == true and .pendingCount == 0 and .count > 0' "$response_file" >/dev/null

api_call GET "/api/readiness?profile=paper-engine"
require_status 200 "Paper engine readiness"
jq -e '.profile == "paper-engine" and .ready == true and (.disclaimer | contains("simulation-only"))' "$response_file" >/dev/null

sqlite3 "$database_dir/trades.db" "UPDATE paper_daily_equity_baselines SET equity = equity + 1 WHERE currency = 'USD';"
api_call GET "/api/admin/audit/verify"
require_status 409 "tampered day-start baseline audit verification"
jq -e '.valid == false and (.reason | contains("paper daily equity baseline outcome hash mismatch"))' "$response_file" >/dev/null

echo "PASS: temporary production router established an audited day-start baseline and completed authenticated gap rejection, BUY and protective-SELL partial-to-complete immutable fills, partial-fill daily realized P&L, OCO resizing/exit, persisted-fill/baseline audit verification, and Paper-engine readiness."
