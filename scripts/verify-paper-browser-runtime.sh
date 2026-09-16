#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
runtime_dir="$(mktemp -d "${TMPDIR:-/tmp}/stockgod-paper-browser.XXXXXX")"
binary_path="$runtime_dir/tradingagents-backend"
server_pid=""

cleanup() {
	if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then
		kill "$server_pid" 2>/dev/null || true
		wait "$server_pid" 2>/dev/null || true
	fi
	rm -rf "$runtime_dir"
}
trap cleanup EXIT INT TERM

node_bin="$(command -v node)"
node_major="$($node_bin -p 'process.versions.node.split(".")[0]')"
if (( node_major < 18 )) && command -v volta >/dev/null 2>&1; then
	node_bin="$(cd "$repo_root/app/frontend" && volta which node)"
	node_major="$($node_bin -p 'process.versions.node.split(".")[0]')"
fi
if (( node_major < 18 )); then
	echo "Paper browser acceptance requires Node.js 18 or newer." >&2
	exit 1
fi

echo "Building current frontend and one temporary backend for isolated desktop and mobile Paper browser acceptance."
(
	cd "$repo_root/app/frontend"
	env PATH="$(dirname "$node_bin"):$PATH" npm run build
)
(
	cd "$repo_root/app/backend"
	go build -o "$binary_path" .
)

for device in desktop mobile; do
	port=""
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

	database_dir="$runtime_dir/database-$device"
	server_log="$runtime_dir/server-$device.log"
	mkdir -p "$database_dir"
	(
		cd "$repo_root/app/backend"
		env PORT="$port" \
			STOCKGOD_DATABASE_DIR="$database_dir" \
			STOCKGOD_ADMIN_TOKEN="paper-browser-$device-$RANDOM-$$" \
			STOCKGOD_PAPER_RUNTIME_FIXTURE="temporary-local-acceptance-v1" \
			STOCKGOD_PAPER_RUNTIME_MANUAL_BASELINE="true" \
			STOCKGOD_REPLAY="false" \
			STOCKGOD_LIVE_MIRROR="false" \
			STOCKGOD_LOCAL_FRONTEND="true" \
			TRADINGAGENTS_RESULTS_DIR="$runtime_dir/results-$device" \
			TRADINGAGENTS_CACHE_DIR="$runtime_dir/cache-$device" \
			TRADINGAGENTS_LLM_PROVIDER="openai_compatible" \
			TRADINGAGENTS_LLM_BACKEND_URL="http://127.0.0.1:9/v1" \
			OPENAI_COMPATIBLE_API_KEY="paper-runtime-no-network" \
			"$binary_path"
	) >"$server_log" 2>&1 &
	server_pid=$!

	ready="false"
	for _ in $(seq 1 100); do
		if curl -fsS "http://127.0.0.1:$port/api/health" >/dev/null 2>&1; then
			ready="true"
			break
		fi
		if ! kill -0 "$server_pid" 2>/dev/null; then
			echo "Temporary $device Paper backend exited before health check" >&2
			tail -80 "$server_log" >&2
			exit 1
		fi
		sleep 0.1
	done
	if [[ "$ready" != "true" ]]; then
		echo "Temporary $device Paper backend did not become healthy" >&2
		tail -80 "$server_log" >&2
		exit 1
	fi

	(
		cd "$repo_root/app/frontend"
		PAPER_LIFECYCLE_WRITES=true PAPER_BASE_URL="http://127.0.0.1:$port" PAPER_VIEWPORT="$device" \
			"$node_bin" scripts/paper-lifecycle-audit.mjs
	)
	kill "$server_pid" 2>/dev/null || true
	wait "$server_pid" 2>/dev/null || true
	server_pid=""
done

echo "PASS: isolated desktop and mobile touch Paper lifecycles completed with screenshots and audit evidence."
