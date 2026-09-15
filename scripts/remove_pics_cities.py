#!/usr/bin/env python3
"""Remove photos_new.yaml entries where city starts with 'Pics' (case-insensitive)."""

import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    sys.exit("Missing dependency: pip install pyyaml")

out_path = Path("photos_new.yaml")
if not out_path.exists():
    sys.exit("photos_new.yaml not found — run from repo root")

with open(out_path) as f:
    entries = yaml.safe_load(f) or []

before = len(entries)
kept = [e for e in entries if not e.get("city", "").lower().startswith("pics")]
removed = before - len(kept)

with open(out_path, "w") as f:
    yaml.dump(kept, f, sort_keys=False, allow_unicode=True)

print(f"Removed {removed} entries, {len(kept)} remaining -> {out_path}")
