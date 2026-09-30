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

### Custom Events (GA4 + New Relic)

Each step of the homeowner funnel is recorded in both tools. GA4 events are
sent from the browser (only on `columbiaoutdoor.com` — on localhost they are
logged to the browser console as `[coTrack]` instead). New Relic events are
recorded server-side.

| # | Step | GA4 event | New Relic event | Fires when |
|---|---|---|---|---|
| 1 | Opens calculator | `calculator_start` | `CalculatorStarted` | Deck or patio calculator page loads |
| 2 | Gets a price | `estimate_view` | `EstimateViewed` | Calculator returns a price |
| 3 | Creates account | `sign_up` | `UserSignedUp` | New account (email or Google) |
| — | Enters contact info | — | `CustomerAdded` | Customer form submitted (NR only — contact info is never sent to GA) |
| 4 | Saves estimate | `estimate_save` | `EstimateCreated` / `PatioEstimateCreated` | New estimate saved for the first time |
| — | Re-saves estimate | — | `EstimateUpdated` / `PatioEstimateUpdated` | Existing estimate re-saved (NR only) |
| 5 | Emails estimate | `estimate_email` | `EstimateEmailed` / `PatioEstimateEmailed` | Estimate email sent |
| 6 | Accepts estimate | `estimate_accept` | `EstimateAccepted` / `PatioEstimateAccepted` | Customer accepts via the shared link |
| 7 | Contact form | `generate_lead` | `ContactFormSubmitted` | Contact form sent |

**GA4 parameters:** `product_type` (`deck` / `patio_cover`) on estimate and
calculator events, plus `value` and `currency` (USD) on `estimate_view`,
`estimate_save`, and `estimate_accept`. `sign_up` has `method`
(`email` / `google`).

**GA4 key events:** `estimate_save`, `estimate_email`, `estimate_accept`,
`generate_lead` (mark in GA Admin → Data display → Events).

**New Relic parameters:** estimate events include `estimate_id`, `total_cost`,
and `contractor_id`; save events add `material` or `patio_type` and `user_id`.
`EstimateViewed` has `product_type`, `total_cost`, and `material` / `patio_type`.
`UserSignedUp` has `user_id` and `method`. `CustomerAdded` is the one event
that carries contact info: `first_name`, `last_name`, `email`, `phone`,
`address`, `city`, `state`, `zip`, plus `entered_by` (`anonymous`,
`homeowner`, `contractor`, `admin`), `user_id`, `product_type`, `total_cost`,
and `estimate_id` (0 if not yet saved). No other event includes contact info.

**How GA4 events are sent:** most actions end in a redirect, so the handler
calls `queueGAEvent()` (`analytics.go`), which stores the event in a short-lived
`ga_events` cookie. The script in `templates/header.gohtml` fires queued events
via `coTrack()` on the next page load and clears the cookie. `estimate_email`
is AJAX, so it calls `coTrack()` directly from the estimate page.

### Useful NRQL Queries

```sql
-- Estimates created over time
SELECT count(*) FROM EstimateCreated TIMESERIES SINCE 30 days ago

-- Average accepted estimate value
SELECT average(total_cost) FROM EstimateAccepted SINCE 30 days ago

-- Full funnel
SELECT count(*) FROM CalculatorStarted, EstimateViewed, UserSignedUp, EstimateCreated, EstimateEmailed, EstimateAccepted SINCE 30 days ago FACET eventType()

-- Latest leads (contact info)
SELECT timestamp, first_name, last_name, email, phone, city, entered_by, product_type, total_cost FROM CustomerAdded SINCE 1 week ago LIMIT 20

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
