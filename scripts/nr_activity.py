#!/usr/bin/env python3
"""Calculator activity report from New Relic.

Shows the funnel events (calculator starts, estimates, emails, saves,
customer info, signups) and, for every estimate, the page-by-page trail of
that visit. Scanner/bot noise (404s, robots.txt, static files) is hidden and
summarized instead.

Reads NEW_RELIC_USER_KEY (a read-only User API key) from the environment or
from the repo's .env file. Standard library only.

Usage:
  scripts/nr_activity.py                 # today, production (colout2)
  scripts/nr_activity.py --since "2 days ago"
  scripts/nr_activity.py --since yesterday --until today
  scripts/nr_activity.py --since "2026-10-12 09:00 -0700" --until "2026-10-12 10:00 -0700"
  scripts/nr_activity.py --app colout2-test   # local dev traffic
  scripts/nr_activity.py --no-trails     # events only
"""
import argparse
import datetime as dt
import json
import os
import pathlib
import sys
import urllib.request

ACCOUNT_ID = 1171346  # ColumbiaOutdoor.com
API = "https://api.newrelic.com/graphql"
FUNNEL_EVENTS = [
    "CalculatorStarted", "EstimateViewed", "EstimateEmailed", "PatioEstimateEmailed",
    "CustomerAdded", "EstimateCreated", "PatioEstimateCreated",
    "EstimateAccepted", "PatioEstimateAccepted", "UserSignedUp", "ContactFormSubmitted",
]
MATERIALS = {
    "outdoorWood": "Outdoor Wood", "cedar": "Cedar", "timberTechPrime": "Composite (25yr)",
    "timberTechProReserve": "Composite (30yr)", "timberTechProLegacy": "Pro Legacy",
    "timberTechAzek": "PVC (50yr)",
}
NOISE_URIS = ("/robots.txt", "/sitemap.xml", "/.well-known/traffic-advice")


def load_key():
    key = os.environ.get("NEW_RELIC_USER_KEY")
    if key:
        return key
    env = pathlib.Path(__file__).resolve().parent.parent / ".env"
    if env.exists():
        for line in env.read_text().splitlines():
            if line.startswith("NEW_RELIC_USER_KEY="):
                return line.split("=", 1)[1].strip().strip("'\"")
    sys.exit("NEW_RELIC_USER_KEY not found in the environment or .env")


def nrql(key, query):
    gql = "{ actor { account(id: %d) { nrql(query: %s, timeout: 60) { results } } } }" % (
        ACCOUNT_ID, json.dumps(query))
    req = urllib.request.Request(API, data=json.dumps({"query": gql}).encode(),
                                 headers={"Content-Type": "application/json", "API-Key": key})
    with urllib.request.urlopen(req, timeout=90) as resp:
        data = json.load(resp)
    if data.get("errors"):
        sys.exit("New Relic error: %s" % data["errors"][0].get("message"))
    return data["data"]["actor"]["account"]["nrql"]["results"]


def nrql_time(v):
    """Relative times ("today", "2 days ago", epoch ms) pass through;
    dates like "2026-10-12 09:00" get the quotes NRQL requires."""
    v = v.strip().strip("'\"")
    if v.isdigit() or v.lower() in ("today", "yesterday", "now") or v.lower().endswith("ago"):
        return v
    return "'%s'" % v.replace("'", "")


def ts(ms):
    return dt.datetime.fromtimestamp(ms / 1000)


def money(v):
    return "${:,.2f}".format(v) if isinstance(v, (int, float)) else ""


def describe(e):
    kind = e.get("eventType()")
    bits = []
    if e.get("product_type"):
        bits.append("patio" if e["product_type"] == "patio_cover" else e["product_type"])
    if e.get("material"):
        bits.append(MATERIALS.get(e["material"], e["material"]))
    if e.get("patio_type"):
        bits.append(e["patio_type"])
    if e.get("discount_code"):
        amt = e.get("discount_amount") or 0
        bits.append("%s (−%s)" % (e["discount_code"], money(amt)) if amt > 0 else "code %s INVALID" % e["discount_code"])
    if e.get("no_account"):
        bits.append("no account")
    if e.get("entered_by"):
        bits.append("by " + e["entered_by"])
    if e.get("method"):
        bits.append(e["method"])
    return kind, " · ".join(bits), money(e.get("total_cost"))


def follow_ups(key, app, rng, events, max_gap_min=30):
    """What happened after each estimate: on-page edits (/api/calc/*), no-login
    emails, and Help/contact visits. Each request is credited to the most
    recent estimate before it (within max_gap_min). Returns {id(event): note}."""
    estimates = [e for e in events if e["eventType()"] == "EstimateViewed"]
    if not estimates:
        return {}
    q = ("SELECT timestamp, request.method, request.uri FROM Transaction WHERE appName = '%s' AND ("
         "request.uri LIKE '/api/calc/%%' OR request.uri LIKE '%%/email-me' OR request.uri LIKE '/contact%%') "
         "AND httpResponseCode != '404' %s LIMIT 2000") % (app, rng)
    tally = {}
    for r in nrql(key, q):
        prior = [e for e in estimates if e["timestamp"] <= r["timestamp"]
                 and r["timestamp"] - e["timestamp"] <= max_gap_min * 60000]
        if not prior:
            continue
        est = max(prior, key=lambda e: e["timestamp"])
        uri = r.get("request.uri", "")
        t = tally.setdefault(id(est), {"edits": 0, "emailed": False, "help": False})
        if uri.startswith("/api/calc"):
            t["edits"] += 1
        elif uri.endswith("/email-me"):
            t["emailed"] = True
        elif uri.startswith("/contact"):
            t["help"] = True
    notes = {}
    for k, t in tally.items():
        bits = []
        if t["edits"]:
            bits.append("%d edit%s" % (t["edits"], "" if t["edits"] == 1 else "s"))
        if t["emailed"]:
            bits.append("emailed")
        if t["help"]:
            bits.append("help/contact")
        if bits:
            notes[k] = "  · " + ", ".join(bits)
    return notes


