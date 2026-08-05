#!/usr/bin/env bash
# Local API smoke checks for StockGod self-hosted backend.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8765}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
US_STOCKS="${ROOT}/app/frontend/public/data/us-stocks.json"

echo "==> health ${BASE_URL}"
code="$(curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/api/health" || true)"
if [[ "${code}" != "200" ]]; then
  echo "FAIL: /api/health returned ${code}. Start backend from app/backend first."
  exit 1
fi

python3 - <<PY
import json, os, sys, urllib.request

base = os.environ.get("BASE_URL", "${BASE_URL}")
us_path = "${US_STOCKS}"

def get(path):
    req = urllib.request.Request(base + path)
    with urllib.request.urlopen(req, timeout=30) as r:
        return dict(r.headers), json.loads(r.read().decode())

failed = []

# market
hdr, market = get("/api/market")
src = hdr.get("X-Data-Source") or hdr.get("X-Data-Source".lower()) or ""
# urllib may lower-case header keys depending on version
src = hdr.get("X-Data-Source") or hdr.get("x-data-source") or ""
print("market source:", src)
if "us-stocks" not in src:
    failed.append(f"/api/market source={src!r} expected us-stocks*")
rklb = (market.get("quotes") or {}).get("RKLB") or {}
print("market RKLB:", rklb)
if not rklb or float(rklb.get("price") or 0) <= 0:
    failed.append("RKLB missing/invalid in /api/market")

# compare with us-stocks if present
if os.path.exists(us_path):
    us = json.loads(open(us_path, encoding="utf-8").read())
    row = next((s for s in us.get("stocks", []) if s.get("sym") == "RKLB"), None)
    if row and rklb:
        if abs(float(rklb["price"]) - float(row["price"])) > 0.011:
            failed.append(f"RKLB market price {rklb['price']} != us-stocks {row['price']}")

# stocks cn
hdr, cn = get("/api/stocks?market=cn&limit=5&sort=marketcap")
src = hdr.get("X-Data-Source") or hdr.get("x-data-source") or ""
print("stocks cn source:", src, "total:", cn.get("total"))
syms = [s.get("symbol", "") for s in cn.get("stocks") or []]
print("stocks cn sample:", syms)
if not syms or not all(str(s)[:1].isdigit() for s in syms):
    failed.append(f"cn stocks look wrong: {syms}")
if any(s in ("AAPL", "NVDA") for s in syms):
    failed.append("cn endpoint returned US symbols")

# macro / movers
for path in ("/api/macro", "/api/premarket-movers"):
    hdr, body = get(path)
    print(path, "ok", "keys", list(body)[:5] if isinstance(body, dict) else type(body))

# brand
with urllib.request.urlopen(base + "/api/reports", timeout=30) as r:
    text = r.read().decode("utf-8", "ignore")
count = text.count("我不是股神")
print("reports 我不是股神 count:", count)
if count:
    failed.append(f"reports still contain 我不是股神 x{count}")

if failed:
    print("FAIL:")
    for f in failed:
        print(" -", f)
    sys.exit(1)
print("PASS: verify-local")
PY
