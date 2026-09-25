#!/usr/bin/env python3
"""
sync_photo_to_gcs.py

Walks a local folder tree recursively, uploads each new image to GCS
(skipping ones already in static/photos.yaml by MD5), and appends an entry
for each newly uploaded photo directly to static/photos.yaml with
reviewed: false, ready for review at /admin/photos.

SETUP
-----
1. Download photos from Google Drive (any folder structure is fine).
2. pip install google-cloud-storage pyyaml pillow pillow-heif
3. Authenticate: gcloud auth application-default login
4. Run (from the repo root):
       scripts/venv/bin/python scripts/sync_photo_to_gcs.py ~/columbia-photos

NOTES
-----
- Walks ALL subdirectories recursively — folder structure does not matter.
- iPhone .heic/.heif photos are supported — they're converted to JPEG before
  upload (most browsers can't display raw HEIC), so they always land in the
  bucket and static/photos.yaml as a photos-NNNN.jpg.
- If photos sit directly in the given folder (no subfolder), the folder's
  own name is used as the project guess (e.g. ~/Desktop/Rehfeldt -> "Rehfeldt").
  Otherwise city is guessed from the top-level subfolder name.
- Category and tags are best-guessed from the full path + filename keywords.
- Duplicate detection is by MD5 against everything already in static/photos.yaml
  — re-running on the same folder is safe, duplicates are skipped entirely
  (never uploaded, never re-merged into an existing reviewed entry).
- GCS filenames are sequential slugs (photos-0001.jpg); the next number is
  computed from the actual bucket contents and static/photos.yaml, so it
  can't collide with an already-used slug.
- New entries are appended as-is (existing entries are never rewritten), with
  reviewed: false — review them at /admin/photos.
"""

import argparse
import hashlib
import io
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    sys.exit("Missing dependency: pip install pyyaml")

try:
    from google.cloud import storage
except ImportError:
    sys.exit("Missing dependency: pip install google-cloud-storage")

try:
    import pillow_heif
    from PIL import Image
except ImportError:
    sys.exit("Missing dependency: pip install pillow pillow-heif")


IMAGE_EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp", ".heic", ".heif"}
HEIC_EXTENSIONS = {".heic", ".heif"}
GCS_NAME_RE = re.compile(r"photos-(\d+)\.")

CATEGORY_KEYWORDS = [
    ("stair",       "stairs"),
    ("step",        "stairs"),
    ("rail",        "rails"),
    ("patio",       "patio"),
    ("cover",       "cover"),
    ("pergola",     "cover"),
    ("lean-to",     "cover"),
    ("lean to",     "cover"),
    ("fence",       "fence"),
    ("gate",        "fence"),
    ("farmstand",   "farm"),
    ("farm stand",  "farm"),
    ("she shed",    "farm"),
    ("sheshed",     "farm"),
    ("shed",        "farm"),
    ("garden",      "farm"),
    ("coop",        "farm"),
    ("deck",        "deck"),
]

TAG_KEYWORDS = {
    "timbertech":     "timbertech",
    "trex":           "trex",
    "azek":           "azek",
    "composite":      "composite",
    "cedar":          "cedar",
    "timberframe":    "timberframe",
    "timber frame":   "timberframe",
    "aluminum":       "aluminum-rail",
    "glass":          "glass-infill",
    "hogwire":        "hogwire-rail",
    "cable":          "cable-rail",
    "picture frame":  "picture-frame-decking",
    "45 degree":      "diagonal-decking",
    "stamped":        "stamped-concrete",
    "flagstone":      "flagstone",
    "vaulted":        "vaulted-ceiling",
    "skylight":       "skylights",
    "lighting":       "lighting",
    "dryspace":       "dry-under-deck",
    "dry space":      "dry-under-deck",
    "dry under":      "dry-under-deck",
    "under deck":     "dry-under-deck",
    "framing":        "framing",
    "frame":          "framing",
    "in progress":    "framing",
    "finish":         "finish",
    "finished":       "finish",
    "complete":       "finish",
}


class IndentDumper(yaml.SafeDumper):
    """Indents block sequences under their parent key, matching the style
    already used in static/photos.yaml (PyYAML's default leaves them
    unindented)."""

    def increase_indent(self, flow=False, indentless=False):
        return super().increase_indent(flow, False)


def heic_to_jpeg_bytes(path: Path) -> bytes:
    heif_file = pillow_heif.open_heif(path, convert_hdr_to_8bit=True)
    img = Image.frombytes(heif_file.mode, heif_file.size, heif_file.data, "raw")
    buf = io.BytesIO()
    img.save(buf, format="JPEG", quality=90)
    return buf.getvalue()


def file_md5(path: Path) -> str:
    h = hashlib.md5()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def guess_category(text: str) -> str:
    lower = text.lower()
    for keyword, category in CATEGORY_KEYWORDS:
        if keyword in lower:
            return category
    return "other"


