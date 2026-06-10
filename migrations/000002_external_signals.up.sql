CREATE TABLE external_signals (
    id            BIGSERIAL   PRIMARY KEY,
    signal_name   TEXT        NOT NULL,
    hospital_id   TEXT        REFERENCES hospitals(id),
    value         DOUBLE PRECISION,
    raw_json      JSONB,
    observed_at   TIMESTAMPTZ NOT NULL,
    scraped_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ext_sig_name_observed ON external_signals (signal_name, observed_at DESC);
CREATE INDEX idx_ext_sig_hospital_observed ON external_signals (hospital_id, observed_at DESC) WHERE hospital_id IS NOT NULL;
