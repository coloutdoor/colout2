#!/usr/bin/env python3
"""
download_images.py

Downloads every photo belonging to a given Project (as tagged in
static/photos.yaml) to a local folder, e.g.:

    scripts/venv/bin/python scripts/download_images.py Rush

Photos land in ~/Pictures/<Project>/ (created if needed), named after their
original filename (falling back to the GCS blob name if that's missing).
Already-downloaded files are skipped, so re-running is safe.
"""

import argparse
import re
import sys
import urllib.request
from pathlib import Path

try:
    import yaml
except ImportError:
    sys.exit("Missing dependency: pip install pyyaml")


def load_entries(photos_yaml: Path):
    if not photos_yaml.exists():
        sys.exit(f"Not found: {photos_yaml}")
    with open(photos_yaml) as f:
        return yaml.safe_load(f) or []


def local_filename(entry: dict) -> str:
    source_path = entry.get("source_path", "")
    name = Path(source_path).name if source_path else Path(entry["uri"]).name
    return re.sub(r'[/\\:*?"<>|]', "_", name)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("project", help='Project name, e.g. "Rush" (matched case-insensitively)')
    parser.add_argument("--photos-yaml", default="static/photos.yaml", help="Path to photos.yaml (default: static/photos.yaml)")
    parser.add_argument("--output-dir", default="~/Pictures", help="Parent folder to download into (default: ~/Pictures)")
    args = parser.parse_args()

    entries = load_entries(Path(args.photos_yaml))
    matches = [e for e in entries if e.get("project", "").lower() == args.project.lower()]

    if not matches:
        known = sorted({e["project"] for e in entries if e.get("project")})
        sys.exit(f'No photos found for project "{args.project}". Known projects: {", ".join(known)}')

    dest_dir = Path(args.output_dir).expanduser() / args.project
    dest_dir.mkdir(parents=True, exist_ok=True)

    downloaded = 0
    skipped = 0
    for entry in matches:
        uri = entry["uri"]
        dest = dest_dir / local_filename(entry)
        if dest.exists():
            print(f"[SKIP exists]  {dest.name}")
            skipped += 1
            continue
        urllib.request.urlretrieve(uri, dest)
        print(f"Downloaded  {dest.name}")
        downloaded += 1

    print(f"\nDone — {downloaded} downloaded, {skipped} already present -> {dest_dir}")


if __name__ == "__main__":
    main()
