# HealthBot — Build Specification

**Date:** 2026-04-20  
**Author:** Aleksandr Lukashkin  
**Status:** Draft

---

## 1. Overview

Personal Telegram bot for health tracking, deployed on K3s. Single-user, self-hosted, full data ownership. Covers fasting, medication, body metrics, nutrition (with AI photo analysis), and multi-month statistics exported to both Telegram and Grafana.

---

## 2. Functional Scope

| Module       | Features                                                                  |
|--------------|---------------------------------------------------------------------------|
| Fasting      | Start/end fast, current status, duration history                          |
| Medication   | Schedule management, reminders with snooze/skip, adherence tracking       |
| Body Metrics | Weight logging, full measurements wizard (5 body parts)                   |
| Nutrition    | Meal logging via photo (OpenAI vision) or manual, macro confirmation flow |
| Statistics   | In-bot PNG charts + text summaries; Grafana dashboards via VictoriaMetrics|

---

## 3. Architecture

```
┌─────────────────────────────────────────────────────┐
│                    K3s Cluster                       │
│                                                      │
│  ┌──────────────┐    ┌─────────────────────────┐    │
│  │  healthbot   │───▶│  CloudNativePG          │    │
│  │  (Go, 1 pod) │    │  (existing cluster)     │    │
│  └──────┬───────┘    └─────────────────────────┘    │
│         │                                            │
│         │            ┌─────────────────────────┐    │
│         ├───────────▶│  VictoriaMetrics         │    │
│         │            │  (existing cluster)      │    │
│         │            └─────────────────────────┘    │
│         │                                            │
│  ┌──────▼───────┐                                    │
│  │  Scheduler   │  (goroutine, medication reminders) │
│  └──────────────┘                                    │
└─────────────────────────────────────────────────────┘
         │                        │
         ▼                        ▼
   Telegram Bot API          OpenAI API
   (long polling)            (gpt-4o vision)
```

**Design decisions:**

- Single Go binary, single pod — stateless, all state in Postgres
- Long polling (not webhook) — simpler ops, no Ingress needed
- In-process goroutine scheduler for medication reminders — no CronJob overhead for personal use
- Per-user in-memory FSM (`sync.Map`) for multi-step flows — GC'd on completion or 10m timeout
- OpenAI called inline during meal photo flow only — no background workers needed

---

## 4. Data Model

```sql
-- Single user but clean to have the table
CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    timezone    TEXT NOT NULL DEFAULT 'Europe/Amsterdam',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fasting
CREATE TABLE fasting_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ,               -- NULL = active fast
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Medications
CREATE TABLE medications (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    dosage      NUMERIC NOT NULL,
    unit        TEXT NOT NULL,             -- mg, ml, units
    frequency   TEXT NOT NULL,             -- daily, weekly
    times       TEXT[] NOT NULL,           -- ["08:00","20:00"] in user TZ
    active      BOOL NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE medication_logs (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id),
    medication_id  BIGINT NOT NULL REFERENCES medications(id),
    scheduled_at   TIMESTAMPTZ NOT NULL,
    taken_at       TIMESTAMPTZ,
    snoozed        BOOL NOT NULL DEFAULT FALSE,
    skipped        BOOL NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Body metrics
CREATE TABLE weight_logs (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    weight_kg    NUMERIC(5,2) NOT NULL
);

CREATE TABLE body_measurements (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    height_cm    NUMERIC(5,1),
    chest_cm     NUMERIC(5,1),
    waist_cm     NUMERIC(5,1),
    hips_cm      NUMERIC(5,1),
    bicep_cm     NUMERIC(5,1),
    thigh_cm     NUMERIC(5,1)
);

-- Nutrition
CREATE TABLE meal_logs (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id),
    recorded_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    meal_type        TEXT NOT NULL,        -- breakfast, lunch, dinner, snack
    photo_file_id    TEXT,                 -- Telegram file_id, NULL for manual entry
    calories         INTEGER,
    protein_g        NUMERIC(6,1),
    carbs_g          NUMERIC(6,1),
    fat_g            NUMERIC(6,1),
    ai_raw_response  JSONB,               -- full OpenAI response for debugging/reparse
    confirmed        BOOL NOT NULL DEFAULT FALSE
);
```

**Index strategy:**