def main():
    ap = argparse.ArgumentParser(description="Calculator activity report from New Relic")
    ap.add_argument("--since", default="today", help='NRQL SINCE, e.g. "today", "2 days ago" (default: today)')
    ap.add_argument("--until", default=None, help='NRQL UNTIL, e.g. "today" (default: now)')
    ap.add_argument("--app", default="colout2", help="colout2 (prod, default) or colout2-test (local)")
    ap.add_argument("--trails", dest="trails", action="store_true", default=True, help=argparse.SUPPRESS)
    ap.add_argument("--no-trails", dest="trails", action="store_false", help="skip the per-visit page trails")
    ap.add_argument("--window", type=int, default=3, help="minutes before/after an estimate to include in its trail (default 3)")
    args = ap.parse_args()

    key = load_key()
    rng = "SINCE %s" % nrql_time(args.since)
    if args.until:
        rng += " UNTIL %s" % nrql_time(args.until)
    app = args.app.replace("'", "")

    fields = ("eventType(), timestamp, product_type, total_cost, material, patio_type, no_account, entered_by, method, "
              "discount_code, discount_amount")
    events = nrql(key, "SELECT %s FROM %s WHERE appName = '%s' %s LIMIT 1000" % (fields, ", ".join(FUNNEL_EVENTS), app, rng))
    events.sort(key=lambda e: e["timestamp"])

    print("\nColumbia Outdoor — calculator activity (%s, %s%s)" % (app, args.since, " → " + args.until if args.until else ""))
    print("=" * 72)
    counts = {}
    for e in events:
        counts[e["eventType()"]] = counts.get(e["eventType()"], 0) + 1
    if not events:
        print("No funnel events in this range.")
    else:
        order = [k for k in FUNNEL_EVENTS if k in counts]
        print("  " + "   ".join("%s: %d" % (k, counts[k]) for k in order))
        print()
        notes = follow_ups(key, app, rng, events)
        for e in events:
            kind, what, cost = describe(e)
            mark = "  " if kind == "CalculatorStarted" else "▶ "
            extra = notes.get(id(e), "")
            print("%s%s  %-22s %-48s %s%s" % (mark, ts(e["timestamp"]).strftime("%m-%d %H:%M:%S"), kind, what, cost, extra))

    if not args.trails:
        return
    estimates = [e for e in events if e["eventType()"] == "EstimateViewed"]
    if not estimates:
        return
    print("\nVisit trails (pages around each estimate, bots/404s hidden)")
    print("=" * 72)
    w = dt.timedelta(minutes=args.window)
    shown_until = None
    for est in estimates:
        t = ts(est["timestamp"])
        start, end = t - w, t + w
        if shown_until and start < shown_until:
            start = shown_until  # don't repeat requests already shown for the previous estimate
        q = ("SELECT timestamp, request.method, request.uri, httpResponseCode FROM Transaction "
             "WHERE appName = '%s' AND request.uri NOT LIKE '/css/%%' AND request.uri NOT LIKE '/images/%%' "
             "AND request.uri NOT LIKE '%%favicon%%' SINCE %d UNTIL %d LIMIT 2000") % (
            app, int(start.timestamp() * 1000), int(end.timestamp() * 1000))
        rows = sorted(nrql(key, q), key=lambda r: r["timestamp"])
        noise = [r for r in rows if str(r.get("httpResponseCode")) == "404" or r.get("request.uri") in NOISE_URIS]
        real = [r for r in rows if r not in noise]
        kind, what, cost = describe(est)
        print("\n%s  %s %s" % (t.strftime("%m-%d %H:%M:%S"), what, cost))
        for r in real:
            uri = r.get("request.uri", "")
            flag = ""
            if uri.startswith("/estimate/email-me") or uri.startswith("/patio-estimate/email-me"):
                flag = "  ← emailed estimate"
            elif uri.startswith("/contact"):
                flag = "  ← contact / help"
            elif uri.startswith("/api/calc"):
                flag = "  ← edited estimate"
            elif uri in ("/estimate", "/patio-estimate") and r.get("request.method") == "POST":
                flag = "  ← estimate"
            print("    %s  %-4s %-34s %s%s" % (ts(r["timestamp"]).strftime("%H:%M:%S"), r.get("request.method", ""),
                                             uri[:34], r.get("httpResponseCode", ""), flag))
        if noise:
            print("    (+%d bot/scanner requests hidden)" % len(noise))
        shown_until = end
    print()


if __name__ == "__main__":
    main()
