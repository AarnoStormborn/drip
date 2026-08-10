# Drip — Research: Procuring Subscription & Payment Data in India

**Project:** Drip — a Golang TUI to track paid subscriptions (what you pay, to whom, when).
**Research compiled:** 2026-08-11 · **Scope:** automated *and* manual ways to obtain subscription/payment data in India (Google Pay/UPI, credit cards, banks), parsing approaches, existing tools, regulation, and a recommended MVP workflow.

---

## Executive Summary

1. **There is no official, consumer-facing API for UPI or credit card data in India.** UPI APIs are licensed by NPCI to banks/PSPs; card APIs are B2B bank APIs. No personal-access APIs exist. Any "automated" ingestion must therefore be built on (a) **file exports the user downloads themselves** (statements, Takeout), (b) **email parsing** (banks email transaction alerts/receipts — there is already an open-source Go tool doing exactly this), or (c) **consumer app reverse engineering** (fragile, ToS-risky — not recommended for a personal tool).

2. **The RBI Account Aggregator (AA) framework is the *only* regulated programmatic path to bank/card data — and it is closed to individuals.** To join as a Financial Information User you must be an RBI/SEBI/IRDAI/PFRDA-regulated entity, pass Sahamati certification, and budget ₹5–25 lakh+ in year one. A personal TUI cannot use it directly. Individuals *can* use AA consumer apps (OneMoney, Finvu, etc.) as a human interface, but there is no sanctioned export path from those apps to your own tool. **Verdict: not viable for Drip's MVP.**

3. **Manual download is genuinely low-effort and reliable.** Banks are required by RBI to provide statements; all major banks offer self-service PDF/Excel/CSV downloads via netbanking/mobile apps (details + PDF password patterns below). Google Pay India now even has a built-in **"Get statement" PDF export** for its UPI transaction history.

4. **UPI AutoPay mandates are now visible in one place.** NPCI circular **OC-223 (Oct 2025)** requires every UPI app to let you view *all* your active UPI Autopay mandates (not just its own), with port/revoke/pause. Google Pay's "UPI Autopay" section is thus a reliable, official source for the mandate list (merchant, amount, frequency, next debit date) — manual copy-in or screenshot-based entry.

