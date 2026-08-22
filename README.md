# Colout2 — Deck Estimator

Go web app for estimating deck construction costs. Homeowners step through a calculator (deck → rails → stairs → customer info), get a cost breakdown, and can save/email the estimate after logging in.

## Requirements

- Go 1.25+
- PostgreSQL (Neon serverless in prod)
- Resend account (email estimates — resend.com)
- Google OAuth2 credentials
- Cloudflare Turnstile site/secret keys (contact form CAPTCHA)

## Configuration

Copy `.env` to the repo root:

```
SESSION_SECRET=<32+ byte secret>
DATABASE_URL=postgresql://user:pass@host/db
RESEND_API_KEY=re_xxxxx
CLOUDFLARE_SECRET_KEY=0x4...
GOOGLE_OAUTH_SECRET=GOCSPX-...
GOOGLE_OAUTH_CALLBACK_URL=http://localhost:8080/auth/google/callback
```

Server listens on `:8080` by default. Override with `SERVER_ADDR=127.0.0.1:8080`.

## Build & Run

```bash
# Run tests
go test ./...

# Local binary
go build -o colout2 .
./colout2

# Or via build script
./build.sh           # run tests then build local binary
./build.sh docker    # Docker image (colout2:latest)
./build.sh deploy    # Push to GCR and deploy to GCP Cloud Run
```

## Tests

Cost calculation logic is covered by table-driven tests in `costs_test.go`. Tests use an inline `Costs` fixture and require no database or file I/O.

## Database

PostgreSQL schema is in `sql/`. No migration framework — apply manually:

```bash
psql $DATABASE_URL -f sql/user_auth.sql
psql $DATABASE_URL -f sql/estimates.sql
```

Two tables: `user_auth` (email/bcrypt + Google OAuth, roles: homeowner/contractor/admin) and `estimates` (all estimate data with FK to user_auth).

## Request Flow

```
/                          Landing page (homeowner.yaml marketing copy)
/calc?option=deck          Step 1: deck dimensions and material
/calc?option=rails         Step 2: rail material and infill
/calc?option=stairs        Step 3: stair width and height
POST /estimate             Calculate costs → save to DB → show estimate
/customer                  Add customer contact info to estimate
/login  /signup            Email/password auth
/auth/google/callback      Google OAuth2
/estimate/{id}             View a previously saved estimate
/contact                   Contact form (Cloudflare Turnstile CAPTCHA)
/deck-builders-{city}      SEO city landing pages
```

## Pricing

Material prices live in `static/costs.yaml`. Sales tax is hardcoded at 8.7% (WA state) in `costs.go`. All cost calculation logic is in `costs.go`; format helpers for displaying line items are in `ui_format.go`.

## Observability (New Relic)

The Go agent (`github.com/newrelic/go-agent/v3`) is wired into the app. Set the license key in `.env`:

```
NEW_RELIC_LICENSE_KEY=<Ingest - License key from one.newrelic.com/api-keys>
```

The app name is set automatically: `colout2-test` when `SERVER_ADDR` contains `localhost`, `colout2` in production. Both appear as separate apps in the NR UI.

### Custom Events

Four business events are recorded in addition to the standard transaction/error telemetry:

| Event | Fires when |
|---|---|
| `EstimateCreated` | New estimate saved to DB for the first time |
| `EstimateUpdated` | Existing estimate re-saved |
| `EstimateEmailed` | Estimate emailed to a homeowner |
| `EstimateAccepted` | Homeowner accepts an estimate via the token link |

All events include `estimate_id`, `total_cost`, and `contractor_id`. Save events also include `material` and `user_id`. `EstimateEmailed` includes `to_email`.

### Useful NRQL Queries

```sql
-- Estimates created over time
SELECT count(*) FROM EstimateCreated TIMESERIES SINCE 30 days ago

-- Average accepted estimate value
SELECT average(total_cost) FROM EstimateAccepted SINCE 30 days ago

-- Email-to-accept conversion funnel
SELECT count(*) FROM EstimateEmailed, EstimateAccepted SINCE 30 days ago

-- Activity by contractor
SELECT count(*) FROM EstimateCreated FACET contractor_id SINCE 30 days ago

-- High-value estimates saved today
SELECT estimate_id, total_cost, material FROM EstimateCreated
WHERE total_cost > 20000 SINCE 1 day ago
```

## Deployment

GCP Cloud Run. `build.sh deploy` tags the local Docker image, pushes to Artifact Registry (`us.gcr.io/columbia-outdoor/colout2`), and updates the Cloud Run service. Production env vars (`DATABASE_URL`, `RESEND_API_KEY`, etc.) are set directly on the Cloud Run service.
