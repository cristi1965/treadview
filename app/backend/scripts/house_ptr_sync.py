#!/usr/bin/env python3
"""Download House Clerk FD index + parse recent PTR PDFs into congress-trades JSON.

No stockgod dependency. Requires: pip install -r scripts/requirements.txt
Usage:
  python3 scripts/house_ptr_sync.py [--years 2025,2026] [--limit 80] [--out data/congress-live.json]
"""
from __future__ import annotations

import argparse
import io
import json
import re
import sys
import time
import zipfile
from datetime import datetime, timedelta
from pathlib import Path
from urllib.request import Request, urlopen
from xml.etree import ElementTree as ET

UA = "TradingAgents DataSync contact@tradingagents.app"
BASE = "https://disclosures-clerk.house.gov"
TICKER_RE = re.compile(r"\(([A-Z][A-Z0-9./\-]{0,10})\)\s*\[(?:ST|OT|PS|OP|CS)\]")
AMOUNT_RE = re.compile(r"\$[\d,]+(?:\s*-\s*\$[\d,]+)?")
DATE_RE = re.compile(r"(\d{2}/\d{2}/\d{4})")


def http_get(url: str, timeout: int = 45) -> bytes:
    req = Request(url, headers={"User-Agent": UA})
    with urlopen(req, timeout=timeout) as resp:
        return resp.read()


def fetch_ptr_index(year: int) -> list[dict]:
    url = f"{BASE}/public_disc/financial-pdfs/{year}FD.zip"
    raw = http_get(url)
    with zipfile.ZipFile(io.BytesIO(raw)) as zf:
        name = next(n for n in zf.namelist() if n.lower().endswith(".xml"))
        root = ET.fromstring(zf.read(name))
    out = []
    for member in root.findall("Member"):
        def t(tag: str) -> str:
            return (member.findtext(tag) or "").strip()

        if t("FilingType").upper() != "P":
            continue
        out.append(
            {
                "last": t("Last"),
                "first": t("First"),
                "state_dst": t("StateDst"),
                "year": int(t("Year") or year),
                "filing_date": t("FilingDate"),
                "doc_id": t("DocID"),
            }
        )
    return out


def parse_filing_date(s: str):
    for fmt in ("%m/%d/%Y", "%Y-%m-%d"):
        try:
            return datetime.strptime(s, fmt)
        except ValueError:
            continue
    return None


def extract_pdf_text(pdf_bytes: bytes) -> str:
    from pypdf import PdfReader

    reader = PdfReader(io.BytesIO(pdf_bytes))
    if reader.is_encrypted:
        try:
            reader.decrypt("")
        except Exception:
            pass
    return "\n".join((page.extract_text() or "") for page in reader.pages)


def normalize_amount(s: str) -> str:
    return re.sub(r"\s+", " ", s).strip()


def parse_ptr_text(
    text: str,
    politician: str,
    district: str,
    filing_date: str,
    source_url: str,
    filing_id: str,
) -> list[dict]:
    trades: list[dict] = []
    # Collapse whitespace for more stable matching
    flat = re.sub(r"[ \t]+", " ", text)
    flat = flat.replace("\r", "")

    for m in TICKER_RE.finditer(flat):
        ticker = m.group(1).replace("/", ".")
        if ticker in {"ST", "OT", "PS", "OP", "CS"}:
            continue
        # Window around match for type/date/amount
        start = max(0, m.start() - 80)
        end = min(len(flat), m.end() + 220)
        window = flat[start:end]

        side = "BUY"
        if re.search(r"\bS(?:\s*\((?:partial|full)\))?\b", window) or "Sale" in window:
            side = "SELL"
        elif re.search(r"\bP(?:\s*\((?:partial|full)\))?\b", window) or "Purchase" in window:
            side = "BUY"

        amounts = AMOUNT_RE.findall(window)
        amount = normalize_amount(amounts[0]) if amounts else "$1,001 - $15,000"

        dates = DATE_RE.findall(window)
        trade_date = ""
        if dates:
            try:
                trade_date = datetime.strptime(dates[0], "%m/%d/%Y").strftime("%Y-%m-%d")
            except ValueError:
                trade_date = dates[0]
        if not trade_date and filing_date:
            fd = parse_filing_date(filing_date)
            trade_date = fd.strftime("%Y-%m-%d") if fd else filing_date

        trades.append(
            {
                "politician": politician,
                "title": "Representative",
                "party": "",
                "district": district,
                "symbol": ticker,
                "type": side,
                "amount": amount,
                "date": trade_date,
                "source": "house-ptr",
                "filingDate": filing_date or "unknown",
                "sourceURL": source_url,
                "filingId": filing_id,
            }
        )

    # Dedup identical rows from repeated description blocks
    seen = set()
    uniq = []
    for t in trades:
        key = (t["politician"], t["symbol"], t["type"], t["date"], t["amount"])
        if key in seen:
            continue
        seen.add(key)
        uniq.append(t)
    return uniq


def normalize_name(first: str, last: str) -> str:
    first = re.sub(r"\s+", " ", first).strip()
    last = re.sub(r"\s+", " ", last).strip()
    return f"{first} {last}".strip()


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--years", default="2025,2026")
    ap.add_argument("--limit", type=int, default=120)
    ap.add_argument("--days", type=int, default=180)
    ap.add_argument("--out", default="")
    args = ap.parse_args()

    years = [int(y.strip()) for y in args.years.split(",") if y.strip()]
    cutoff = datetime.utcnow() - timedelta(days=args.days)

    filings: list[dict] = []
    for y in years:
        try:
            rows = fetch_ptr_index(y)
            print(f"[house-ptr] year={y} ptr_filings={len(rows)}", file=sys.stderr)
            filings.extend(rows)
            time.sleep(0.2)
        except Exception as e:
            print(f"[house-ptr] index {y} failed: {e}", file=sys.stderr)

    def sort_key(r: dict):
        return parse_filing_date(r.get("filing_date") or "") or datetime(1970, 1, 1)

    filings.sort(key=sort_key, reverse=True)
    selected = []
    for r in filings:
        d = parse_filing_date(r.get("filing_date") or "")
        if d and d < cutoff:
            continue
        selected.append(r)
        if len(selected) >= args.limit:
            break

    trades: list[dict] = []
    for i, r in enumerate(selected):
        year = r["year"]
        doc = r["doc_id"]
        url = f"{BASE}/public_disc/ptr-pdfs/{year}/{doc}.pdf"
        politician = normalize_name(r["first"], r["last"])
        district = r.get("state_dst") or ""
        try:
            pdf = http_get(url)
            text = extract_pdf_text(pdf)
            parsed = parse_ptr_text(
                text,
                politician,
                district,
                r.get("filing_date") or "",
                url,
                doc,
            )
            trades.extend(parsed)
            print(f"[house-ptr] {i+1}/{len(selected)} {politician} -> {len(parsed)} trades", file=sys.stderr)
        except Exception as e:
            print(f"[house-ptr] skip {politician} {doc}: {e}", file=sys.stderr)
        time.sleep(0.12)

    out = {
        "updatedAt": datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "source": "disclosures-clerk.house.gov",
        "filingsParsed": len(selected),
        "count": len(trades),
        "trades": trades,
    }

    out_path = Path(args.out) if args.out else Path(__file__).resolve().parents[1] / "data" / "congress-live.json"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(out, ensure_ascii=False, indent=2))
    print(f"[house-ptr] wrote {out_path} trades={len(trades)}", file=sys.stderr)
    print(str(out_path))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