```sql
CREATE INDEX ON fasting_logs (user_id, started_at DESC);
CREATE INDEX ON weight_logs (user_id, recorded_at DESC);
CREATE INDEX ON meal_logs (user_id, recorded_at DESC) WHERE confirmed = TRUE;
CREATE INDEX ON medication_logs (user_id, scheduled_at DESC);
CREATE INDEX ON body_measurements (user_id, recorded_at DESC);
```

---

## 5. Bot UX — Menu Structure

```
📱 Main Menu (ReplyKeyboard, persistent)
├── 🍽 Nutrition
│   ├── 📸 Log meal (photo)
│   │   └── [send photo] → AI analysis → inline confirm/edit/discard
│   ├── ✏️ Log meal (manual)
│   │   └── wizard: meal_type → calories → protein → carbs → fat → save
│   └── 📋 Today's summary
│
├── ⏱ Fasting
│   ├── ▶️ Start fast
│   ├── ⏹ End fast
│   └── 📍 Current status
│
├── 💊 Medications
│   ├── ✅ Take medication   (inline list of today's pending)
│   ├── ⚙️ Manage            (add / edit / deactivate)
│   └── 📋 Today's schedule
│
├── ⚖️ Body Metrics
│   ├── 🔢 Log weight
│   ├── 📏 Log measurements  (wizard: chest→waist→hips→bicep→thigh)
│   └── 📋 Last entry
│
└── 📊 Statistics
    ├── 7 days
    ├── 30 days
    └── 90 days
```

**Inline keyboard patterns:**

```
Meal photo confirmation:
[✅ Confirm]  [✏️ Edit macros]  [🗑 Discard]

Medication reminder:
[✅ Took it]  [⏰ Snooze 30m]  [⏭ Skip]

Measurements wizard step:
[value input] then [➡️ Next] [⏭ Skip field] [❌ Cancel]
```

---

## 6. Project Structure

```
healthbot/
├── cmd/
│   └── bot/
│       └── main.go
├── internal/
│   ├── bot/
│   │   ├── handler.go          # tgbotapi update router
│   │   ├── middleware.go       # telegram_id whitelist auth
│   │   ├── keyboards.go        # ReplyKeyboard + InlineKeyboard builders
│   │   └── fsm.go              # per-user state machine (sync.Map)
│   ├── modules/
│   │   ├── fasting/
│   │   │   ├── handler.go
│   │   │   └── service.go
│   │   ├── nutrition/
│   │   │   ├── handler.go
│   │   │   ├── service.go
│   │   │   └── openai.go       # photo → macros wrapper
│   │   ├── medication/
│   │   │   ├── handler.go
│   │   │   ├── service.go
│   │   │   └── reminder.go     # goroutine scheduler
│   │   ├── metrics/
│   │   │   ├── handler.go
│   │   │   └── service.go
│   │   └── stats/
│   │       ├── handler.go
│   │       ├── service.go
│   │       └── charts.go       # SVG chart rendering
│   ├── db/
│   │   ├── migrations/         # embedded via embed.FS
│   │   │   ├── 001_init.up.sql
│   │   │   └── 001_init.down.sql
│   │   └── queries/            # hand-written pgx queries
│   │       ├── users.go
│   │       ├── fasting.go
│   │       ├── medication.go
│   │       ├── metrics.go
│   │       └── nutrition.go
│   ├── vm/
│   │   └── client.go           # Prometheus remote write to VictoriaMetrics
│   └── config/
│       └── config.go           # viper, env-first
├── sqlc.yaml
├── Dockerfile
├── k8s/
│   ├── namespace.yaml
│   ├── deployment.yaml
│   ├── secret.yaml
│   └── configmap.yaml
├── grafana/
│   └── healthbot-dashboard.json
└── doc/
    ├── spec.md                 # this file
    └── plan.md                 # implementation plan
```

---

## 7. Key Libraries

| Concern    | Library                                   |
|------------|-------------------------------------------|
| Telegram   | `go-telegram-bot-api/telegram-bot-api/v5` |
| DB driver  | `jackc/pgx/v5` with pgxpool               |
| Migrations | `golang-migrate/migrate` (embedded FS)    |
| Charts     | SVG rendered in pure Go                   |
| Config     | `spf13/viper`                             |
| OpenAI     | `sashabaranov/go-openai`                  |
| Metrics    | `prometheus/client_golang` (remote write) |

---

## 8. OpenAI Meal Photo Flow

