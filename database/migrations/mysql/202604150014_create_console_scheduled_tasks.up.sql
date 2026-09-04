-- Schedule parameters only. The task body lives in the Worker's code registry,
-- keyed by name, so a row can never introduce a task the binary cannot run.
CREATE TABLE IF NOT EXISTS console_scheduled_tasks (
    id VARCHAR(26) PRIMARY KEY,
    name VARCHAR(120) NOT NULL UNIQUE,
    display_name VARCHAR(160) NOT NULL DEFAULT '',
    schedule VARCHAR(120) NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    mutex TINYINT(1) NOT NULL DEFAULT 1,
    timeout_seconds INT NOT NULL DEFAULT 0,
    run_requested_at DATETIME(6) NULL,
    last_run_at DATETIME(6) NULL,
    last_status VARCHAR(20) NOT NULL DEFAULT '',
    last_error VARCHAR(1000) NOT NULL DEFAULT '',
    last_duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX idx_console_scheduled_tasks_enabled (enabled),
    INDEX idx_console_scheduled_tasks_run_requested_at (run_requested_at),
    CONSTRAINT chk_console_scheduled_tasks_timeout
        CHECK (timeout_seconds >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
