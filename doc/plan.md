# HealthBot — Implementation Plan

**Date:** 2026-04-20  
**Branch:** `claude/build-healthbot-telegram-AePzd`

---

## Phase 1 — Project Scaffold, Config, DB, Auth, Main Menu

**Goal:** Working skeleton that connects to Postgres, passes auth, and shows the main menu.

### Tasks
- [x] Initialize Go module (`github.com/kerhoff/healthbot`)
- [x] Add all dependencies (tgbotapi, pgx, golang-migrate, viper, openai, prometheus, go-echarts)
- [x] `internal/config/config.go` — viper env-first config; required: `BOT_TOKEN`, `DB_DSN`
- [x] `internal/db/migrations/001_init.up.sql` — all 7 tables + indexes
- [x] `internal/db/db.go` — pgxpool connect + golang-migrate with embedded FS
- [x] `internal/bot/fsm.go` — per-user `sync.Map` FSM with 10-minute GC
- [x] `internal/bot/middleware.go` — `ALLOWED_TELEGRAM_ID` whitelist check
- [x] `internal/bot/keyboards.go` — all ReplyKeyboard and InlineKeyboard builders
- [x] `internal/bot/handler.go` — update router dispatching by FSM state then by button text
- [x] `cmd/bot/main.go` — migrations → pool → bot → scheduler → healthz → long-poll loop

### Key decisions
- FSM state checked before button text — multi-step wizards take priority over menu commands
- `ALLOWED_TELEGRAM_ID=0` disables auth (useful for local dev)
- `/healthz` runs on `:8080` for K8s liveness probe

---

## Phase 2 — Fasting + Weight Modules

**Goal:** Users can start/end/check fasts and log weight.

### Tasks
- [x] `internal/db/queries/fasting.go` — StartFast, EndFast, GetActiveFast, GetFastingLogs
- [x] `internal/db/queries/metrics.go` — InsertWeight, GetWeightLogs, GetLastWeight, InsertBodyMeasurement, GetLastBodyMeasurement, GetBodyMeasurements
- [x] `internal/modules/fasting/service.go` — business logic, duration formatting, VM push on end
- [x] `internal/modules/fasting/handler.go` — HandleStart / HandleEnd / HandleStatus
- [x] `internal/modules/metrics/service.go` — LogWeight (validates input), GetLastEntry, SaveBodyMeasurement
- [x] `internal/modules/metrics/handler.go` — weight input flow, measurements wizard steps

### FSM states used
- `weight_input` — waiting for kg value
- `measure_chest/waist/hips/bicep/thigh` — wizard steps (Cancel at any step)

---

## Phase 3 — Medication Module + Scheduler

**Goal:** Users can manage medications; bot sends timed reminders.

### Tasks
- [x] `internal/db/queries/medication.go` — full CRUD + MedLogExists, GetPendingMedLogs, GetTodayMedSchedule, SnoozeMedication, TakeMedication, SkipMedication
- [x] `internal/modules/medication/service.go` — AddMedication, TakeMed/SnoozeMed/SkipMed (push VM event each), GetTodaySchedule
- [x] `internal/modules/medication/handler.go` — 5-step add wizard, manage list with deactivate inline, today's schedule
- [x] `internal/modules/medication/reminder.go` — `Scheduler.Run` goroutine; checks every minute; inserts `medication_log` + sends inline reminder if within ±1min window and no log exists

### Inline callbacks
- `med_took:<id>` / `med_take:<id>` → TakeMedication
- `med_snooze:<id>` → SnoozeMedication (scheduled_at += 30m)
- `med_skip:<id>` → SkipMedication
- `med_deactivate:<id>` → DeactivateMedication
- `med_add_new` → starts add wizard

### Scheduler notes
- Runs as goroutine from `main.go`, cancelled via context on SIGTERM
- Only fires if `ALLOWED_TELEGRAM_ID != 0` (uses it as both userID and chatID for single-user setup)
- `MedLogExists` uses a ±1 minute window to prevent duplicate entries

---

## Phase 4 — Nutrition Module + OpenAI Integration

**Goal:** Users can log meals by photo (AI analysis) or manually.

### Tasks
- [x] `internal/db/queries/nutrition.go` — InsertMealLog, GetMealLog, ConfirmMealLog, UpdateMealLogMacros, DeleteMealLog, GetTodayMeals, GetMealLogs
- [x] `internal/modules/nutrition/openai.go` — `AnalyzeMealPhoto`: downloads image, base64 encodes, calls gpt-4o with vision, parses JSON response
- [x] `internal/modules/nutrition/service.go` — AnalyzePhoto (download + AI), LogManual, ConfirmMeal, DiscardMeal, UpdateMacros, TodaySummary
- [x] `internal/modules/nutrition/handler.go` — photo flow (meal type → send photo → AI → inline), manual wizard (meal type → cal → protein → carbs → fat), edit flow from inline callback

### Photo flow FSM states
- `nutrition_photo_meal_type` — waiting for meal type selection
- `nutrition_photo_wait` — waiting for photo to arrive

