ALTER TABLE users
    ADD COLUMN phone VARCHAR(32) NOT NULL DEFAULT '' AFTER email,
    ADD COLUMN avatar VARCHAR(255) NOT NULL DEFAULT '' AFTER phone,
    ADD COLUMN status TINYINT NOT NULL DEFAULT 1 AFTER avatar,
    ADD COLUMN remark VARCHAR(500) NOT NULL DEFAULT '' AFTER status,
    ADD COLUMN last_login_at DATETIME(6) NULL AFTER remark;

CREATE INDEX idx_users_phone ON users (phone);
CREATE INDEX idx_users_status_created_at ON users (status, created_at);
