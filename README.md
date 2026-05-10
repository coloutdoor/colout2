# Colout2 — Deck Estimator

Go web app for estimating deck construction costs. Homeowners step through a calculator (deck → rails → stairs → customer info), get a cost breakdown, and can save/email the estimate after logging in.

## Requirements

- Go 1.25+
- PostgreSQL (Neon serverless in prod)
- SendGrid account (email estimates)
- Google OAuth2 credentials
- Cloudflare Turnstile site/secret keys (contact form CAPTCHA)

## Configuration

Copy `.env` to the repo root:

```
SESSION_SECRET=<32+ byte secret>
DATABASE_URL=postgresql://user:pass@host/db
SENDGRID_API_KEY=SG.xxxxx
CLOUDFLARE_SECRET_KEY=0x4...
GOOGLE_OAUTH_SECRET=GOCSPX-...
GOOGLE_OAUTH_CALLBACK_URL=http://localhost:8080/auth/google/callback
```

Server listens on `:8080` by default. Override with `SERVER_ADDR=127.0.0.1:8080`.

## Build & Run

```bash
# Local binary
go build -o colout2 .
./colout2

# Or via build script
./build.sh           # local binary
./build.sh docker    # Docker image (colout2:latest)
./build.sh deploy    # Push to GCR and deploy to GCP Cloud Run
```

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

## Deployment

GCP Cloud Run. `build.sh deploy` tags the local Docker image, pushes to Artifact Registry (`us.gcr.io/columbia-outdoor/colout2`), and updates the Cloud Run service. Production env vars (`DATABASE_URL`, `SENDGRID_API_KEY`, etc.) are set directly on the Cloud Run service.