### Manual flow FSM states
- `nutrition_meal_type` → `nutrition_calories` → `nutrition_protein` → `nutrition_carbs` → `nutrition_fat`

### Inline callbacks
- `meal_confirm:<id>` → ConfirmMealLog + VM push
- `meal_edit:<id>` → starts edit sub-wizard (reuses calories/protein/carbs/fat states with `edit_log_id` in FSM data)
- `meal_discard:<id>` → DeleteMealLog

---

## Phase 5 — Body Measurements Wizard

**Goal:** Users step through 5 body parts; each can be skipped.

### Tasks
- [x] Wizard states in `fsm.go`: `measure_chest/waist/hips/bicep/thigh/done`
- [x] `HandleLogMeasurements` in `metrics/handler.go` — starts wizard
- [x] `HandleMeasurementStep` — advances through states; on last step calls `SaveBodyMeasurement`; Cancel at any step aborts and returns to menu

### Skip behaviour
- User sends "⏭ Skip" or "skip" → field omitted from `BodyMeasurement` (stored as NULL)

---

## Phase 6 — Statistics + Charts + VictoriaMetrics

**Goal:** `/stats` shows text summary + SVG charts for 7/30/90 day ranges; all events push to VM.

### Tasks
- [x] `internal/modules/stats/service.go` — `Compute` aggregates weight/fasting/meals/meds/measurements into `Summary`; `TextSummary` formats markdown; `GetWeightSeries`, `GetFastingSeries`, `GetCaloriesSeries` return time series for charts
- [x] `internal/modules/stats/charts.go` — pure-Go SVG rendering: `renderLineSVG` (weight), `renderBarSVG` (fasting, calories); dark theme (#1e1e2e), labeled axes, ±6 X-axis ticks
- [x] `internal/modules/stats/handler.go` — sends text summary then weight/fasting/calorie SVG documents
- [x] `internal/vm/client.go` — Prometheus remote write via `prometheus/client_golang/prometheus/push`; PushWeight, PushFast, PushMeal, PushMedication, PushBodyMeasurement

### VM push pattern
- All pushes are `go func() { _ = vm.Push...() }()` — fire-and-forget, never blocks user response
- `VM_REMOTE_WRITE_URL=""` silently no-ops all pushes (safe for dev without VM)

---

## Phase 7 — K8s Manifests + Grafana Dashboard

**Goal:** Ready-to-deploy K8s resources and a usable Grafana dashboard.

### Tasks
- [x] `k8s/namespace.yaml` — `healthbot` namespace
- [x] `k8s/deployment.yaml` — 1 replica, resource limits (50m/64Mi req, 200m/128Mi limit), liveness + readiness probes on `/healthz:8080`, `envFrom` secret + configmap
- [x] `k8s/secret.yaml` — `BOT_TOKEN`, `OPENAI_API_KEY`, `DB_DSN`, `VM_REMOTE_WRITE_URL`
- [x] `k8s/configmap.yaml` — `TZ`, `ALLOWED_TELEGRAM_ID`, `HEALTHZ_PORT`
- [x] `grafana/healthbot-dashboard.json` — 6 panels: Weight (timeseries), Fasting Duration (timeseries), Daily Calories (timeseries with increase()), Macros P/C/F (timeseries), Medication Events taken/skipped (timeseries), Body Measurements by part (timeseries)
- [x] `Dockerfile` — multi-stage: `golang:1.24-alpine` builder → `distroless/static:nonroot` runtime; CGO disabled

### Deploy steps
```bash
# 1. Fill in real values in k8s/secret.yaml
# 2. Build and push image
docker build -t registry.nebius.local/healthbot:latest .
docker push registry.nebius.local/healthbot:latest

# 3. Apply manifests
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml

# 4. Import Grafana dashboard
# Upload grafana/healthbot-dashboard.json via Grafana UI → Dashboards → Import
```

---

## Environment Variables Reference

| Variable              | Required | Default            | Description                              |
|-----------------------|----------|--------------------|------------------------------------------|
| `BOT_TOKEN`           | yes      | —                  | Telegram bot token from @BotFather       |
| `DB_DSN`              | yes      | —                  | PostgreSQL DSN (pgx format)              |
| `OPENAI_API_KEY`      | no       | —                  | Required for photo meal analysis         |
| `VM_REMOTE_WRITE_URL` | no       | —                  | VictoriaMetrics remote write endpoint    |
| `ALLOWED_TELEGRAM_ID` | no       | 0 (allow all)      | Telegram user ID whitelist (single user) |
| `TZ`                  | no       | `Europe/Amsterdam` | Timezone for medication scheduler        |
| `HEALTHZ_PORT`        | no       | `8080`             | Port for `/healthz` liveness endpoint    |

---

## Out of Scope (v1)

- Multi-user support
- Body fat % / HRV / sleep tracking
- Barcode scanning for packaged food
- Weekly/monthly report auto-send
- Data export (CSV/JSON)
- Web UI
