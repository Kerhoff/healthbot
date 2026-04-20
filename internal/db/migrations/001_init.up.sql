CREATE TABLE IF NOT EXISTS users (
    id          BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    timezone    TEXT NOT NULL DEFAULT 'Europe/Amsterdam',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fasting_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS medications (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    dosage      NUMERIC NOT NULL,
    unit        TEXT NOT NULL,
    frequency   TEXT NOT NULL,
    times       TEXT[] NOT NULL,
    active      BOOL NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS medication_logs (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id),
    medication_id  BIGINT NOT NULL REFERENCES medications(id),
    scheduled_at   TIMESTAMPTZ NOT NULL,
    taken_at       TIMESTAMPTZ,
    snoozed        BOOL NOT NULL DEFAULT FALSE,
    skipped        BOOL NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS weight_logs (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    weight_kg    NUMERIC(5,2) NOT NULL
);

CREATE TABLE IF NOT EXISTS body_measurements (
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

CREATE TABLE IF NOT EXISTS meal_logs (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id),
    recorded_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    meal_type        TEXT NOT NULL,
    photo_file_id    TEXT,
    calories         INTEGER,
    protein_g        NUMERIC(6,1),
    carbs_g          NUMERIC(6,1),
    fat_g            NUMERIC(6,1),
    ai_raw_response  JSONB,
    confirmed        BOOL NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_fasting_logs_user_started ON fasting_logs (user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_weight_logs_user_recorded ON weight_logs (user_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_meal_logs_user_recorded_confirmed ON meal_logs (user_id, recorded_at DESC) WHERE confirmed = TRUE;
CREATE INDEX IF NOT EXISTS idx_medication_logs_user_scheduled ON medication_logs (user_id, scheduled_at DESC);
CREATE INDEX IF NOT EXISTS idx_body_measurements_user_recorded ON body_measurements (user_id, recorded_at DESC);
