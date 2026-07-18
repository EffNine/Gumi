#!/usr/bin/env python3
"""Fetch the OpenAI HumanEval dataset and write benchmark/suites/coding/humaneval.jsonl.

Usage:
    python3 scripts/fetch_humaneval.py

Source: https://github.com/openai/human-eval (canonical JSONL, gzip-compressed)
Output: benchmark/suites/coding/humaneval.jsonl
"""

import gzip
import json
import os
import sys
import urllib.request
from pathlib import Path

URL = "https://github.com/openai/human-eval/raw/master/data/HumanEval.jsonl.gz"


def repo_root() -> Path:
    script = Path(__file__).resolve()
    return script.parent.parent


def download(url: str) -> bytes:
    print(f"downloading {url} ...")
    with urllib.request.urlopen(url, timeout=120) as resp:
        return resp.read()


def main() -> int:
    out_dir = repo_root() / "benchmark" / "suites" / "coding"
    out_dir.mkdir(parents=True, exist_ok=True)
    out_path = out_dir / "humaneval.jsonl"

    raw = download(URL)
    decoded = gzip.decompress(raw).decode("utf-8")

    records = []
    for line in decoded.strip().splitlines():
        row = json.loads(line)
        # Keep only the fields the benchmark harness needs.
        records.append(
            {
                "task_id": row["task_id"],
                "prompt": row["prompt"],
                "canonical_solution": row["canonical_solution"],
                "test": row["test"],
                "entry_point": row["entry_point"],
            }
        )

    with open(out_path, "w", encoding="utf-8") as f:
        for rec in records:
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")

    print(f"wrote {len(records)} problems to {out_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
