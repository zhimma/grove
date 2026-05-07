ALTER TABLE console_operation_logs
    ADD COLUMN IF NOT EXISTS target_type VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS detail_json TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_console_operation_logs_target_type ON console_operation_logs (target_type);
CREATE INDEX IF NOT EXISTS idx_console_operation_logs_target_id ON console_operation_logs (target_id);