5. **A hybrid "manual-export + local parser" model is the right MVP:** user downloads 2–4 files a month (bank statement CSV/PDF, credit-card statement PDFs, optionally GPay statement PDF, plus the GPay mandate list), Drip parses them locally (pdfplumber-style extraction; Go libs exist), detects recurring transactions by amount/cadence/merchant, and maintains a subscription ledger. This matches what the best-ranked consumer tools do (LowerMySubs' client-side statement scanner beat bank-connected tools on accuracy in a 2026 test) and keeps all data on-device.

6. **Regulatory/privacy landscape is favorable to a self-hosted tool:** India's DPDP Act 2023 is built on consent, and a personal tool processing your *own* statements locally is the least-problematic case. The main risks are (a) sharing bank credentials with third parties (screen-scraping ToS), and (b) app-store/email-scope consent hygiene if you use Gmail API parsing.

---

## 0. Background: Where Subscription Data Actually Lives in India

Recurring payments in India ride four different rails; each has a different "source of truth":

| Rail | How it works | Where the record lives | Regulator/operator |
|---|---|---|---|
| **UPI AutoPay** | Standing e-mandate approved once with UPI PIN; merchant pulls on schedule | UPI app (mandate list, now cross-app via NPCI OC-223); bank statement shows debits | NPCI |
| **Card e-mandate** (credit/debit) | Recurring debit on card per RBI e-mandate framework (AFA up to ₹15,000 per tx without extra AFA) | Bank/card issuer netbanking "Standing Instructions / e-Mandates"; card statement | RBI |
| **App-store billing** | Google Play / Apple subscriptions billed to your UPI/card | Google Play account (subscriptions page), payments.google.com, Apple Subscriptions | Google/Apple |
| **Direct merchant billing** | Merchant charges on its own schedule (Netflix via credit card, gym, insurance premiums, net-banking standing instructions) | Merchant account page; card/bank statement | — |

Key facts:
- **RBI e-mandate framework:** card-based recurring payments up to ₹15,000 per transaction can execute with AFA at mandate creation only (raised from ₹2,000 → ₹5,000 → ₹15,000 in Jun 2022); above that, per-transaction AFA applies. Banks must offer an e-mandate "cancel" surface. Sources: [RBI circular 2019](https://www.rbi.org.in/Scripts/NotificationUser.aspx?Id=11784), [The Hindu, Jun 2022](https://www.thehindu.com/business/rbi-raises-limit-of-e-mandates-for-transactions-up-to-15000/article65534388.ece), [Business Standard](https://www.business-standard.com/article/finance/new-e-mandate-guidelines-rbi-enhances-limit-for-e-mandates-on-credit-debit-cards-to-rs-15-000-122060800417_1.html).
- **UPI AutoPay rules:** merchants must send a pre-debit notification ≥24h before each debit (SMS/app) for mandates above ₹5,000; NPCI runs a dispute-redressal mechanism. Source: [NPCI UPI AutoPay product overview](https://www.npci.org.in/what-we-do/autopay/product-overview), [HDFC cancel-UPI-AutoPay page](https://www.hdfc.bank.in/payments/services/cancel-upi-autopay-mandate).
- ~60% of Indian recurring OTT/utility bills now run on UPI AutoPay or card e-mandates (industry estimate, per Essara's 2026 tracker comparison; flagged as to-verify).

---

## 1. Google Pay / UPI Data Extraction

### 1.1 Google Takeout (Google Pay / Wallet data)

- **Method:** takeout.google.com → select **Google Pay** (and/or **Google Wallet**) → format (ZIP, JSON/CSV/HTML or both) → export. [Google support: Find, export, or delete Google Pay info](https://support.google.com/googlepay/answer/9015738?hl=en)
- **What you get (typically):** your Google Payments profile activity — transactions made via Google-billed surfaces (Play Store, YouTube, etc.), gift cards, payment methods, activity logs. Files land as JSON/CSV/HTML bundles; transaction exports are commonly structured records (JSON with nested `transactions`; CSV with columns like Date, Description, Amount, Status, Payment method).
- **Critical caveat for India:** Google Pay (India) UPI activity is *not* a clean "all UPI transactions" export. The GPay India app's own help explicitly says transaction history "only contains those made through Google Pay and not all UPI or banking transactions." GPay activity lives on [myactivity.google.com/product/gpay](https://myactivity.google.com/product/gpay) and is **auto-deleted after 18 months by default** (you can change 3/18/36 months or disable). So Takeout is a *supplement* for Google-billed purchases, not the canonical UPI ledger. Sources: [GPay India help — view transaction history](https://support.google.com/pay/india/answer/7430307?hl=en-IN), [Google support — export/delete GPay info](https://support.google.com/googlepay/answer/9015738?hl=en).
- **Google Data Portability API (programmatic):** Google offers a [Data Portability API](https://developers.google.com/data-portability) (built for DPDP/DTPA compliance) with JSON exports for Google Play Store data (incl. **Purchases** object) and more, with one-time or time-bound (30/180-day) consent scopes. It requires registering a project as a data-exporting product and a user consent flow — heavyweight for a personal TUI, but it is the only *official* programmatic Google export path. Sources: [Data Portability API overview](https://developers.google.com/data-portability/user-guide/overview), [Play Store schema reference](https://developers.google.com/data-portability/schema-reference/play), [release notes](https://developers.google.com/data-portability/docs/release-notes).

### 1.2 Google Pay India app — "Get statement" (new, important)

- The GPay India app now supports **direct PDF/e-statement export of your GPay transaction history**: app → **See transaction history** → menu (⋮) → **Get statement** → pick period → view/save/share. Source: [GPay India help — View transaction history](https://support.google.com/pay/india/answer/7430307?hl=en-IN) (section "Get a PDF or e-statement of Google Pay transaction history").
- Third-party samples confirm the statement naming convention like `gpay-statement-20250801-20251031` (PDF). This is a *primary, official* export for the user's UPI-via-GPay history — Drip should treat the GPay statement PDF as a first-class import format.

### 1.3 UPI transaction history — canonical sources

- **The bank is canonical:** UPI debits appear in your bank account statement (e.g., descriptors like `UPI/…/NETFLIX/NETFLIX.COM` or `UPI/…/Spotify/Spotify AB`). Banks let you download statements for 3–7+ years (see §4).
- **Other PSPs:** Paytm offers UPI statement downloads in PDF and Excel ([Paytm blog](https://paytm.com/blog/payments/paytm-expands-upi-statement-downloads-with-excel-format-for-easy-tax-filing-and-expense-tracking/), [ET](https://economictimes.indiatimes.com/wealth/save/paytm-upi-statement-download-paytm-upi-users-can-now-download-expense-statement-in-pdf-excel-to-track-spending-habits/articleshow/118846903.cms)). PhonePe has a statement/export path too (verify exact menu).
- **No official API:** There is no consumer API for UPI transaction history. UPI APIs are licensed by NPCI to member banks/PSPs. Google's only GPay-India APIs are *merchant* APIs (e.g., [Get Transaction Details](https://developers.google.com/pay/india/api/otherapis/omnichannel/get-transaction-details)) tied to a merchant transaction ID — useless for personal ingestion. A Stack Overflow question asking for exactly this ("API for Google Pay UPI to fetch transaction history") went unanswered (SO#75873845).

### 1.4 UPI AutoPay / e-mandate management (listing active mandates) — big 2025 change

- **NPCI OC-223 (circular NPCI/UPI/OC-223/2025-26, 7 Oct 2025; compliance deadline 31 Dec 2025):** every UPI app must now show **all** of your active UPI Autopay mandates (view-anywhere), and support **porting** a mandate to another app (once per rolling 90 days, UPI PIN required, no incentives allowed). Source: [NPCI circular PDF](https://www.npci.org.in/uploads/UPI_OC_No_223_FY_2025_26_Enhancement_of_UPI_Autopay_88b38535cb.pdf), [RTI Wiki explainer](https://righttoinformation.wiki/upi-autopay-mandate-port-manage-across-apps-2026).
- **Practical consequence for Drip:** the user can open **Google Pay → Profile → UPI Autopay** (or any compliant UPI app) and see the full mandate list: merchant, amount, frequency, next debit date, and revoke/pause/port actions. This is the *best official source* for "what auto-pays are active right now" — it's a manual read/screenshot/copy-in step, but it's authoritative and complete across apps.
- **Bank-side view:** most bank netbanking/mobile apps also expose e-mandates / standing instructions (e.g., SBI YONO, HDFC NetBanking "E-Mandate", SBI Card e-mandate FAQ). The merchant can also cancel on their side.
- **No NPCI consumer portal:** NPCI does not offer an individual-facing mandate dashboard; management happens in PSP apps and bank apps only.
- **Pre-debit notifications:** for mandates above ₹5,000, merchant must notify ≥24h in advance (via UPI app + remitter-bank SMS) — useful as a secondary "due payment" signal Drip could remind about.

---

## 2. Credit Card Data (India)

### 2.1 Downloading statements per issuer

All major issuers provide self-service statement download via netbanking/card portals; most PDFs are password-protected (see password table in §4.2). Formats:

| Issuer | Channel | Formats | Notes |
|---|---|---|---|
| **HDFC Cards** | NetBanking (Cards → e-statements / Credit Card statement); mobile app | PDF (password: first 4 letters of name CAPS + last 4 digits of card, e.g. `RAJE3456`); CSV export on some cards (e.g., Regalia Gold, RuPay UPI cards per `hdfc-cc-parser-rs`); XLS via statement download | ~3 years history online; historical statements downloadable with charges |
| **ICICI Cards** | iMobile / NetBanking (Cards → e-statements) | PDF (password: first 4 letters of name lowercase + DOB DDMM, e.g. `rohi0806`) | multiple years history |
| **SBI Card** | sbicard.com → statement download; e-statement by email | PDF (password: DOB **DDMMYYYY** + last 4 digits of card; emailed statements: DOB DDMM + `@` + last 4 of registered mobile) | [SBI Card FAQ](https://www.sbicard.com/en/faq/statement-billing-related.page), [Airtel](https://www.airtel.in/blog/credit-card/what-is-my-sbi-credit-card-statement-password/), [ClearTax](https://cleartax.in/s/sbi-statement-password) |
| **Axis Bank** | NetBanking → Cards → statement download | PDF (password varies — often first 4 of name + last 4 of card; **verify**) | — |
| **American Express India** | americanexpress.com/in → statements | PDF; CSV report export available for merchant accounts ([Amex merchant support](https://www.americanexpress.com/in/merchant/support-centre/payments/downloading-e-statements.html)); consumer CSV **to verify** | — |
| **RuPay** | Issued by member banks → use the issuing bank's portal | Same as issuing bank | RuPay is an NPCI card network, not an issuer — no own portal for consumer statements ([rupay.co.in](https://www.rupay.co.in/)) |

Notes:
- **Networks (Visa/Mastercard/RuPay) don't issue statements to consumers** — the issuing bank does. Visa/Mastercard's "statement APIs" (e.g., [Mastercard Open Banking](https://www.mastercard.com/us/en/business/open-finance/solutions/data/statements.html)) are for regulated businesses and are US/EU-oriented; not usable by an individual in India.
- **Bank API portals are B2B:** e.g., [HDFC Bank API portal](https://developer.hdfcbank.com/credit-card-billed-unbilled-transactions) offers "Credit Card Billed/Unbilled Transactions" APIs for approved business partners — not for individuals.
- **PDF passwords are derived from personal data and can be unlocked locally** with tools like `qpdf`/`pikepdf` (you know the components: name/DOB/card last4). For a personal tool, asking the user to supply the components and deriving/unlocking locally is simple and keeps the secret on-device.

### 2.2 CRED as an aggregator

- CRED aggregates credit-card statements/bills across your cards (15+ cards) and shows due dates, unbilled spends, etc. It obtains data via **cardholder-authorized integrations with issuers** (card addition + verification), not via a public API. Exactly how (issuer tie-ups vs credential-based fetch) is not fully public; one engineer publicly documented **reverse-engineering the CRED Android app** to pull statements for 15 cards via its internal APIs ([LinkedIn: Shailesh Jain](https://www.linkedin.com/posts/shailujain_reverseengineering-apis-googlescript-activity-7247187417936384000-pD2V)). Useful as proof the data is fetchable, but app-reverse-engineering is fragile (API churn, ToS) — not recommended for Drip.
- CRED/INDmoney-style aggregation is a *viewing* convenience; the underlying truth remains the issuer's statement, which you can download yourself.

---

## 3. RBI Account Aggregator (AA) Framework

### 3.1 How it works

- Tripartite consent architecture defined in the RBI Master Direction (first issued 2016, updated 9× since): **FIP** (Financial Information Provider — bank/insurer/depository/NPS CRA/GSTN holding the data) ⇄ **AA** (RBI-licensed consent manager, data-blind pipe) ⇄ **FIU** (Financial Information User — the consuming app).
- Flow: FIU requests consent via the customer's AA app → customer approves (what data, from which FIPs, for how long, for what purpose) → FIP encrypts and streams the data through the AA → FIU decrypts. Customer can view/revoke consents anytime. Technical specs by **ReBIT** (api.rebit.org.in). Sources: [Sahamati](https://sahamati.org.in/what-is-account-aggregator/), [casparser "State of AA 2026"](https://casparser.in/blog/state-of-account-aggregator-2026/), [Setu docs](https://docs.setu.co/data/account-aggregator/v1/licenses-and-go-live/participants-in-aa), [Business Standard explainer](https://www.business-standard.com/finance/personal-finance/what-are-account-aggregators-why-do-you-need-them-how-do-they-work-124082700746_1.html).

### 3.2 Licensed AAs and ecosystem state (as of early-mid 2026)

- ~17 operating AAs (13 with live FIP integrations); top AAs by FIP coverage: **Anumati (80+), CAMS AA (70+), OneMoney (65+), Finvu (60+), NADL (60+)**. Other licensed AAs: PhonePe AA, Jio Financial AA, Setu AA (gateway/TSP model), DigiO, Yodlee AA, etc. Sahamati is the RBI-recognized SRO. Sources: [Sahamati — download AA apps](https://sahamati.org.in/download-account-aggregator-apps), [Sahamati](https://sahamati.org.in/), [Setu participants list](https://docs.setu.co/data/account-aggregator/v1/licenses-and-go-live/participants-in-aa).
- **Live data:** savings accounts (all 72 banks), equities/MF via CDSL+NSDL, MF folios via CAMS/KFin RTAs, GST returns, NPS. **Patchy:** FDs/RDs (~40% of banks), current accounts, insurance (varies by AA). **Not live at all:** bonds/debentures/G-Secs/CPs/CDs, EPF, PPF. **Excluded:** joint accounts, NRE/NRO.
- **Credit cards via AA: not a reliable data source.** Card *billing* data is not among the broadly-live AA categories; don't design around it.

### 3.3 Can a personal tool use AA? **No (effectively).**

- To become an **FIU** you must be regulated by RBI/SEBI/IRDAI/PFRDA, pass **Sahamati certification** (security + API-schema + FI-schema conformance + central-registry integration), run quarterly self-tests, biennial IS audits, and re-certify on spec changes. Since Oct 2023, regulated entities joining as FIU must also join as **FIP** if they hold financial information (no free-riding). Cost: ₹5–25 lakh+ first year via TSPs (Setu, etc.). Sources: [casparser AA state 2026](https://casparser.in/blog/state-of-account-aggregator-2026/), [Setu FIU onboarding](https://docs.setu-aa.com/fiu-onboarding).
- **Individuals *can* use AA consumer apps** (OneMoney, Finvu, CAMS Finserv AA, NADL AA, Anumati AA — Android/web) to view and share their own data with consent, including with banks' own portals ([ICICI AA page](https://www.icici.bank.in/account-aggregator)). But there is **no sanctioned export/download of raw JSON from these consumer apps** for a personal pipeline; they are viewing tools. (Worth a targeted check per app — "download statement" buttons may appear; currently not documented.)
- **Setu AA gateway:** Setu (Pine Labs) offers AA APIs + a [Go client SDK](https://github.com/fintech-sdk/setu-client-go) and consent-flow docs ([consent flow](https://docs.setu.co/data/account-aggregator/api-integration/consent-flow), [overview](https://docs.setu.co/data/account-aggregator/overview)), but access is gated to business onboarding (KYC + FIU pathway) — not personal use.
- **Verdict for Drip:** treat AA as a *future* integration only if Drip ever becomes a licensed product; for a personal TUI, it's out of scope.

---

## 4. Bank Statement Formats & Parsing

### 4.1 What you can download (verified per bank)

| Bank | Netbanking formats | Mobile-app formats | Online history | PDF password (typical) |
|---|---|---|---|---|
| **SBI** | PDF, Excel (**no CSV**) | YONO: PDF (passbook icon) | recent periods; older via branch | 11-digit account no. (download); email: last5-mobile + DOB DDMMYY; YONO: DOB DDMM + `@` + last4 mobile |
| **HDFC** | PDF, Excel, Text/Delimited | PDF, Excel, **CSV** | ~3 years | Customer ID (savings); name-first4-CAPS + card-last4 (cards) |
| **ICICI** | PDF, Excel, **CSV** | PDF only (iMobile) | 4–7 years | name-first4 lowercase + DOB DDMM |
| **Axis** | PDF (+ others vary) | — | — | verify per card/account |

Sources: [BankStatementLab — SBI/HDFC/ICICI download guide (rechecked Jul 2026)](https://www.bankstatementlab.com/en/blog/en-download-bank-statement-sbi-hdfc-icici), [CreditMantri — HDFC](https://www.creditmantri.com/hdfc-bank-statement-download/), [CreditMantri — Indian Bank](https://www.creditmantri.com/indian-bank-statement-download/), [SBI Card FAQ](https://www.sbicard.com/en/faq/statement-billing-related.page).
- RBI requires banks to provide a monthly statement/passbook on request at no charge (amended directions effective 1 Apr 2026 reaffirm monthly periodicity) — so downloading is a guaranteed right, and routine monthly downloads are free.
- WhatsApp/SMS channels exist for balance/mini-statements (SBI: 09223588888 ESTMT; HDFC: 70700 22222; ICICI: 86400 86400) but give PDF statements only via netbanking/apps.

### 4.2 PDF password handling (for Drip's importer)

- Patterns are derivable (name, DOB, card/account last digits — see tables above). Bank statement PDFs are encrypted at rest; a local tool can unlock them with `qpdf --password=…` or Python `pikepdf`, or the user can save an unlocked copy once.
- **Caveat:** passwords differ by channel (download vs email vs app) and account type; always read the instruction line in the email/PDF. Case-sensitivity matters (ICICI lowercase).

### 4.3 Parsing approaches & existing libraries

- **Text extraction:** `pdfplumber` / `camelot` / `tabula-py` (Python) for tabular PDF statements; layout-based table extraction works well for HDFC/ICICI/Axis statements; SBI's two-column + running-balance layout is trickier.
- **Go options (fits Drip's stack):** `unipdf`, `pdfcpu`, `ledongthuc/pdf` for text extraction; `excelize` for XLS/XLSX (password-protected XLS needs an unlock step first); pure-Go CSV trivial. Realistically, the robust path is: **PDF→text→regex/tokenization per bank template**, mirroring the Python ecosystem.
- **Existing open-source parsers (reference implementations):**
  - `xaneem/hdfc-credit-card-statement-parser` (Python) — HDFC card statements ([GitHub](https://github.com/xaneem/hdfc-credit-card-statement-parser))
  - `santosh1994/hdfc-creditcard-statement-parser` — Diners/Rewards-points extraction ([GitHub](https://github.com/santosh1994/hdfc-creditcard-statement-parser))
  - `joeirimpan/hdfc-cc-parser-rs` — **Rust**, HDFC CSV exports (Regalia Gold, RuPay UPI cards) ([GitHub](https://github.com/joeirimpan/hdfc-cc-parser-rs))
  - `madhav921/stmt-forge` (Card-Statement-Analyser) — Python, parses HDFC/ICICI/SBI/Axis + 5 more card PDFs ([GitHub](https://github.com/madhav921/stmt-forge))
  - `raptar231/indian-bank-statement-parser` — PyPI package, HDFC/ICICI/SBI/Axis → CSV/JSON ([PyPI](https://pypi.org/project/indian-bank-statement-parser/))
  - `nikhilweee` gist — HDFC card PDF → Excel ([gist](https://gist.github.com/nikhilweee/24cae428f68c153afda495dc17ef43d6))
  - SBI statement converter (Streamlit, PDF→CSV) ([app](https://sbt-statement-converter.streamlit.app/))
  - `akhilnarang/bank-statement-parser` (Python, MIT) ([GitHub](https://github.com/akhilnarang/bank-statement-parser))
  - `nitinn77/credit-card-spends-tracker` — **Go**, Gmail-API based, Axis+HDFC ([GitHub](https://github.com/nitinn77/credit-card-spends-tracker)) — see §7
- **Indian statement quirks:** UPI descriptors like `UPI/<ref>/<merchant>/<merchant.psp>` (e.g., `UPI/123456789/Netflix/Netflix.com`), `NEFT/IMPS` credits, "bal" running-balance columns, lakh/₹-format numbers, transaction-date vs value-date pairs, and per-bank date formats (DD/MM/YY vs MM/DD/YY). CSV exports from HDFC mobile/ICICI netbanking are the least painful inputs.

---

## 5. Existing Subscription-Management Tools (and how they get data)

### 5.1 Global (US/EU-centric)

| Tool | Data acquisition | India-relevant? |
|---|---|---|
| **Rocket Money** (ex-Truebill) | Bank sync via **Plaid** (OAuth), auto-detects recurring charges (~94% acc.); human cancellation service | Plaid doesn't cover Indian banks → not usable |
| **Monarch / Quicken Simplifi** | Plaid/Finicity bank sync, 12k–14k institutions | Same limitation |
| **Trim** | Bank credentials (SMS chatbot), bill negotiation | No |
| **Bobby** (iOS) | **Manual entry** (1,000+ service library), local storage | Works as an app, but manual-only and iOS-only |
| **Subby** (Android) | Manual entry (1,500+ services), renewal alerts | Works; no UPI-AutoPay model |
| **LowerMySubs** | **Browser-based statement scanner** (upload CSV/PDF, client-side recurring-charge detection) + manual | Concept directly portable to Drip; in a 2026 8-app test it found 14/14 subs and beat bank-connected apps ([test](https://www.lowermysubs.com/blog/best-subscription-trackers-2026)) |

### 5.2 India-specific

- **Essara** (web/PWA/Android) — treats UPI AutoPay as a first-class rail with mandate-ID field, no bank login, manual-first; explicitly argues bank feeds miss UPI descriptors and expose statements to third parties ([comparison](https://essara.space/blog/best-subscription-tracker-app-india)). **This validates Drip's manual-first premise.**
- **INDmoney** — credit-card bill tracking via (per reviews) **email parsing of card statements** + card integrations; also syncs investments ([features](https://www.indmoney.com/features/track-credit-card-bills), [review](https://aayushbhaskar.com/indmoney-review/)).
- **CRED** — card-bill aggregation via authorized issuer integrations (see §2.2); also offers "subscriptions" management.
- **UPI Track Autopay & Subscription** (Android, com.budrock.upitracker) — tracks UPI autopay mandates + subscription costs; data acquisition is manual entry with reminders (no bank login) ([Play Store](https://play.google.com/store/apps/details?id=com.budrock.upitracker)).
- **Golang TUIs / open source:** `shen-kit/finance-tracker-tui` (Go, SQLite ledger) ([GitHub](https://github.com/shen-kit/finance-tracker-tui/)); `leeberlin/subscription-tracker` (TS); no mature Go TUI for Indian subscriptions exists — **Drip has a clear niche**.

### 5.3 Data models used by these tools (for Drip's schema)

Common subscription entity across Bobby/Subby/Essara/LowerMySubs:
- **Identity:** service name (+ logo/icon), category
- **Money:** amount, currency (INR), billing cycle (weekly/monthly/quarterly/yearly), trial flag, price history
- **Schedule:** next payment date, renewal frequency, "due soon" reminders
- **Payment rail:** UPI AutoPay (with UPI app + mandate ID), card e-mandate (card last4, issuer), Google Play/App Store, direct merchant, net-banking SI
- **State:** active / paused / cancelled, plus last-charged and expected-next-charge (reconciled against statements)
- **Detection signals:** merchant descriptor matches (e.g., "NETFLIX"), recurring-amount cadence, same-day-of-month debits

---

## 6. Regulatory & Privacy Considerations (India)

- **DPDP Act 2023** (India's first comprehensive data-protection law, in force): built on notice-and-consent; consent must be free, specific, informed, unambiguous. A **self-hosted tool processing your own statements on your own device is the lowest-risk processing** imaginable (no data fiduciary in the third-party sense). If Drip ever syncs anything to a server, DPDP rules on consent, purpose limitation, and DPIA would apply. Sources: [MeitY DPDP Act text](https://www.meity.gov.in/static/uploads/2024/06/2bf1f0e9f04e6fb4f8fef35e82c42aa5.pdf), [Wikipedia overview](https://en.wikipedia.org/wiki/Digital_Personal_Data_Protection_Act,_2023), [HLC summary](https://www.hlc.com/en/publications/indias-digital-personal-data-protection-act-2023-brought-into-force-).
- **DEPA (Data Empowerment & Protection Architecture, NITI Aayog):** the design philosophy behind consent-based data sharing via "consent managers" — the AA framework is its financial-sector implementation ([NITI Aayog draft](https://www.niti.gov.in/sites/default/files/2023-03/Data-Empowerment-and-Protection-Architecture-A-Secure-Consent-Based.pdf), [ORF](https://www.orfonline.org/research/data-empowerment-and-protection-architecture-concept-and-assessment)). Relevant only if Drip later plugs into AA.
- **Banking secrecy & RBI stance:** you have a right to your own statements (see §4.1); downloading them for personal use is routine. The risk area is **sharing credentials**: screen-scraping banks' portals with saved passwords (what some aggregators do) violates bank ToS and is a fraud vector; Drip should never store bank passwords. Prefer OAuth (Gmail API with your own credentials) or manual file download.
- **Gmail API consent hygiene:** email-based ingestion (card alerts, receipts) must use a restricted Google Cloud OAuth scope, your own client ID, and ideally `gmail.metadata`/label filters, never "read all email" if avoidable; Google reviews restricted-scope apps.
- **RBI e-mandate / NPCI AutoPay rules** (§0) matter as *product* knowledge: pre-debit notifications, ₹15,000 AFA threshold, mandate porting guardrails (once/90 days), and the fact that **cancellation must happen at the source** (UPI app / bank SI menu / app store / merchant) — a tracker can only remind, not cancel (except CRED-style concierge, out of scope).
- **Data retention:** GPay MyActivity auto-deletes after 18 months by default — if the user wants long history, disable auto-delete or archive Takeout/statement exports locally (which Drip should encourage: store parsed JSON next to raw files).

---

## 7. Recommended Workflow for Drip (MVP)

### 7.1 Recommended approach: "manual export + local parser" (hybrid)

Matches the highest-ranked consumer tools, avoids every dead-end (no APIs, no AA, no Plaid), and keeps data 100% on-device.

**Monthly routine (~10–15 min):**

1. **Bank statement(s)** — download from netbanking (or mobile app) in **CSV/XLS where available (ICICI/HDFC), else PDF**; covers *all* UPI + card e-mandate debits.
2. **GPay UPI Autopay mandate list** — Google Pay → Profile → UPI Autopay: read off active mandates (merchant, amount, frequency, next debit). NPCI OC-223 guarantees this list is complete across apps. (Copy-in as text or CSV; or screenshot→OCR later.)
3. **Credit-card statement PDFs** — one per card, from issuer portals (HDFC/ICICI/SBI Card/Axis/Amex); password rules in §4.2.
4. **(Optional) GPay "Get statement" PDF** — GPay app export (§1.2) as a cross-check for UPI-via-GPay history.
5. **Run `drip import`** — point at a folder; Drip parses (PDF unlock + text extraction, CSV/XLS parse), matches merchant descriptors, runs **recurring-detection heuristics** (same amount + same-day-of-month across ≥2 months; descriptor substring matches like `NETFLIX`, `SPOTIFY`, `YOUTUBE PREMIUM`, `GPay*`), and upserts subscriptions with last-charged / next-due dates.
6. **Reconcile** — flag mandates in GPay's list missing from the ledger and vice versa; flag price changes; remind before each next-due date (optionally using pre-debit notifications as a trigger).

### 7.2 Automation ladder (in increasing complexity)

1. **Statement imports** (CSV > XLS > PDF) — the MVP core; no credentials, no third parties.
2. **Email parsing via Gmail API** — pattern: `nitinn77/credit-card-spends-tracker` (Go) already does this for Axis/HDFC card transactions; extend to receipts/reminders (e.g., "Netflix invoice", "your bill is due"). Own OAuth client, restricted scope, local SQLite.
3. **Google Takeout / Data Portability API** — periodic JSON import for Google-billed purchases (Play, YouTube). Takeout is manual; the Portability API is programmatic but needs a consent/approval flow — defer.
4. **GPay mandate list automation** — the app UI is the only official surface today; automation would require app reverse-engineering (like the CRED RE project) — explicitly **not recommended** for v1.
5. **Account Aggregator (future/long-term)** — only if Drip becomes a licensed product; out of MVP scope (₹5–25L/yr, certification).

### 7.3 Comparison: automated vs manual sources

| Source | Automation level | Effort | Reliability | Coverage | Notes |
|---|---|---|---|---|---|
| Bank statement CSV/XLS (ICICI, HDFC mobile) | Manual download; parse automatic | Low | High | All debits incl. UPI & card e-mandates | Best canonical ledger |
| Bank statement PDF | Manual download; parse semi-auto (unlock+extract) | Low–Med | High (template-specific) | Same | Password patterns known; per-bank parsers exist |
| Credit-card statement PDF | Manual download; parse semi-auto | Low–Med | High | Card-specific | One per card |
| GPay "Get statement" PDF | Manual export in app | Low | High | GPay UPI history | Official, new feature |
| GPay UPI Autopay mandate list | Manual read/copy (in-app) | Low | High (OC-223) | All UPI AutoPay mandates | Best "active subscriptions" truth; not exportable |
| Gmail API (bank emails) | Semi-automated (OAuth) | Med (setup) | High for parsed banks | Cards/alerts only | Go reference impl exists |
| Google Takeout (GPay/Wallet/Play) | Manual (scheduled possible) | Low | Med | Google-billed only | JSON/CSV/HTML; GPay UPI data NOT complete |
| Google Data Portability API | Programmatic (needs project+consent) | High | High | Play purchases etc. | DPDP-flavored official path |
| CRED app APIs (reverse-engineered) | Programmatic but fragile | Very high | Depends | Cards | ToS/breakage risk; not for a personal tool |
| RBI AA (Setu etc.) | Programmatic (business onboarding) | Very high | High | Banks/investments (not cards reliably) | Requires regulated entity + certification; ₹5–25L/yr |
| Manual entry | Manual | Low | Depends on user | Everything | Always available as fallback |

---

## 8. Recommended MVP Data Model (sketch)

```
Subscription {
  id, service, category
  amount, currency (INR), cycle (weekly|monthly|quarterly|yearly)
  rail: upi_autopay | card_emandate | app_store | merchant_direct | bank_si
  mandate_id?, upi_app?, card_last4?, issuer?, platform?
  next_payment_date, last_payment_date, status (active|paused|cancelled)
  merchant_descriptors: [..], first_seen, price_history: []
}
StatementRecord { date, value_date, description, amount, balance, source_file, account }
Import { source, format, file, imported_at, matched: n, unmatched: m }
```

Recurring-detection core: group debits by descriptor-normalized merchant + amount within ±tolerance, require ≥2 occurrences at consistent cadence → candidate subscription; overlay mandate list from GPay for UPI AutoPay truth.

---

## 9. Open Questions (to verify during build)

1. **User's actual banks/cards:** ~~Drip should target HDFC/ICICI/SBI first — confirm which issuers the user holds (parsers differ per bank template).~~ **RESOLVED (planning session): user holds HDFC and ICICI cards; UPI via Google Pay only.** Target parsers: HDFC + ICICI (savings statements + card statements) and GPay statement PDF.
2. **Axis card-statement password rule** and **Amex India consumer CSV export** — verify against the user's actual statement emails.
3. **GPay "Get statement" PDF layout** — needs a sample to spec the parser (dates, columns, ₹ formatting, descriptor truncation).
4. **GPay mandate list exportability** — confirm whether Google Pay's UPI Autopay screen offers any share/copy affordance (today it appears view-only; manual entry or OCR).
5. **PhonePe mandate list** — if the user also uses PhonePe, same OC-223 path applies; verify menu labels.
6. **Gmail-based ingestion scope** — decide in v2: card txn emails vs receipts vs reminders; restricted-scope app approval overhead.
7. **Multi-source reconciliation rules** — when bank CSV and GPay statement disagree on a debit, which wins (bank CSV), and how to surface mismatches.
8. **Historical backfill** — one-off Takeout + 12-month bank statement import to seed the ledger.

---

## 10. Key Sources

**Google / UPI**
- [Google Pay help — export or delete GPay info (Takeout)](https://support.google.com/googlepay/answer/9015738?hl=en)
- [GPay India help — view transaction history + Get statement PDF](https://support.google.com/pay/india/answer/7430307?hl=en-IN)
- [GPay India — manage recurring payments / UPI autopay](https://support.google.com/pay/india/answer/10840624?hl=en-IN)
- [Google Data Portability API](https://developers.google.com/data-portability) · [Play Store schema](https://developers.google.com/data-portability/schema-reference/play)
- [NPCI UPI AutoPay product overview](https://www.npci.org.in/what-we-do/autopay/product-overview) · [NPCI OC-223 circular (PDF)](https://www.npci.org.in/uploads/UPI_OC_No_223_FY_2025_26_Enhancement_of_UPI_Autopay_88b38535cb.pdf) · [RTI Wiki explainer](https://righttoinformation.wiki/upi-autopay-mandate-port-manage-across-apps-2026)
- [Paytm UPI statement download (PDF/Excel) — ET](https://economictimes.indiatimes.com/wealth/save/paytm-upi-statement-download-paytm-upi-users-can-now-download-expense-statement-in-pdf-excel-to-track-spending-habits/articleshow/118846903.cms)

**Credit cards / banks**
- [HDFC — cancel UPI AutoPay / mandate FAQs](https://www.hdfc.bank.in/payments/services/cancel-upi-autopay-mandate)
- [SBI Card — statement/billing FAQ](https://www.sbicard.com/en/faq/statement-billing-related.page) · [SBI Card e-mandate FAQ (PDF)](https://www.sbicard.com/sbi-card-en/assets/docs/pdf/FAQ-auto-bill-pay.pdf)
- [HDFC Bank API portal — credit card transactions (B2B)](https://developer.hdfcbank.com/credit-card-billed-unbilled-transactions)
- [Mastercard Open Banking — statement API (US/EU)](https://www.mastercard.com/us/en/business/open-finance/solutions/data/statements.html)
- [CRED reverse-engineering writeup — LinkedIn](https://www.linkedin.com/posts/shailujain_reverseengineering-apis-googlescript-activity-7247187417936384000-pD2V)
- [Statement password patterns — BankStatementLab](https://www.bankstatementlab.com/en/blog/en-download-bank-statement-sbi-hdfc-icici) · [SBI password — ClearTax](https://cleartax.in/s/sbi-statement-password) · [SBI card password — Airtel](https://www.airtel.in/blog/credit-card/what-is-my-sbi-credit-card-statement-password/)

**Account Aggregator**
- [Sahamati (SRO)](https://sahamati.org.in/) · [AA apps](https://sahamati.org.in/download-account-aggregator-apps) · [What is AA](https://sahamati.org.in/what-is-account-aggregator/)
- [State of Account Aggregator 2026 — casparser](https://casparser.in/blog/state-of-account-aggregator-2026/)
- [Setu AA docs — participants/consent flow/FIP APIs](https://docs.setu.co/data/account-aggregator/v1/licenses-and-go-live/participants-in-aa) · [Setu FIU onboarding](https://docs.setu-aa.com/fiu-onboarding) · [Setu AA product](https://setu.co/data/financial-data-apis/account-aggregator/) · [setu-client-go (SDK)](https://github.com/fintech-sdk/setu-client-go)
- [ICICI Bank AA page](https://www.icici.bank.in/account-aggregator) · [BOI FIP/FIU list](https://bankofindia.bank.in/account-aggregator)
- [RBI Master Direction on AA](https://www.rbi.org.in/Scripts/BS_ViewMasDirections.aspx?id=10598)

**Parsing**
- [xaneem/hdfc-credit-card-statement-parser](https://github.com/xaneem/hdfc-credit-card-statement-parser) · [santosh1994 parser](https://github.com/santosh1994/hdfc-creditcard-statement-parser) · [joeirimpan/hdfc-cc-parser-rs](https://github.com/joeirimpan/hdfc-cc-parser-rs) · [stmt-forge](https://github.com/madhav921/stmt-forge) · [indian-bank-statement-parser (PyPI)](https://pypi.org/project/indian-bank-statement-parser/) · [nikhilweee gist](https://gist.github.com/nikhilweee/24cae428f68c153afda495dc17ef43d6) · [SBI statement converter](https://sbt-statement-converter.streamlit.app/) · [nitinn77/credit-card-spends-tracker (Go)](https://github.com/nitinn77/credit-card-spends-tracker)

**Subscription tools**
- [Essara — best tracker app India 2026](https://essara.space/blog/best-subscription-tracker-app-india) · [LowerMySubs 8-app test](https://www.lowermysubs.com/blog/best-subscription-trackers-2026) · [Rocket Money](https://www.rocketmoney.com/) · [CNBC best trackers](https://www.cnbc.com/select/best-subscription-trackers/) · [INDmoney card tracking](https://www.indmoney.com/features/track-credit-card-bills) · [UPI Track Autopay app](https://play.google.com/store/apps/details?id=com.budrock.upitracker) · [shen-kit/finance-tracker-tui (Go)](https://github.com/shen-kit/finance-tracker-tui/)

**Regulation**
- [DPDP Act 2023 — MeitY (PDF)](https://www.meity.gov.in/static/uploads/2024/06/2bf1f0e9f04e6fb4f8fef35e82c42aa5.pdf) · [HLC summary](https://www.hlc.com/en/publications/indias-digital-personal-data-protection-act-2023-brought-into-force-)
- [DEPA — NITI Aayog draft](https://www.niti.gov.in/sites/default/files/2023-03/Data-Empowerment-and-Protection-Architecture-A-Secure-Consent-Based.pdf) · [ORF on DEPA](https://www.orfonline.org/research/data-empowerment-and-protection-architecture-concept-and-assessment)
- [RBI e-mandate circular (2019)](https://www.rbi.org.in/Scripts/NotificationUser.aspx?Id=11784) · [₹15,000 limit — The Hindu](https://www.thehindu.com/business/rbi-raises-limit-of-e-mandates-for-transactions-up-to-15000/article65534388.ece) · [Khaitan & Co note](https://www.khaitanco.com/thought-leaderships/RBI-enhances-transaction-limits-for-processing-of-e-mandates-for-recurring-transactions:-from-INR-5000-to-INR-15000)
