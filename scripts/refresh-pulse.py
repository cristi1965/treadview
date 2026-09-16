#!/usr/bin/env python3
"""Refresh pulse-companies.json from stockgod.xyz RSC payload.

Usage:
  python3 scripts/refresh-pulse.py
"""
from __future__ import annotations

import json
import re
import sys
import urllib.request
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT_PATHS = [
    ROOT / "app/frontend/public/data/pulse-companies.json",
    ROOT / "app/frontend/dist/data/pulse-companies.json",
    ROOT / "app/backend/data/pulse-companies.json",
]


def fetch_rsc() -> str:
    req = urllib.request.Request(
        "https://stockgod.xyz/?_rsc=1",
        headers={"User-Agent": "Mozilla/5.0", "RSC": "1"},
    )
    with urllib.request.urlopen(req, timeout=120) as resp:
        return resp.read().decode("utf-8", "ignore")


def parse_companies(text: str) -> list[dict]:
    companies: list[dict] = []
    idx = 0
    while True:
        i = text.find('{"id":', idx)
        if i < 0:
            break
        depth = 0
        j = i
        in_str = False
        esc = False
        while j < len(text):
            ch = text[j]
            if in_str:
                if esc:
                    esc = False
                elif ch == "\\":
                    esc = True
                elif ch == '"':
                    in_str = False
            else:
                if ch == '"':
                    in_str = True
                elif ch == "{":
                    depth += 1
                elif ch == "}":
                    depth -= 1
                    if depth == 0:
                        j += 1
                        break
            j += 1
        raw = text[i:j]
        idx = i + 5
        if '"ticker":' not in raw or '"heat":' not in raw:
            continue
        try:
            d = json.loads(raw)
        except json.JSONDecodeError:
            continue
        if isinstance(d.get("ticker"), str) and isinstance(d.get("heat"), (int, float)):
            companies.append(d)
    # dedupe
    seen: dict[tuple, dict] = {}
    for c in companies:
        seen[(c["ticker"], c.get("region"))] = c
    return list(seen.values())


def slim(c: dict) -> dict:
    return {
        "ticker": c["ticker"],
        "name": c.get("name") or c["ticker"],
        "layer": c.get("layer") or "L6",
        "segment": c.get("segment"),
        "region": c.get("region") or "US",
        "marketCapB": c.get("marketCapB") or 0,
        "heat": c.get("heat") or 0,
        "industries": c.get("industries") or [],
        "moat": c.get("moat"),
        "pos52": c.get("pos52"),
        "pct": c.get("pct"),
        "livePrice": c.get("livePrice"),
        "liveMcapYi": c.get("liveMcapYi"),
        "valuationPct": c.get("valuationPct"),
        "momentum20d": c.get("momentum20d"),
        "rsi": c.get("rsi"),
        "sentiment": c.get("sentiment"),
        "dataSource": c.get("dataSource"),
        "liveBar": c.get("liveBar"),
    }


def main() -> int:
    print("fetching RSC…")
    text = fetch_rsc()
    companies = parse_companies(text)
    print("parsed", len(companies), Counter(c.get("region") for c in companies))
    if len(companies) < 1000:
        print("ERROR: too few companies", file=sys.stderr)
        return 1
    slimmed = [slim(c) for c in companies]
    nv = next((c for c in slimmed if c["ticker"] == "NVDA"), None)
    payload = {
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "source": "stockgod-rsc-capture",
        "source_live_bar": (nv or {}).get("liveBar"),
        "count": len(slimmed),
        "companies": slimmed,
    }
    raw = json.dumps(payload, ensure_ascii=False, separators=(",", ":"))
    for path in OUT_PATHS:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(raw)
        print("wrote", path)
    us_ai = sum(1 for c in slimmed if c.get("region") == "US" and "AI" in c.get("industries", []))
    print("US AI", us_ai, "NVDA", nv)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
