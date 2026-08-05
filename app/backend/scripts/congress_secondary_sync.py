#!/usr/bin/env python3
"""Fetch Congressional trades from public secondary aggregators (no eFD / no stockgod).

Priority:
  1) QuiverQuant public HTML embed (`recentTradesData`) — free, no API key
  2) Optional QUIVER_API_KEY / FMP_API_KEY if set
  3) GitHub senate-stock-watcher historical dump (stale fallback)

Outputs JSON consumed by Go RunCapitolTradesSync:
  data/congress-secondary.json

Usage:
  python3 scripts/congress_secondary_sync.py [--out data/congress-secondary.json]
"""
from __future__ import annotations

import argparse
import ast
import json
import os
import re
import sys
import urllib.request
from datetime import datetime
from pathlib import Path

UA = "TradingAgents DataSync contact@tradingagents.app (secondary disclosure aggregator)"


def http_get(url: str, headers: dict | None = None, timeout: int = 60) -> bytes:
    h = {"User-Agent": UA, "Accept": "*/*"}
    if headers:
        h.update(headers)
    req = urllib.request.Request(url, headers=h)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return resp.read()


def normalize_side(raw: str) -> str:
    s = (raw or "").strip().lower()
    if s.startswith("sale") or s.startswith("sell"):
        return "SELL"
    if s.startswith("purchase") or s.startswith("buy"):
        return "BUY"
    if "exchange" in s:
        return "BUY"
    return "BUY"


def normalize_party(raw: str) -> str:
    s = (raw or "").strip().upper()
    if s.startswith("R"):
        return "Republican"
    if s.startswith("D"):
        return "Democratic"
    return "Unknown"


def normalize_date(raw: str) -> str:
    s = str(raw or "").strip()
    if not s:
        return ""
    s = s[:10]
    for fmt in ("%Y-%m-%d", "%m/%d/%Y"):
        try:
            return datetime.strptime(s, fmt).strftime("%Y-%m-%d")
        except ValueError:
            continue
    # try longer timestamps
    try:
        return datetime.strptime(str(raw)[:19], "%Y-%m-%d %H:%M:%S").strftime("%Y-%m-%d")
    except ValueError:
        return s


def clean_ticker(raw: str) -> str:
    t = str(raw or "").strip().upper()
    if t in {"", "-", "--", "N/A", "NONE", "NULL"}:
        return ""
    t = t.replace("$", "")
    return t[:12]


def fetch_quiver_public() -> tuple[list[dict], str]:
    html = http_get("https://www.quiverquant.com/congresstrading/").decode("utf-8", "replace")
    m = re.search(r"let recentTradesData = (\[.*?\]);\s*\n", html, re.S)
    if not m:
        raise RuntimeError("recentTradesData not found on Quiver page")
    raw = m.group(1)
    try:
        rows = ast.literal_eval(raw)
    except Exception as e:
        raise RuntimeError(f"parse recentTradesData failed: {e}") from e

    trades = []
    for r in rows:
        if not isinstance(r, (list, tuple)) or len(r) < 10:
            continue
        ticker = clean_ticker(r[0])
        # Keep equity-like rows; skip empty ticker munis/bonds for Whales UI
        asset_type = str(r[2] or "")
        if not ticker:
            continue
        if asset_type and any(k in asset_type.lower() for k in ("municipal", "corporate bond", "government", "non-public")):
            continue
        chamber = str(r[6] or "")
        title = "Senator" if chamber.lower() == "senate" else "Representative"
        trades.append(
            {
                "politician": str(r[5] or "").strip(),
                "title": title,
                "party": normalize_party(str(r[7] or "")),
                "district": chamber,
                "symbol": ticker,
                "type": normalize_side(str(r[3] or "")),
                "amount": str(r[4] or "").strip(),
                "date": normalize_date(str(r[9] or r[8] or "")),
                "source": "quiver-public",
                "assetName": str(r[1] or ""),
            }
        )
    return trades, "quiverquant.com/congresstrading (public HTML embed)"


def fetch_quiver_api() -> tuple[list[dict], str]:
    key = os.getenv("QUIVER_API_KEY", "").strip()
    if not key:
        raise RuntimeError("QUIVER_API_KEY not set")
    raw = http_get(
        "https://api.quiverquant.com/beta/live/congresstrading",
        headers={"Authorization": f"Bearer {key}", "Accept": "application/json"},
    )
    data = json.loads(raw.decode("utf-8"))
    if not isinstance(data, list):
        raise RuntimeError("unexpected Quiver API payload")
    trades = []
    for r in data:
        ticker = clean_ticker(r.get("Ticker") or r.get("ticker"))
        if not ticker:
            continue
        house = str(r.get("House") or r.get("Chamber") or "")
        title = "Senator" if house.lower() == "senate" else "Representative"
        trades.append(
            {
                "politician": str(r.get("Representative") or r.get("Politician") or "").strip(),
                "title": title,
                "party": normalize_party(str(r.get("Party") or "")),
                "district": house,
                "symbol": ticker,
                "type": normalize_side(str(r.get("Transaction") or "")),
                "amount": str(r.get("Range") or r.get("Amount") or "").strip(),
                "date": normalize_date(str(r.get("TransactionDate") or r.get("Date") or "")),
                "source": "quiver-api",
            }
        )
    return trades, "api.quiverquant.com (QUIVER_API_KEY)"


