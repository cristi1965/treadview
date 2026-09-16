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
import json, os, sys, urllib.error, urllib.request
from datetime import datetime, timedelta, timezone
try:
    from zoneinfo import ZoneInfo
except Exception:
    ZoneInfo = None

base = os.environ.get("BASE_URL", "${BASE_URL}")
us_path = "${US_STOCKS}"

def get(path):
    req = urllib.request.Request(base + path)
    with urllib.request.urlopen(req, timeout=30) as r:
        return dict(r.headers), json.loads(r.read().decode())

def get_status(path):
    req = urllib.request.Request(base + path)
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            raw = r.read().decode()
            try:
                body = json.loads(raw)
            except Exception:
                body = raw
            return r.status, dict(r.headers), body
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "ignore")
        try:
            body = json.loads(raw)
        except Exception:
            body = raw
        return e.code, dict(e.headers), body

def header(hdr, name):
    return hdr.get(name) or hdr.get(name.lower()) or ""

def parse_data_time(value):
    value = (value or "").strip()
    if not value:
        return None
    if value.startswith("ts:"):
        value = value[3:]
    if value.isdigit() and len(value) >= 13:
        return datetime.fromtimestamp(int(value) / 1000, tz=timezone.utc)
    for candidate in (value, value.replace("Z", "+00:00")):
        try:
            dt = datetime.fromisoformat(candidate)
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return dt.astimezone(timezone.utc)
        except Exception:
            pass
    return None

def check_freshness_headers(path, hdr, status, require_time=True, max_age_hours=None):
    src = header(hdr, "X-Data-Source")
    data_time = header(hdr, "X-Data-Time")
    stale = header(hdr, "X-Data-Stale").lower()
    reason = header(hdr, "X-Data-Stale-Reason")
    partial = header(hdr, "X-Data-Partial-Errors")
    lowered = src.lower()
    if "mock" in lowered or "fallback" in lowered:
        failed.append(f"{path} disallowed data source: {src!r}")
    if status < 400 and require_time and not data_time:
        failed.append(f"{path} missing X-Data-Time for successful product response")
    if stale == "true" and not reason:
        failed.append(f"{path} marked stale without X-Data-Stale-Reason")
    if status < 400 and max_age_hours is not None and stale != "true":
        dt = parse_data_time(data_time)
        if dt is None:
            failed.append(f"{path} cannot parse X-Data-Time: {data_time!r}")
        else:
            age_hours = (datetime.now(timezone.utc) - dt).total_seconds() / 3600.0
            if age_hours > max_age_hours:
                failed.append(f"{path} old data marked fresh: {data_time} age={age_hours:.1f}h > {max_age_hours}h")
    if partial:
        print(path, "partial errors:", partial)
    return src, data_time, stale

def expected_report_date():
    if ZoneInfo:
        now = datetime.now(ZoneInfo("America/New_York"))
    else:
        now = datetime.now(timezone.utc) - timedelta(hours=4)
    if now.hour < 9:
        now = now - timedelta(days=1)
    while now.weekday() >= 5:
        now = now - timedelta(days=1)
    return now.date().isoformat()

def max_report_date(payload):
    reports = payload.get("reports") if isinstance(payload, dict) else payload
    if not isinstance(reports, list):
        return ""
    return max((str(r.get("date") or "") for r in reports if isinstance(r, dict)), default="")

failed = []

# market
hdr, market = get("/api/market")
src, data_time, stale = check_freshness_headers("/api/market", hdr, 200)
print("market source:", src, "dataTime:", data_time, "stale:", stale)
if not (("us-stocks" in src) or src.startswith("live-") or src.startswith("closed-") or src.startswith("stale-snapshot:")):
    failed.append(f"/api/market source={src!r} expected live-*|closed-*|stale-snapshot:*")
quotes = market.get("quotes") or {}
if not quotes or int(market.get("count") or 0) != len(quotes):
    failed.append(f"/api/market invalid quote coverage: count={market.get('count')} rows={len(quotes)}")
if src.startswith("live-") and len(quotes) < 500:
    failed.append(f"/api/market partial universe presented as live: {len(quotes)} rows")
