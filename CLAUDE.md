# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Colout2 is a Go web application for estimating deck/outdoor structure construction costs. Homeowners fill out a multi-step calculator (deck → rails → stairs → customer info), get a cost breakdown, and can save/email the estimate after logging in.

## Build & Run

```bash
# Build and run locally
go build -o colout2 .
./colout2

# Build options via build.sh
./build.sh          # local binary
./build.sh docker   # Docker image (colout2:latest)
./build.sh deploy   # GCP Cloud Run (requires .env)
```

**Required environment variables** (put in `.env` at repo root):
```
SESSION_SECRET=<32+ byte secret>
DATABASE_URL=postgresql://user:pass@host/db
RESEND_API_KEY=re_xxxxx
CLOUDFLARE_SECRET_KEY=0x4...
GOOGLE_OAUTH_SECRET=GOCSPX-...
GOOGLE_OAUTH_CALLBACK_URL=http://localhost:8080/auth/google/callback
```

The app starts an HTTP server on `:8080` (override with `SERVER_ADDR`). Session files are written to `./sessions/` at runtime.

## No Tests

There are no `*_test.go` files. Validation is done manually by running the app and exercising the flows.

## Code Architecture

All Go source files are in the root package. Each file maps to a concern:

| File | Responsibility |
|------|---------------|
| `main.go` | HTTP mux, all route registration, session setup, template funcMap wiring |
| `estimate.go` | Estimate creation/editing, DB save/retrieve, email sending via SendGrid |
| `costs.go` | All pricing formulas — deck sq ft, rails lin ft, stairs, fascia, demo |
| `calculator.go` | Routes `/calc?option=deck|rails|stairs` to correct template |
| `customer.go` | Customer contact info form handler |
| `login.go` | bcrypt auth, Google OAuth2 callback, user registration |
| `session.go` | Gorilla FilesystemStore wrapper |
| `homeowner.go` | Landing page; reads `static/homeowner.yaml` for marketing copy |
| `citiesSEO.go` | Generates city-specific landing pages from a hardcoded city list |
| `contact.go` | Contact form with Cloudflare Turnstile CAPTCHA verification |
| `ui_format.go` | Template helper functions: `formatCost`, `formatDeckDescription`, etc. |

### Request Flow

```
/ → ownerHandler → homeowner.gohtml
/calc?option=deck → calcHandler → calc/deck.gohtml
POST /estimate → estimateHandler → costs.go calculations → DB save → estimate.gohtml
/customer → customerHandler → customer.gohtml
/login, /signup → loginHandler → login.gohtml / signup.gohtml
/auth/google/callback → Google OAuth → session → redirect
/estimate/{id} → load saved estimate from DB → estimate.gohtml
/contact → Cloudflare CAPTCHA verify → SendGrid email
```

### Pricing Data

`static/costs.yaml` holds material prices. `static/homeowner.yaml` holds marketing copy for the landing page. Both are read at startup (or per request) using `gopkg.in/yaml.v3`.

Sales tax is hardcoded at 8.7% (Washington state) in `costs.go`.

### Templates

All templates are in `templates/`. They use Go's `html/template` with a custom `funcMap` registered in `main.go` / `estimate.go`:
- `formatCost(float64)` → `"$13,680.00"`
- `formatDeckDescription()`, `formatRailDescription()`, etc. → human-readable line items for the estimate display
- `currentYear()` → used in footer

Partials: `header.gohtml`, `footer.gohtml` are included by most pages. Calculator sub-templates live in `templates/calc/`.

Template files use `.gohtml` extension (except legacy `.html` files like `session.html`, `error404.html`, `privacy.html`).

### Database

PostgreSQL (Neon serverless). Schema is in `sql/estimates.sql` and `sql/user_auth.sql`. No migration framework — schema is applied manually.

**Database rules:**
- For all DB queries and changes, always use `DATABASE_URL` from `.env` (this is the test/dev database).
- `DATABASE_URL_PROD` in `.env` points to the **production** database. Before running any write (INSERT, UPDATE, DELETE, DROP, ALTER) against `DATABASE_URL_PROD`, always stop and ask: "Are you sure you want to modify the production database?" and wait for explicit confirmation before proceeding.
- Use `psql "$DATABASE_URL" -c "..."` for write operations (the MCP postgres tool is read-only).

Two tables:
- `estimates` — all estimate data plus customer fields and FK to `user_auth`
- `user_auth` — email/password (bcrypt) + Google OAuth users, roles: `homeowner | contractor | admin`

**`estimates` columns** (primary key: `estimate_id`):
`estimate_id`, `description`, `height`, `material`, `rail_material`, `rail_infill`, `stair_width`, `stair_rail_count`, `has_demo`, `has_fascia`, `has_stair_fascia`, `has_stair_tk`, `total_cost`, `first_name`, `last_name`, `address`, `city`, `state`, `zip`, `phone_number`, `email`, `save_date`, `accept_date`, `expiration_date`, `created_at`, `updated_at`, `user_id`, `contractor_id`, `version`, `rail_feet_override`

Related table: `estimate_sections` (columns: `id`, `estimate_id`, `label`, `length`, `width`, `sort_order`, `created_at`)

**`user_auth` columns** (primary key: `id`):
`id`, `email`, `password_hash`, `google_id`, `role`, `created_at`, `updated_at`

### Subdirectories

- `designer/` — standalone Python script (`deck_plan.py`) for visualizing deck layouts; not part of the web app
- `materials/` — standalone Go program (`main.go`) using Gemini/langchain to analyze material rules; not part of the web app
- `sql/` — schema files only; no ORM or migration runner
- `static/` — YAML config files, `robots.txt`, Bing IndexNow key, `t_and_c.txt`
- `sessions/` — runtime session files (gitignored)

## Deployment

Deployed to GCP Cloud Run as a Docker container. `build.sh deploy` builds the image, pushes to Artifact Registry, and updates the Cloud Run service. Production `DATABASE_URL` is set as a Cloud Run environment variable (`DATABASE_URL_PROD`).

## Workflow

### Bug Fixes and Features
1. **One issue per bug or feature** — created on GitHub (`github.com/coloutdoor/colout2/issues`)
2. Read the issue with `gh issue view N --repo coloutdoor/colout2`, discuss before coding
3. Implement the fix or feature
4. `/deploy-test` to build and start the local server
5. Manual verification in the browser + `/smoke-test` to catch regressions
6. Commit using `fixes #N` (bugs) or `closes #N` (features) — auto-closes the issue on push
7. `git push` — issue closes automatically

### Branching
- Commit directly to `master` for bug fixes and single-session features
- Create a branch only for major multi-session features that aren't ready to ship
- `master` should always be deployable

### Slash Commands
- `/deploy-test` — build local binary and start dev server on :8080
- `/smoke-test` — run 9 endpoint checks against a running server (default: localhost:8080, accepts a base URL argument)