def fetch_fmp_api() -> tuple[list[dict], str]:
    key = os.getenv("FMP_API_KEY", "").strip()
    if not key:
        raise RuntimeError("FMP_API_KEY not set")
    trades = []
    for chamber, path in (
        ("Senate", f"https://financialmodelingprep.com/stable/senate-latest?page=0&limit=500&apikey={key}"),
        ("House", f"https://financialmodelingprep.com/stable/house-latest?page=0&limit=500&apikey={key}"),
    ):
        try:
            raw = http_get(path)
            data = json.loads(raw.decode("utf-8"))
        except Exception as e:
            print(f"[secondary] FMP {chamber} failed: {e}", file=sys.stderr)
            continue
        if not isinstance(data, list):
            continue
        for r in data:
            ticker = clean_ticker(r.get("symbol") or r.get("ticker"))
            if not ticker:
                continue
            title = "Senator" if chamber == "Senate" else "Representative"
            trades.append(
                {
                    "politician": str(r.get("office") or r.get("representative") or r.get("senator") or "").strip(),
                    "title": title,
                    "party": normalize_party(str(r.get("party") or "")),
                    "district": chamber,
                    "symbol": ticker,
                    "type": normalize_side(str(r.get("type") or r.get("transaction") or "")),
                    "amount": str(r.get("amount") or "").strip(),
                    "date": normalize_date(str(r.get("transactionDate") or r.get("date") or "")),
                    "source": "fmp-api",
                }
            )
    if not trades:
        raise RuntimeError("FMP returned no trades")
    return trades, "financialmodelingprep.com (FMP_API_KEY)"


def fetch_github_senate_historical() -> tuple[list[dict], str]:
    url = "https://raw.githubusercontent.com/timothycarambat/senate-stock-watcher-data/master/aggregate/all_transactions.json"
    raw = http_get(url, timeout=120)
    data = json.loads(raw.decode("utf-8"))
    trades = []
    for r in data:
        ticker = clean_ticker(r.get("ticker"))
        if not ticker:
            continue
        trades.append(
            {
                "politician": str(r.get("senator") or "").strip(),
                "title": "Senator",
                "party": "Unknown",
                "district": "Senate",
                "symbol": ticker,
                "type": normalize_side(str(r.get("type") or "")),
                "amount": str(r.get("amount") or "").strip(),
                "date": normalize_date(str(r.get("transaction_date") or "")),
                "source": "github-senate-stock-watcher",
            }
        )
    # Keep only latest ~2y-ish of historical if huge; dataset ends ~2020
    trades.sort(key=lambda t: t.get("date") or "", reverse=True)
    return trades[:2000], "github.com/timothycarambat/senate-stock-watcher-data (historical)"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default="")
    args = ap.parse_args()

    errors = []
    trades: list[dict] = []
    source = ""

    for name, fn in (
        ("quiver-api", fetch_quiver_api),
        ("fmp-api", fetch_fmp_api),
        ("quiver-public", fetch_quiver_public),
        ("github-historical", fetch_github_senate_historical),
    ):
        try:
            trades, source = fn()
            print(f"[secondary] using {name}: {len(trades)} trades ({source})", file=sys.stderr)
            break
        except Exception as e:
            errors.append(f"{name}: {e}")
            print(f"[secondary] skip {name}: {e}", file=sys.stderr)

    if not trades:
        print("[secondary] all sources failed:\n  " + "\n  ".join(errors), file=sys.stderr)
        return 1

    # Prefer senate rows but keep house too (useful overlay)
    out = {
        "updatedAt": datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "source": source,
        "count": len(trades),
        "senateCount": sum(1 for t in trades if t.get("title") == "Senator"),
        "houseCount": sum(1 for t in trades if t.get("title") == "Representative"),
        "trades": trades,
        "errors": errors,
    }
    out_path = Path(args.out) if args.out else Path(__file__).resolve().parents[1] / "data" / "congress-secondary.json"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(out, ensure_ascii=False, indent=2))
    print(f"[secondary] wrote {out_path} trades={len(trades)} senate={out['senateCount']}", file=sys.stderr)
    print(str(out_path))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