if (src.startswith("closed-") or src.startswith("stale-snapshot:")) and stale != "true":
    failed.append(f"/api/market degraded source missing stale header: {src!r}")
rklb = (market.get("quotes") or {}).get("RKLB") or {}
print("market RKLB:", rklb)
if len(quotes) >= 500 and (not rklb or float(rklb.get("price") or 0) <= 0):
    failed.append("RKLB missing/invalid in /api/market")

# Seed file may lag live cache; only compare when market is still serving seed snapshot.
if os.path.exists(us_path) and "us-stocks@" in src:
    us = json.loads(open(us_path, encoding="utf-8").read())
    row = next((s for s in us.get("stocks", []) if s.get("sym") == "RKLB"), None)
    if row and rklb:
        if abs(float(rklb["price"]) - float(row["price"])) > 0.011:
            failed.append(f"RKLB market price {rklb['price']} != us-stocks {row['price']}")
elif src.startswith("live-"):
    print("market live mode — skip us-stocks price equality check")
# stocks cn
hdr, cn = get("/api/stocks?market=cn&limit=5&sort=marketcap")
src, data_time, stale = check_freshness_headers("/api/stocks?market=cn", hdr, 200)
print("stocks cn source:", src, "dataTime:", data_time, "stale:", stale, "total:", cn.get("total"))
syms = [s.get("symbol", "") for s in cn.get("stocks") or []]
print("stocks cn sample:", syms)
if not syms or not all(str(s)[:1].isdigit() for s in syms):
    failed.append(f"cn stocks look wrong: {syms}")
if any(s in ("AAPL", "NVDA") for s in syms):
    failed.append("cn endpoint returned US symbols")

# macro / movers. A 503 with stale metadata is an honest, acceptable result
# when the public upstream is unavailable; it must not abort the whole audit.
for path in ("/api/macro", "/api/premarket-movers"):
    status, hdr, body = get_status(path)
    src, data_time, stale = check_freshness_headers(path, hdr, status, require_time=(status == 200), max_age_hours=24 if status == 200 else None)
    print(path, "status:", status, "source:", src, "dataTime:", data_time, "stale:", stale, "keys", list(body)[:5] if isinstance(body, dict) else type(body))
    if status not in (200, 503) or (status == 503 and stale != "true"):
        failed.append(f"{path} expected 200 or stale-block 503, got {status}: {body}")
    if path == "/api/macro" and not (src.startswith("live-macro") or src.startswith("stale-snapshot:") or src == "missing"):
        failed.append(f"{path} source={src!r} expected live-macro|stale-snapshot:*|missing")
    if path == "/api/premarket-movers" and not (src.startswith("live-us-movers") or src.startswith("stale-snapshot:")):
        failed.append(f"{path} source={src!r} expected live-us-movers|stale-snapshot:*")

hdr, panel = get("/api/panel-summary")
_, _, panel_stale = check_freshness_headers("/api/panel-summary", hdr, 200)
psrc = header(hdr, "X-Panel-Path")
print("/api/panel-summary ok path:", psrc, "generated_at:", panel.get("generated_at"))
generated_at = panel.get("generated_at") or ""
if generated_at:
    try:
        dt = datetime.fromisoformat(generated_at.replace("Z", "+00:00"))
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        age_hours = (datetime.now(timezone.utc) - dt.astimezone(timezone.utc)).total_seconds() / 3600.0
        if age_hours > 6 and panel_stale != "true":
            failed.append(f"/api/panel-summary generated_at too old: {generated_at}")
        elif age_hours > 6:
            print("panel summary historical mode — old generation is explicitly stale")
    except Exception as exc:
        failed.append(f"/api/panel-summary generated_at parse failed: {generated_at} ({exc})")

# brand
with urllib.request.urlopen(base + "/api/reports", timeout=30) as r:
    check_freshness_headers("/api/reports", dict(r.headers), r.status)
    text = r.read().decode("utf-8", "ignore")
count = text.count("我不是股神")
print("reports 我不是股神 count:", count)
if count:
    failed.append(f"reports still contain 我不是股神 x{count}")
