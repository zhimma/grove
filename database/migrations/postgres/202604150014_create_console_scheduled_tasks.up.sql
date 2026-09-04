-- Schedule parameters only. The task body lives in the Worker's code registry,
-- keyed by name, so a row can never introduce a task the binary cannot run.
CREATE TABLE IF NOT EXISTS console_scheduled_tasks (
    id VARCHAR(26) PRIMARY KEY,
    name VARCHAR(120) NOT NULL UNIQUE,
    display_name VARCHAR(160) NOT NULL DEFAULT '',
    schedule VARCHAR(120) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    mutex BOOLEAN NOT NULL DEFAULT TRUE,
    timeout_seconds INTEGER NOT NULL DEFAULT 0,
    run_requested_at TIMESTAMPTZ NULL,
    last_run_at TIMESTAMPTZ NULL,
    last_status VARCHAR(20) NOT NULL DEFAULT '',
    last_error VARCHAR(1000) NOT NULL DEFAULT '',
    last_duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_console_scheduled_tasks_timeout
        CHECK (timeout_seconds >= 0)
);

CREATE INDEX IF NOT EXISTS idx_console_scheduled_tasks_enabled ON console_scheduled_tasks(enabled);
CREATE INDEX IF NOT EXISTS idx_console_scheduled_tasks_run_requested_at ON console_scheduled_tasks(run_requested_at);
