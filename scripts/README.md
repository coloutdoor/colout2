# scripts/

## sync_photo_to_gcs.py

Walks a local folder tree recursively, uploads each new image to the `columbiaoutdoor-images` GCS bucket, and appends an entry directly to `static/photos.yaml` for each one — with `reviewed: false`, ready to review at `/admin/photos`. Duplicate detection is by MD5 against everything already in `static/photos.yaml` — re-running on the same folder is safe, duplicates are skipped entirely (never re-uploaded).

### Setup

```bash
# Create virtual environment (one-time)
python3 -m venv scripts/venv
scripts/venv/bin/pip install google-cloud-storage pyyaml

# Authenticate with GCP (one-time per machine)
gcloud auth application-default login
```

### Folder structure

Any structure works — the script walks all subdirectories recursively.

- If photos sit **directly** in the folder you point it at, the folder's own name is used as the project guess (e.g. `~/Desktop/Rehfeldt` → project `Rehfeldt`).
- If photos sit in **subfolders**, each subfolder name is used as the city guess (the admin reviewer corrects it if needed).

```
~/columbia-photos/
    Woodland/
        timbertech deck with rails.jpeg
    Some Project Name/
        cover1.jpg
        cover2.jpg
```

### Usage

Run from the repo root:

```bash
scripts/venv/bin/python scripts/sync_photo_to_gcs.py ~/Desktop/SomeProject
```

That's it — new photos are uploaded to GCS and appended to `static/photos.yaml`. Re-running on the same folder is safe.

### After running

Go to `/admin/photos` — the new photos show up under "To Review." Correct city/category/description, add tags, mark featured shots, and approve.