hdr, reports_body = get("/api/reports?limit=5")
report_src, report_data_time, report_stale = check_freshness_headers("/api/reports?limit=5", hdr, 200)
latest_report = max_report_date(reports_body)
target_report = expected_report_date()
print("reports source:", report_src, "dataTime:", report_data_time, "stale:", report_stale, "latest:", latest_report, "target:", target_report)
if latest_report < target_report:
    if report_stale != "true":
        failed.append(f"/api/reports old snapshot is not marked stale: {latest_report} < {target_report}")
    else:
        print("reports historical mode — latest dated report remains readable and is explicitly stale")

status, _, body = get_status("/api/stocks?market=hk&limit=1")
print("stocks hk contract status:", status)
if status != 400:
    failed.append(f"/api/stocks?market=hk expected 400, got {status}: {body}")

status, _, body = get_status("/api/quote?symbol=AAPL")
print("quote legacy symbol status:", status)
if status != 400:
    failed.append(f"/api/quote?symbol expected 400, got {status}: {body}")

for path in ("/data/market.json", "/data/macro.json", "/data/premarket-movers.json", "/data/us-panel-summary.json"):
    status, hdr, body = get_status(path)
    check_freshness_headers(path, hdr, status)
    print(path, "status:", status, "path:", header(hdr, "X-Data-Path"))
    if status != 200:
        failed.append(f"{path} expected 200, got {status}")

a_api_status, a_api_hdr, a_api_body = get_status("/api/a-market")
a_api_src, a_api_time, a_api_stale = check_freshness_headers("/api/a-market", a_api_hdr, a_api_status, require_time=(a_api_status == 200))
status, hdr, body = get_status("/data/a-market.json")
src, data_time, stale = check_freshness_headers("/data/a-market.json", hdr, status, require_time=(status == 200))
print("a-market static status:", status, "source:", src, "dataTime:", data_time, "stale:", stale, "apiStatus:", a_api_status, "apiStale:", a_api_stale)
if status == 200:
    quotes = body.get("quotes") if isinstance(body, dict) else None
    valid_rows = [q for q in (quotes or {}).values() if isinstance(q, dict) and float(q.get("price") or 0) > 0 and q.get("source") and q.get("dataTime") and not str(q.get("source")).startswith(("local-", "stale-snapshot:"))]
    row_times = [str(q.get("dataTime")) for q in valid_rows]
    if not isinstance(quotes, dict) or int(body.get("count") or 0) != len(quotes) or len(valid_rows) != len(quotes):
        failed.append(f"/data/a-market.json incomplete per-symbol provenance: count={body.get('count') if isinstance(body, dict) else None} rows={len(quotes or {})} provenanced={len(valid_rows)}")
    elif stale == "true" or data_time != min(row_times):
        failed.append(f"/data/a-market.json freshness disagrees with oldest quote: stale={stale!r} header={data_time!r} oldest={min(row_times)!r}")
    if a_api_status != 200 or a_api_stale == "true" or a_api_time != data_time or int(a_api_body.get("count") or 0) != len(quotes or {}):
        failed.append(f"static /data/a-market.json disagrees with /api/a-market: staticTime={data_time!r} apiStatus={a_api_status} apiStale={a_api_stale!r} apiTime={a_api_time!r}")
elif status != 410 or stale != "true":
    failed.append(f"/data/a-market.json expected verified 200 or explicit stale 410, got {status} stale={stale!r}")
elif a_api_status == 200 and a_api_stale != "true":
    failed.append("/data/a-market.json is blocked stale while /api/a-market claims fresh")

status, hdr, body = get_status("/data/reports.json")
src, data_time, stale = check_freshness_headers("/data/reports.json", hdr, status, require_time=(status == 200))
static_latest = max_report_date(body) if status == 200 else ""
print("/data/reports.json status:", status, "source:", src, "dataTime:", data_time, "stale:", stale, "latest:", static_latest)
if status == 200:
    if static_latest < target_report:
        failed.append(f"/data/reports.json latest date too old: {static_latest} < {target_report}")
elif status != 410 or stale != "true":
    failed.append(f"/data/reports.json expected 200 fresh or 410 stale block, got {status} stale={stale!r}")

