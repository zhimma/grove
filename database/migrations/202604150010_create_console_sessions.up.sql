CREATE TABLE IF NOT EXISTS console_sessions (
    id VARCHAR(26) PRIMARY KEY,
    admin_id VARCHAR(64) NOT NULL,
    refresh_token_hash CHAR(64) NOT NULL,
    device_name VARCHAR(120) NOT NULL DEFAULT '',
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(500) NOT NULL DEFAULT '',
    last_active_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    revoke_reason VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_console_sessions_admin
        FOREIGN KEY (admin_id)
        REFERENCES console_admins (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_console_sessions_expiry
        CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_console_sessions_refresh_token_hash
    ON console_sessions (refresh_token_hash);

CREATE INDEX IF NOT EXISTS idx_console_sessions_admin_active
    ON console_sessions (admin_id, last_active_at DESC)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_console_sessions_expires_at
    ON console_sessions (expires_at);
