DROP INDEX IF EXISTS idx_users_status_created_at;
DROP INDEX IF EXISTS idx_users_phone;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS remark,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS avatar,
    DROP COLUMN IF EXISTS phone;
