-- Migration 085: IP geolocation cache used by the live globe dashboard.
--
-- Each row maps a client IP to a coarse geo (country + city + lat/lng).
-- The globe service backfills this table by batching uncached IPs from
-- usage_logs against ip-api.com's free /batch endpoint, then re-uses the
-- cache for the lifetime of that IP. We deliberately keep this table
-- separate from usage_logs so the (very hot) usage_logs table never has
-- to JOIN to anything heavyweight at write time.

CREATE TABLE IF NOT EXISTS ip_geo_cache (
    ip            VARCHAR(45) PRIMARY KEY,
    country       VARCHAR(80),
    country_code  VARCHAR(8),
    region        VARCHAR(80),
    city          VARCHAR(120),
    lat           DOUBLE PRECISION,
    lng           DOUBLE PRECISION,
    asn           VARCHAR(32),
    isp           VARCHAR(160),
    status        VARCHAR(16) NOT NULL DEFAULT 'ok',
    looked_up_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ip_geo_cache_country_code ON ip_geo_cache(country_code);
CREATE INDEX IF NOT EXISTS idx_ip_geo_cache_looked_up_at ON ip_geo_cache(looked_up_at);
