CREATE TABLE IF NOT EXISTS console_operation_logs (
    id VARCHAR(64) PRIMARY KEY,
    admin_id VARCHAR(26) NOT NULL DEFAULT '',
    method VARCHAR(12) NOT NULL,
    path VARCHAR(255) NOT NULL,
    route VARCHAR(255) NOT NULL,
    module VARCHAR(120) NOT NULL,
    action VARCHAR(180) NOT NULL,
    request_id VARCHAR(120) NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 200,
    success BOOLEAN NOT NULL DEFAULT TRUE,
    error_message VARCHAR(500) NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(500) NOT NULL DEFAULT '',
    request_query TEXT NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL
);

CREATE INDEX idx_console_operation_logs_admin_id ON console_operation_logs (admin_id);
CREATE INDEX idx_console_operation_logs_method ON console_operation_logs (method);
CREATE INDEX idx_console_operation_logs_path ON console_operation_logs (path);
CREATE INDEX idx_console_operation_logs_route ON console_operation_logs (route);
CREATE INDEX idx_console_operation_logs_module ON console_operation_logs (module);
CREATE INDEX idx_console_operation_logs_request_id ON console_operation_logs (request_id);
CREATE INDEX idx_console_operation_logs_success_created_at ON console_operation_logs (success, created_at);

CREATE TABLE IF NOT EXISTS console_login_logs (
    id VARCHAR(64) PRIMARY KEY,
    admin_id VARCHAR(26) NOT NULL DEFAULT '',
    account VARCHAR(120) NOT NULL,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    failure_reason VARCHAR(255) NOT NULL DEFAULT '',
    request_id VARCHAR(120) NOT NULL DEFAULT '',
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL
);

CREATE INDEX idx_console_login_logs_admin_id ON console_login_logs (admin_id);
CREATE INDEX idx_console_login_logs_account ON console_login_logs (account);
CREATE INDEX idx_console_login_logs_success_created_at ON console_login_logs (success, created_at);
