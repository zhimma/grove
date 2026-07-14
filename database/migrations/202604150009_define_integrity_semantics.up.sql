ALTER TABLE console_operation_logs
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE console_login_logs
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_email_key;

ALTER TABLE console_roles
    DROP CONSTRAINT IF EXISTS console_roles_code_key;

ALTER TABLE console_admins
    DROP CONSTRAINT IF EXISTS console_admins_account_key;

DROP INDEX IF EXISTS idx_console_admins_email_non_empty;
DROP INDEX IF EXISTS idx_console_admins_phone_non_empty;
DROP INDEX IF EXISTS idx_system_configs_group_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active
    ON users (email)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_console_roles_code_active
    ON console_roles (code)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_console_admins_account_active
    ON console_admins (account)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_console_admins_email_active
    ON console_admins (email)
    WHERE deleted_at IS NULL AND email IS NOT NULL AND email <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_console_admins_phone_active
    ON console_admins (phone)
    WHERE deleted_at IS NULL AND phone <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_system_configs_group_key
    ON system_configs (config_group, config_key)
    WHERE deleted_at IS NULL;

ALTER TABLE console_admins
    ADD CONSTRAINT fk_console_admins_role
    FOREIGN KEY (role_id)
    REFERENCES console_roles (id)
    ON UPDATE CASCADE
    ON DELETE RESTRICT;

ALTER TABLE console_admins
    ADD CONSTRAINT chk_console_admins_status
    CHECK (status IN (0, 1, 2));

ALTER TABLE console_roles
    ADD CONSTRAINT chk_console_roles_status
    CHECK (status IN (0, 1));

ALTER TABLE system_configs
    ADD CONSTRAINT chk_system_configs_value_type
    CHECK (value_type IN ('string', 'array', 'int', 'bool', 'json'));

UPDATE casbin_rules
SET
    ptype = COALESCE(ptype, ''),
    v0 = COALESCE(v0, ''),
    v1 = COALESCE(v1, ''),
    v2 = COALESCE(v2, ''),
    v3 = COALESCE(v3, ''),
    v4 = COALESCE(v4, ''),
    v5 = COALESCE(v5, '');

DELETE FROM casbin_rules current_rule
USING casbin_rules duplicate_rule
WHERE current_rule.id > duplicate_rule.id
  AND (current_rule.ptype, current_rule.v0, current_rule.v1, current_rule.v2, current_rule.v3, current_rule.v4, current_rule.v5)
      = (duplicate_rule.ptype, duplicate_rule.v0, duplicate_rule.v1, duplicate_rule.v2, duplicate_rule.v3, duplicate_rule.v4, duplicate_rule.v5);

ALTER TABLE casbin_rules
    ALTER COLUMN ptype SET DEFAULT '',
    ALTER COLUMN ptype SET NOT NULL,
    ALTER COLUMN v0 SET DEFAULT '',
    ALTER COLUMN v0 SET NOT NULL,
    ALTER COLUMN v1 SET DEFAULT '',
    ALTER COLUMN v1 SET NOT NULL,
    ALTER COLUMN v2 SET DEFAULT '',
    ALTER COLUMN v2 SET NOT NULL,
    ALTER COLUMN v3 SET DEFAULT '',
    ALTER COLUMN v3 SET NOT NULL,
    ALTER COLUMN v4 SET DEFAULT '',
    ALTER COLUMN v4 SET NOT NULL,
    ALTER COLUMN v5 SET DEFAULT '',
    ALTER COLUMN v5 SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_casbin_rules_unique
    ON casbin_rules (ptype, v0, v1, v2, v3, v4, v5);

UPDATE console_casbin_rules
SET
    ptype = COALESCE(ptype, ''),
    v0 = COALESCE(v0, ''),
    v1 = COALESCE(v1, ''),
    v2 = COALESCE(v2, ''),
    v3 = COALESCE(v3, ''),
    v4 = COALESCE(v4, ''),
    v5 = COALESCE(v5, '');

DELETE FROM console_casbin_rules current_rule
USING console_casbin_rules duplicate_rule
WHERE current_rule.id > duplicate_rule.id
  AND (current_rule.ptype, current_rule.v0, current_rule.v1, current_rule.v2, current_rule.v3, current_rule.v4, current_rule.v5)
      = (duplicate_rule.ptype, duplicate_rule.v0, duplicate_rule.v1, duplicate_rule.v2, duplicate_rule.v3, duplicate_rule.v4, duplicate_rule.v5);

ALTER TABLE console_casbin_rules
    ALTER COLUMN ptype SET DEFAULT '',
    ALTER COLUMN ptype SET NOT NULL,
    ALTER COLUMN v0 SET DEFAULT '',
    ALTER COLUMN v0 SET NOT NULL,
    ALTER COLUMN v1 SET DEFAULT '',
    ALTER COLUMN v1 SET NOT NULL,
    ALTER COLUMN v2 SET DEFAULT '',
    ALTER COLUMN v2 SET NOT NULL,
    ALTER COLUMN v3 SET DEFAULT '',
    ALTER COLUMN v3 SET NOT NULL,
    ALTER COLUMN v4 SET DEFAULT '',
    ALTER COLUMN v4 SET NOT NULL,
    ALTER COLUMN v5 SET DEFAULT '',
    ALTER COLUMN v5 SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_console_casbin_rules_unique
    ON console_casbin_rules (ptype, v0, v1, v2, v3, v4, v5);
