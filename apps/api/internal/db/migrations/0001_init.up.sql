CREATE SCHEMA IF NOT EXISTS jiaohao;
SET search_path TO jiaohao, public;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL CHECK (role IN ('diner', 'staff', 'admin')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    token_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE canteens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    sort INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE floors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canteen_id UUID NOT NULL REFERENCES canteens (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sort INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX floors_canteen_id_idx ON floors (canteen_id);

CREATE TABLE windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canteen_id UUID NOT NULL REFERENCES canteens (id) ON DELETE CASCADE,
    floor_id UUID REFERENCES floors (id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'pause_take', 'closed')),
    skip_timeout_seconds INTEGER NOT NULL DEFAULT 0 CHECK (skip_timeout_seconds >= 0),
    blurb TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    display_token_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (canteen_id, code)
);

CREATE INDEX windows_canteen_id_idx ON windows (canteen_id);
CREATE INDEX windows_floor_id_idx ON windows (floor_id);

CREATE TABLE staff_window_grants (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    window_id UUID NOT NULL REFERENCES windows (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, window_id)
);

CREATE TABLE window_day_counters (
    window_id UUID NOT NULL REFERENCES windows (id) ON DELETE CASCADE,
    business_date DATE NOT NULL,
    next_number INTEGER NOT NULL DEFAULT 1 CHECK (next_number >= 1),
    PRIMARY KEY (window_id, business_date)
);

CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    window_id UUID NOT NULL REFERENCES windows (id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    business_date DATE NOT NULL,
    number INTEGER NOT NULL CHECK (number >= 1),
    status TEXT NOT NULL CHECK (status IN ('waiting', 'called', 'completed', 'skipped', 'cancelled', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    called_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    skip_after TIMESTAMPTZ,
    cancel_reason TEXT,
    announcement_id TEXT,
    UNIQUE (window_id, business_date, number)
);

CREATE UNIQUE INDEX tickets_one_active_per_user
    ON tickets (user_id)
    WHERE status IN ('waiting', 'called');

CREATE INDEX tickets_window_day_status_idx
    ON tickets (window_id, business_date, status, number);
