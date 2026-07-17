# ClassKhata — Coaching-Class Manager

Fees, attendance and parent communication for Indian tuition and coaching
centres, in one ledger ("khata"). A single Go binary serves both the JSON API
and an embedded web UI — no database, no CDN, fully offline-capable.

**Who pays:** institute owners, at **₹799/month per institute**. The product
replaces the paper bahi-khata + a personal WhatsApp number: monthly dues are
generated automatically per enrollment, partial cash/UPI payments stay on the
ledger, absences and overdue fees turn into ready-to-send bilingual WhatsApp
messages.

Everything is anchored to a fixed "today" — **Friday, 17 July 2026** — so
dues math, the dashboard and demo data are fully deterministic.

## Quickstart

```bash
make run          # serves http://localhost:8105
make test         # domain-logic tests
make build        # bin/classkhata
```

Open http://localhost:8105 and press **Load demo data** (or
`curl -X POST localhost:8105/api/v1/demo`) to seed 2 batches, 14 students,
four months of dues with a realistic paid / partial / overdue mix, and this
week's attendance registers.

State persists as a JSON snapshot in `./data/store.json` (written after every
change, loaded on boot).

## API summary (`/api/v1`)

| Method & path | What it does |
| --- | --- |
| `GET /health` | `{"status":"ok"}` + provider mode + anchor date |
| `GET/POST /batches`, `GET/PUT/DELETE /batches/{id}` | Batch CRUD (name, subject, days, times, monthly fee ₹) |
| `GET/POST /students`, `GET/PUT/DELETE /students/{id}` | Student CRUD (parent name + `+91` phone) |
| `GET/POST /enrollments`, `DELETE /enrollments/{id}` | Enroll student↔batch from a joining month; dues auto-generate |
| `GET /attendance?batchId=&date=` | Roll-call grid with existing marks and per-student % |
| `POST /attendance` | Save a register idempotently; new absences queue parent alerts |
| `GET /dues` | Full fee ledger: per-month fee, paid, balance, status |
| `POST /dues/generate` | Regenerate missing dues (idempotent, never duplicates) |
| `GET/POST /payments` | Record cash/UPI payments; partial amounts allowed |
| `POST /reminders/fees` | Queue one bilingual reminder per enrollment with a balance |
| `POST /announcements` | Broadcast to a batch — one personalized message per parent |
| `GET /outbox` | The mock WhatsApp outbox, newest first |
| `GET /dashboard` | Collections this month ₹, outstanding ₹, active students, today's batches, week attendance, recent payments |
| `POST /demo` | Reset to the deterministic demo institute |

## Upgrade to live integrations

Zero keys are needed to run. Where a live integration would exist, a small
`core.Provider` interface is defined and only the deterministic mock ships
(message ids are FNV hashes of recipient+body, so identical input always
yields identical output).

| Env var | Default | Switches |
| --- | --- | --- |
| `PORT` | `8105` | HTTP port |
| `WHATSAPP_PROVIDER` | `mock` | `live` would send via the WhatsApp Business Cloud API (not shipped in MVP; falls back to mock with a log notice) |
| `WHATSAPP_TOKEN` | — | Cloud API access token, used only in live mode |
| `WHATSAPP_PHONE_NUMBER_ID` | — | Sender phone-number id, used only in live mode |

## Layout

```
cmd/server        main: wiring, PORT, provider selection
internal/core     domain: store, fees engine, attendance, seed, messages
internal/api      HTTP handlers (Go 1.22 pattern routing)
web               embedded UI (vanilla HTML/CSS/JS, go:embed)
data/store.json   runtime snapshot (gitignored)
```

Tests cover the money math and rules engine: dues idempotency across
regenerations, partial-payment outstanding (never negative), attendance %
computation, absence-alert idempotency, INR formatting and demo-seed
determinism — `go test ./...`.
