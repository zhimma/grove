CREATE TABLE IF NOT EXISTS casbin_rules (
    id BIGSERIAL PRIMARY KEY,
    ptype VARCHAR(100),
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_casbin_rules_ptype ON casbin_rules (ptype);
CREATE INDEX IF NOT EXISTS idx_casbin_rules_v0 ON casbin_rules (v0);
CREATE INDEX IF NOT EXISTS idx_casbin_rules_v1 ON casbin_rules (v1);
CREATE INDEX IF NOT EXISTS idx_casbin_rules_v2 ON casbin_rules (v2);