status, hdr, body = get_status("/data/etf-analyses.json")
src, data_time, stale = check_freshness_headers("/data/etf-analyses.json", hdr, status, require_time=(status == 200))
print("/data/etf-analyses.json status:", status, "source:", src, "dataTime:", data_time, "stale:", stale)
if status == 200:
    failed.append("/data/etf-analyses.json served direct static data; expected stale block until source is refreshed")
elif status != 410 or stale != "true":
    failed.append(f"/data/etf-analyses.json expected 410 stale block, got {status} stale={stale!r}")

status, hdr, body = get_status("/api/etf/sectors?mode=current")
src, data_time, stale = check_freshness_headers("/api/etf/sectors?mode=current", hdr, status, require_time=(status == 200))
current_etf_src = src
print("api etf current status:", status, "source:", src, "dataTime:", data_time, "stale:", stale)
if status == 200:
    if stale == "true" or body.get("dataMode") != "current" or not data_time:
        failed.append(f"/api/etf/sectors?mode=current must be fresh current data: stale={stale!r} dataTime={data_time!r} body={body}")
    current_etfs = body.get("etfs") if isinstance(body, dict) else None
    complete_etfs = [item for item in (current_etfs or []) if isinstance(item, dict) and item.get("sym") and item.get("name") and item.get("sector") and item.get("kind") and float(item.get("aum") or 0) > 0 and isinstance(item.get("expense"), (int, float)) and float(item.get("expense")) >= 0]
    if not isinstance(current_etfs, list) or not current_etfs or len(complete_etfs) != len(current_etfs):
        failed.append(f"/api/etf/sectors?mode=current incomplete decision metadata: rows={len(current_etfs or [])} complete={len(complete_etfs)}")
elif status != 503 or stale != "true":
    failed.append(f"/api/etf/sectors?mode=current expected fresh 200 or explicit stale 503, got {status} stale={stale!r}: {body}")

status, hdr, body = get_status("/api/etf/sectors?mode=historical")
src, data_time, stale = check_freshness_headers("/api/etf/sectors?mode=historical", hdr, status, require_time=True)
print("api etf historical status:", status, "source:", src, "dataTime:", data_time, "stale:", stale)
if status != 200:
    failed.append(f"/api/etf/sectors?mode=historical expected usable dated response 200, got {status}: {body}")
elif stale != "true" or body.get("dataMode") != "historical" or not data_time:
    failed.append(f"/api/etf/sectors?mode=historical must expose dated historical mode: stale={stale!r} dataTime={data_time!r} body={body}")
elif "etf-analyses-current.json" in src or (current_etf_src and src == current_etf_src):
    failed.append(f"/api/etf/sectors?mode=historical reused current source: current={current_etf_src!r} historical={src!r}")

for route in ("/api/etf/SPY/holdings", "/api/etf/compare?syms=SPY,QQQ", "/api/etf/owners?symbol=NVDA"):
    status, hdr, body = get_status(route)
    src, data_time, stale = check_freshness_headers(route, hdr, status, require_time=True)
    mode = body.get("dataMode") if isinstance(body, dict) else None
    print("api etf holdings contract:", route, "status:", status, "source:", src, "dataTime:", data_time, "stale:", stale, "mode:", mode)
    expected_mode = "historical" if stale == "true" else "current"
    if status != 200 or mode != expected_mode or body.get("updated") != data_time:
        failed.append(f"{route} freshness/mode mismatch: status={status} stale={stale!r} dataTime={data_time!r} body={body}")
    if stale == "true" and not body.get("degradedReason"):
        failed.append(f"{route} stale response missing degradedReason")

# GPU rental pricing: dynamic providers fail closed without credentials, while
# the dated public reference remains usable and separate from live/history.
status, hdr, gpu_current = get_status("/api/gpu-prices")
gsrc, gtime, gstale = check_freshness_headers("/api/gpu-prices", hdr, status, require_time=False)
print("gpu prices status:", status, "state:", gpu_current.get("status") if isinstance(gpu_current, dict) else None, "source:", gsrc, "dataTime:", gtime, "stale:", gstale)
if status != 200 or not isinstance(gpu_current, dict):
    failed.append(f"/api/gpu-prices expected structured 200, got {status}: {gpu_current}")