```
User sends photo
       │
       ▼
Download from Telegram → base64 encode
       │
       ▼
POST /v1/chat/completions (gpt-4o)
  model: gpt-4o
  messages:
    - role: system
      content: "You are a nutrition analyst. Respond ONLY with JSON:
                {\"calories\":int,\"protein_g\":float,
                 \"carbs_g\":float,\"fat_g\":float,\"meal_name\":string}"
    - role: user
      content: [image/base64, "Estimate macros for this meal"]
       │
       ▼
Parse JSON response → store ai_raw_response
       │
       ▼
Bot sends estimate with inline keyboard:
  "🍝 Estimated: 650 kcal | P:28g C:82g F:18g
   [✅ Confirm] [✏️ Edit] [🗑 Discard]"
       │
  ┌────┴────┐
  ▼         ▼
Confirm   Edit (re-enter macros manually)
  │         │
  └────┬────┘
       ▼
  meal_logs INSERT (confirmed=true)
  VM metrics push
```

---

## 9. Medication Reminder Scheduler

```go
// On bot startup: load all active medications,
// schedule next reminder for each

func (s *Scheduler) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case <-ticker.C:
            s.checkAndSend(ctx)
        case <-ctx.Done():
            return
        }
    }
}

// checkAndSend: for each medication, check if current
// time matches any scheduled time (±1min window)
// if yes and no medication_log exists for today's slot:
//   INSERT medication_log (scheduled_at, taken_at=NULL)
//   Send Telegram message with inline keyboard
```

Snooze: update `scheduled_at += 30m`, re-queue.  
Skip: set `skipped=true`.  
Confirm: set `taken_at=NOW()`.

---

## 10. VictoriaMetrics Metrics

Pushed via Prometheus remote write on every successful log:

```
healthbot_weight_kg                          gauge
healthbot_fast_duration_hours                gauge
healthbot_meal_calories_total                counter (+ meal_type label)
healthbot_meal_protein_g                     gauge
healthbot_meal_carbs_g                       gauge
healthbot_meal_fat_g                         gauge
healthbot_medication_event_total             counter (status=taken|skipped|snoozed)
healthbot_body_measurement_cm{part=...}      gauge (waist, chest, hips, bicep, thigh)
```

All metrics carry `user="asl"` label.

---

## 11. K8s Deployment

```yaml
# deployment.yaml (key fields)
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: healthbot
          image: registry.nebius.local/healthbot:latest
          envFrom:
            - secretRef:
                name: healthbot-secrets
            - configMapRef:
                name: healthbot-config
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
```

```yaml
# secret.yaml keys
BOT_TOKEN: <telegram bot token>
OPENAI_API_KEY: <openai key>
DB_DSN: postgresql://healthbot:pass@cnpg-rw:5432/healthbot
VM_REMOTE_WRITE_URL: http://victoria-metrics:8428/api/v1/write

# configmap.yaml keys
TZ: Europe/Amsterdam
ALLOWED_TELEGRAM_ID: "123456789"
```

---

## 12. Statistics — In-Bot Charts

Per time range (7d / 30d / 90d), bot sends SVG document + text summary:

| Chart                | Type          | Data                                    |
|----------------------|---------------|-----------------------------------------|
| Weight trend         | Line          | weight_kg over time                     |
| Fasting windows      | Bar           | fast duration per day                   |
| Daily calories       | Bar           | calories per day (confirmed meals only) |
| Macro split          | Text          | protein/carbs/fat avg per day           |
| Medication adherence | Text %        | taken/(taken+skipped) per medication    |
| Measurements delta   | Text table    | first vs last in period, Δ              |

Charts rendered server-side in pure Go SVG → sent as Telegram document.

---

## 13. Out of Scope (v1)

- Multi-user support
- Body fat % / HRV / sleep tracking
- Barcode scanning for packaged food
- Weekly/monthly report auto-send
- Data export (CSV/JSON)
- Web UI

---

## 14. Implementation Phases

| Phase | Scope                                                       | Effort |
|-------|-------------------------------------------------------------|--------|
| 1     | Project scaffold, DB migrations, auth middleware, main menu | 1–2d   |
| 2     | Fasting + weight modules (simplest flows)                   | 1d     |
| 3     | Medication module + scheduler                               | 2d     |
| 4     | Nutrition module + OpenAI integration                       | 2d     |
| 5     | Body measurements wizard                                    | 1d     |
| 6     | Statistics (in-bot charts + VM push)                        | 2d     |
| 7     | K8s manifests, Grafana dashboard, e2e test                  | 1d     |

**Total: ~10 days of focused work**
