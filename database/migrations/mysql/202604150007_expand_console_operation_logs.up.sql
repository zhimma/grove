ALTER TABLE console_operation_logs
    ADD COLUMN target_type VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN target_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN detail_json TEXT NOT NULL;

CREATE INDEX idx_console_operation_logs_target_type ON console_operation_logs (target_type);
CREATE INDEX idx_console_operation_logs_target_id ON console_operation_logs (target_id);
