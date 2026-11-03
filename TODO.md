# TODO

The project's to-do list (replaces GitHub issues). Add items at the bottom of a section; delete them when done (git history keeps the record).

## Before the next deploy

- **Add `stair_landing_cost` to the prod DB.** Without it every estimate shows "Estimate not found". Run as `neondb_owner` in the Neon SQL editor on the Production branch:
  `ALTER TABLE estimates ADD COLUMN IF NOT EXISTS stair_landing_cost DOUBLE PRECISION DEFAULT 0;`

## Bugs

- **Keep on-page estimate edits when Save goes to the contact info page.** On the deck and patio estimate pages, editing the estimate and clicking Save without contact info sends the visitor to `/customer`; coming back shows the original estimate because edits only live in the browser until a save. Email and Help are fine. Fix: before redirecting, POST the page's form fields (`save-form` / `patio-save-form`) to a small endpoint that stores them in the session estimate (reuse `applyEstimateForm` / `applyPatioForm`). Same for the "ADD YOUR INFO" / "Edit" links in the party card.

## Features

- **Materials take off: multi-section decks** — `takeoff` returns `ErrMultiSection` for 1028, 1035, 1036, 1037, 1055.
- **Materials take off: rails** — `rails_rules.txt` is a placeholder. Include the stair landing rail (4' per railed side).
- **Materials take off: picture framing.**
- **Wire the take off into the site** — adapter from `DeckEstimate` to `takeoff.Input`, replacing `generateMaterialsList` (Claude) in `materials_handler.go`.
- **Confirm take-off assumptions** — Cortex pack sizes (100/300 LF); 1 box of 50 LedgerLok screws is always enough.
- **Decking Only option on the calculator** — scope option with deck boards only, no framing or footings, alongside Full Service / DIY modes.
- **Marketing landing page for the calculators.** The calculator pages are being slimmed down to feel like an app; their marketing/SEO copy moves to a dedicated page (benefits, how it works, service area, photos, CTA into `/calc`). Decide its URL and whether it replaces `/calc` in the sitemap. Copy removed from `/calc`:
  > Columbia Outdoor's free calculators price out decks, patio covers, pergolas, and lean-tos for homeowners across Clark and Cowlitz County, SW Washington. Enter your project's size and materials and get an itemized cost breakdown instantly — no phone call, no sales visit, and no account required to see your number. When you're ready, save your estimate, talk to a local pro, or use the numbers to plan your budget.
- **Track save drop-off at the login wall.** Add GA4 + New Relic events when a save hits the login redirect (e.g. `estimate_save_login_prompt` / `SaveLoginPrompted`) and when it completes after login. Context: Oct 11, two ad visitors priced $18k and $25k decks and left without saving.

## Security

- **Turnstile CAPTCHA on `/signup` (and `/login`).** The contact form verifies Turnstile; signup doesn't. On Oct 11 a bot created account user_id 15 a second after a rejected contact form post. Reuse the `contact.go` logic as a shared helper. Then delete user_id 15 from prod `user_auth` (check it has no estimates first).

## SEO and marketing

- **www → non-www 301 redirect** at the DNS / Cloud Run level (today only the canonical tag handles it).
- **Descriptive image filenames** — GCS names like `photos-0508.jpg` are a weak Google Images signal. Renaming touches the photos table `uri`, the GCS objects, `scripts/sync_photo_to_gcs.py`, and the image lists in `citiesSEO.go`.
- **"Unknown" project name** — one Camas composite-deck photo (tags `deck`, `rails`) is grouped under the project name `Unknown`, which shows on `/projects`. Give it a real name (and update `static/project_descriptions.yaml`).
- **Google Local Service Ads (Google Guaranteed)** — needs a background check and license/insurance verification (2–4 weeks). Pursue once the Google Ads test results are in.
