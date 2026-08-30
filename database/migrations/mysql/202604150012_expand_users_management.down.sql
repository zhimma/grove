DROP INDEX idx_users_status_created_at ON users;
DROP INDEX idx_users_phone ON users;

ALTER TABLE users
    DROP COLUMN last_login_at,
    DROP COLUMN remark,
    DROP COLUMN status,
    DROP COLUMN avatar,
    DROP COLUMN phone;