def guess_tags(text: str) -> list:
    lower = text.lower()
    found = []
    for keyword, tag in TAG_KEYWORDS.items():
        if keyword in lower and tag not in found:
            found.append(tag)
    return found


def humanize(stem: str) -> str:
    text = re.sub(r"[_\-]+", " ", stem)
    text = re.sub(r"\s+", " ", text).strip()
    return text[0].upper() + text[1:] if text else text


def load_known_hashes(photos_yaml: Path):
    if not photos_yaml.exists():
        return set()
    with open(photos_yaml) as f:
        entries = yaml.safe_load(f) or []
    return {e["md5"] for e in entries if "md5" in e}, entries


def next_counter(bucket, entries) -> int:
    """Highest photos-NNNN number seen in the bucket or in static/photos.yaml, plus one."""
    highest = 0
    for blob in bucket.list_blobs(prefix="photos-"):
        m = GCS_NAME_RE.search(blob.name)
        if m:
            highest = max(highest, int(m.group(1)))
    for e in entries:
        m = GCS_NAME_RE.search(e.get("uri", ""))
        if m:
            highest = max(highest, int(m.group(1)))
    return highest + 1


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("root", help="Local folder containing photos (any structure)")
    parser.add_argument("--bucket", default="columbiaoutdoor-images", help="GCS bucket name (no gs:// prefix)")
    parser.add_argument("--photos-yaml", default="static/photos.yaml", help="Path to photos.yaml (default: static/photos.yaml)")
    args = parser.parse_args()

    root = Path(args.root).expanduser()
    if not root.is_dir():
        sys.exit(f"Not a directory: {root}")

    photos_yaml = Path(args.photos_yaml)
    known_hashes, entries = load_known_hashes(photos_yaml)
    print(f"Existing entries in {photos_yaml}: {len(entries)}  Known hashes: {len(known_hashes)}")

    client = storage.Client()
    bucket_obj = client.bucket(args.bucket)
    counter = next_counter(bucket_obj, entries)

    all_images = sorted(p for p in root.rglob("*") if p.suffix.lower() in IMAGE_EXTENSIONS)
    print(f"Found {len(all_images)} image files under {root}\n")

    new_entries = []
    skipped_dupes = 0

    for photo_path in all_images:
        md5 = file_md5(photo_path)
        if md5 in known_hashes:
            print(f"[SKIP dupe]  {photo_path.relative_to(root)}")
            skipped_dupes += 1
            continue

        rel = photo_path.relative_to(root)
        parts = rel.parts
        top = parts[0] if len(parts) > 1 else None

        if top and top.lower() == "projects":
            city_guess = "unknown"
            project_name = re.sub(r"\d+$", "", photo_path.stem).strip()
        elif top:
            city_guess = top
            project_name = ""
        else:
            # Photos sit directly in the given folder — use the folder's own
            # name as the project guess (e.g. ~/Desktop/Rehfeldt -> "Rehfeldt").
            city_guess = "unknown"
            project_name = root.name

        full_text = str(rel) + " " + photo_path.stem
        category = guess_category(full_text)
        tags = guess_tags(full_text)
        description = humanize(photo_path.stem)

        is_heic = photo_path.suffix.lower() in HEIC_EXTENSIONS
        ext = ".jpg" if is_heic else photo_path.suffix.lower()
        gcs_name = f"photos-{counter:04d}{ext}"
        uri = f"https://storage.googleapis.com/{args.bucket}/{gcs_name}"

        blob = bucket_obj.blob(gcs_name)
        if is_heic:
            blob.upload_from_string(heic_to_jpeg_bytes(photo_path), content_type="image/jpeg")
        else:
            blob.upload_from_filename(str(photo_path))
        print(f"Uploaded  {rel}  ->  gs://{args.bucket}/{gcs_name}" + ("  (converted from HEIC)" if is_heic else ""))

        entry = {
            "uri":         uri,
            "md5":         md5,
            "source_path": str(rel),
            "city":        city_guess,
            "category":    category,
            "description": description,
        }
        if tags:
            entry["tags"] = tags
        entry["featured"] = False
        entry["reviewed"] = False
        if project_name:
            entry["project"] = project_name

        new_entries.append(entry)
        known_hashes.add(md5)
        counter += 1

    if new_entries:
        text = yaml.dump(new_entries, Dumper=IndentDumper, sort_keys=False, allow_unicode=True, default_flow_style=False)
        with open(photos_yaml, "a") as f:
            f.write(text)

    print(f"\nDone — {len(new_entries)} new, {skipped_dupes} dupes skipped -> {photos_yaml}")
    if new_entries:
        print("Review them at /admin/photos.")


if __name__ == "__main__":
    main()
