# Drip — v1 Plan (Full Hybrid MVP)

Status: **DRAFT — pending user review & adjustments** (last updated 2026-08-11)
Research basis: `docs/research.md` (310 lines, cited)

## Product

Drip = Golang TUI to track & manage paid subscriptions:
- what you pay (amount, cycle), to whom (service, category)
- when each payment is due (next-due dates, reminders)
- a database of what you've been subscribed to and for how long (price history, first/last seen)

## Confirmed decisions (planning session)

| Decision | Value | Notes |
|---|---|---|
| User's banks/cards | **HDFC, ICICI** | Parser templates target these first |
| UPI apps | **Google Pay only** | GPay UPI Autopay mandate list (NPCI OC-223) is authoritative |
| MVP scope | **Full hybrid** | Statement imports + recurring detection + mandate entry + due-date reminders |
| TUI stack | **Bubble Tea + Lip Gloss + Bubbles** | Go 1.24+, `modernc.org/sqlite` (pure Go, no CGO) |
| Data acquisition | **Manual export + local parser** | No APIs/AA (dead ends, per research) |

## Stack

| Component | Choice | Why |
|---|---|---|
| TUI | Bubble Tea + Lip Gloss + Bubbles | Mature, componentized tables/forms |
| Storage | SQLite via `modernc.org/sqlite` | Single-file, zero deps, no CGO |
| PDF extraction | `unipdf` or `ledongthuc/pdf` | Card statements are password-protected |
| PDF unlock | `qpdf` (or derive password locally) | HDFC/ICICI password patterns known |
| Module | `drip` (local module for now) | Remote path can be added later |

## Data model (SQLite tables)

- `subscriptions` — service, category, amount, currency (INR), cycle (weekly/monthly/quarterly/yearly),
  rail (`upi_autopay|card_emandate|app_store|merchant_direct|bank_si`), mandate_id, upi_app,
  card_last4, issuer, next_payment_date, last_payment_date, status (active/paused/cancelled),
  merchant_descriptors, first_seen
- `statements` — raw ledger: date, value_date, description, amount, balance, source_file, account
- `imports` — audit: source, format, file, imported_at, matched, unmatched
- `price_history` — amount changes over time

## Import pipeline (monthly routine ~10–15 min)

```
drip import ~/drip-inbox/
├─ HDFC savings CSV (mobile app)      → UPI + e-mandate debits
├─ ICICI savings CSV (netbanking)     → UPI + e-mandate debits
├─ HDFC card PDF (pwd: NAME-first4-CAPS + card-last4)
├─ ICICI card PDF (pwd: name-first4-lower + DOB DDMM)
└─ GPay "Get statement" PDF           → UPI-via-GPay cross-check
```

Plus `drip mandate add`: paste/copy GPay **UPI Autopay list** (authoritative, cross-app complete per NPCI OC-223).

## Detection engine

- Normalize descriptors (`UPI/123/Netflix/Netflix.com` → `netflix`)
- Match **amount ± tolerance + same day-of-month across ≥2 statements** → candidate subscription
- Overlay mandate list → auto-activate / flag mismatches

## TUI screens

1. Dashboard — active subs, monthly spend, next 7 days due
2. Subscriptions — sortable table, status, next due, amount
3. Detail — price history, last charges, descriptors, reconcile status
4. Add/Edit form — manual fallback (always available)
5. Import — pick folder, run parse, show matched/unmatched
6. Reconcile — GPay mandates vs ledger gaps

## Milestones

1. ✅ **Scaffold** — module, Bubble Tea shell w/ nav, SQLite schema, migrations (committed)
2. ✅ **Import engine** — `drip import <dir>` + HDFC PDF parser (158-record real-statement test: 120 debit/38 credit, balance chain, totals) — CSV/XLSX parsers pending samples
3. ✅ **Detection** — recurring heuristics → subscription upserts (UPI-AUTOPAY marker = guaranteed sub; dominant-amount per service; cadence inference; price history; reactivation)
4. ⏳ **Card PDFs** — unlock + extract HDFC/ICICI card statements
5. ⏳ **GPay** — statement PDF + mandate list ingestion
6. ✅ **TUI polish** — interactive subscriptions tab: ↑/↓ select, detail view (price history, descriptors), edit form, delete confirm; dashboard due-in-7-days list; due-today ⚠ markers
7. ⏳ **Reconcile view** + backfill from existing statements

## Open items (verify during build)

- GPay "Get statement" PDF layout — need a sample to spec the parser
- GPay UPI Autopay screen export affordance (view-only today → manual/OCR)
- HDFC/ICICI savings statement CSV exact column layouts — user's real exports
- Card statement PDF layouts for user's specific cards
- Recurring-detection tolerances (amount drift, day-of-month drift)
- Whether user wants Gmail-API ingestion in v2 (restricted OAuth scope)

## Next step

User reviews this plan and adjusts; then we kick off milestone 1 (scaffold).
