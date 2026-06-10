CREATE TABLE hospitals (
    id           TEXT PRIMARY KEY,
    name         TEXT        NOT NULL,
    address      TEXT,
    facility_type TEXT       NOT NULL,
    source_url   TEXT,
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE wait_time_snapshots (
    id           BIGSERIAL   PRIMARY KEY,
    hospital_id  TEXT        NOT NULL REFERENCES hospitals(id),
    wait_minutes INT         NOT NULL,
    category     TEXT        NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL,
    scraped_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Covers the most common query pattern: latest snapshot per hospital.
CREATE INDEX idx_wts_hospital_scraped ON wait_time_snapshots (hospital_id, scraped_at DESC);

-- Covers time-range history queries and anomaly window scans.
CREATE INDEX idx_wts_hospital_recorded ON wait_time_snapshots (hospital_id, recorded_at DESC);