else:
    providers = gpu_current.get("providers") or []
    quotes = gpu_current.get("quotes") or []
    if {p.get("provider") for p in providers if isinstance(p, dict)} != {"runpod", "modal", "lambda", "vast"}:
        failed.append(f"/api/gpu-prices provider coverage invalid: {providers}")
    if int(gpu_current.get("count") or 0) != len(quotes):
        failed.append(f"/api/gpu-prices count mismatch: {gpu_current.get('count')} != {len(quotes)}")
    if gpu_current.get("status") == "unconfigured" and gstale != "true":
        failed.append("/api/gpu-prices unconfigured state must be explicitly stale")

status, hdr, gpu_reference = get_status("/api/gpu-prices/reference")
rsrc, rtime, rstale = check_freshness_headers("/api/gpu-prices/reference", hdr, status, require_time=True)
print("gpu reference status:", status, "source:", rsrc, "dataTime:", rtime, "stale:", rstale)
if status != 200 or not isinstance(gpu_reference, dict):
    failed.append(f"/api/gpu-prices/reference expected structured 200, got {status}: {gpu_reference}")
else:
    items = gpu_reference.get("items") or []
    providers = gpu_reference.get("providers") or []
    vast = next((p for p in providers if isinstance(p, dict) and p.get("provider") == "vast"), {})
    if gpu_reference.get("dataMode") != "historical" or rstale != "true" or int(gpu_reference.get("count") or 0) != len(items) or len(items) < 20:
        failed.append(f"/api/gpu-prices/reference invalid dated historical contract: {gpu_reference}")
    if any(item.get("provider") == "vast" for item in items if isinstance(item, dict)) or vast.get("hasFixedReference") is not False:
        failed.append("/api/gpu-prices/reference must not invent a fixed Vast marketplace price")

status, hdr, gpu_history = get_status("/api/gpu-prices/history?days=30&limit=5")
check_freshness_headers("/api/gpu-prices/history", hdr, status, require_time=False)
if status != 200 or not isinstance(gpu_history, dict) or int(gpu_history.get("count") or 0) != len(gpu_history.get("items") or []):
    failed.append(f"/api/gpu-prices/history invalid response: status={status} body={gpu_history}")

# A-share toolbox: picks + routines (experience + HA smoke)
hdr, picks = get("/api/cn/picks")
psrc, pdata_time, pstale = check_freshness_headers("/api/cn/picks", hdr, 200, max_age_hours=168)
print("cn/picks source:", psrc, "dataTime:", pdata_time, "stale:", pstale, "counts:", picks.get("counts"))
etfs = picks.get("etf_recs") or []
if len(etfs) < 5:
    failed.append(f"cn/picks etf_recs too few: {len(etfs)}")
live_etf = sum(1 for e in etfs if e.get("hasQuote"))
if live_etf < 1:
    failed.append("cn/picks: no live ETF quotes (eastmoney fill failed?)")
if not (picks.get("seed") or picks.get("hot") is not None):
    failed.append("cn/picks missing seed/hot")

hdr, routines = get("/api/cn/routines")
rsrc, rdata_time, rstale = check_freshness_headers("/api/cn/routines", hdr, 200, max_age_hours=168)
print("cn/routines source:", rsrc, "dataTime:", rdata_time, "stale:", rstale, "n:", len(routines.get("routines") or []))
rs = routines.get("routines") or []
if len(rs) < 3:
    failed.append(f"cn/routines too few: {len(rs)}")
prof = routines.get("profile") or {}
if float(prof.get("cash_floor_pct") or 0) < 10:
    failed.append("cn/routines profile cash_floor missing")
# at least one symbol with buy/sell ladder when quote present
ladder_ok = False
for r in rs:
    for s in r.get("symbols") or []:
        if s.get("hasQuote") and (s.get("buy_levels") or s.get("sell_levels")):
            ladder_ok = True
            break
if not ladder_ok:
    failed.append("cn/routines: no buy/sell ladders with live quotes")

if failed:
    print("FAIL:")
    for f in failed:
        print(" -", f)
    sys.exit(1)
print("PASS: verify-local")
PY
