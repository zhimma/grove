ALTER TABLE console_operation_logs DROP COLUMN deleted_at;
ALTER TABLE console_login_logs DROP COLUMN deleted_at;

ALTER TABLE users DROP INDEX email;
ALTER TABLE console_roles DROP INDEX code;
ALTER TABLE console_admins DROP INDEX account;
ALTER TABLE system_configs DROP INDEX idx_system_configs_group_key;

ALTER TABLE users
    ADD COLUMN active_email VARCHAR(160)
        GENERATED ALWAYS AS (IF(deleted_at IS NULL, email, NULL)) STORED,
    ADD UNIQUE INDEX idx_users_email_active (active_email);

ALTER TABLE console_roles
    ADD COLUMN active_code VARCHAR(120)
        GENERATED ALWAYS AS (IF(deleted_at IS NULL, code, NULL)) STORED,
    ADD UNIQUE INDEX idx_console_roles_code_active (active_code);

ALTER TABLE console_admins
    ADD COLUMN active_account VARCHAR(120)
        GENERATED ALWAYS AS (IF(deleted_at IS NULL, account, NULL)) STORED,
    ADD COLUMN active_email VARCHAR(160)
        GENERATED ALWAYS AS (IF(deleted_at IS NULL AND email IS NOT NULL AND email <> '', email, NULL)) STORED,
    ADD COLUMN active_phone VARCHAR(32)
        GENERATED ALWAYS AS (IF(deleted_at IS NULL AND phone <> '', phone, NULL)) STORED,
    ADD UNIQUE INDEX idx_console_admins_account_active (active_account),
    ADD UNIQUE INDEX idx_console_admins_email_active (active_email),
    ADD UNIQUE INDEX idx_console_admins_phone_active (active_phone);

ALTER TABLE system_configs
    ADD COLUMN active_config_group VARCHAR(64)
        GENERATED ALWAYS AS (IF(deleted_at IS NULL, config_group, NULL)) STORED,
    ADD COLUMN active_config_key VARCHAR(120)
        GENERATED ALWAYS AS (IF(deleted_at IS NULL, config_key, NULL)) STORED,
    ADD UNIQUE INDEX idx_system_configs_group_key (active_config_group, active_config_key);

ALTER TABLE console_admins
    ADD CONSTRAINT fk_console_admins_role
    FOREIGN KEY (role_id)
    REFERENCES console_roles (id)
    ON UPDATE CASCADE
    ON DELETE RESTRICT;

ALTER TABLE console_admins
    ADD CONSTRAINT chk_console_admins_status CHECK (status IN (0, 1, 2));

ALTER TABLE console_roles
    ADD CONSTRAINT chk_console_roles_status CHECK (status IN (0, 1));

ALTER TABLE system_configs
    ADD CONSTRAINT chk_system_configs_value_type CHECK (value_type IN ('string', 'array', 'int', 'bool', 'json'));

UPDATE casbin_rules
SET ptype = COALESCE(ptype, ''),
    v0 = COALESCE(v0, ''),
    v1 = COALESCE(v1, ''),
    v2 = COALESCE(v2, ''),
    v3 = COALESCE(v3, ''),
    v4 = COALESCE(v4, ''),
    v5 = COALESCE(v5, '');

DELETE current_rule
FROM casbin_rules current_rule
JOIN casbin_rules duplicate_rule
  ON current_rule.id > duplicate_rule.id
 AND current_rule.ptype = duplicate_rule.ptype
 AND current_rule.v0 = duplicate_rule.v0
 AND current_rule.v1 = duplicate_rule.v1
 AND current_rule.v2 = duplicate_rule.v2
 AND current_rule.v3 = duplicate_rule.v3
 AND current_rule.v4 = duplicate_rule.v4
 AND current_rule.v5 = duplicate_rule.v5;

ALTER TABLE casbin_rules
    MODIFY COLUMN ptype VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v0 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v1 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v2 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v3 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v4 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v5 VARCHAR(100) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX idx_casbin_rules_unique
    ON casbin_rules (ptype, v0, v1, v2, v3, v4, v5);

UPDATE console_casbin_rules
SET ptype = COALESCE(ptype, ''),
    v0 = COALESCE(v0, ''),
    v1 = COALESCE(v1, ''),
    v2 = COALESCE(v2, ''),
    v3 = COALESCE(v3, ''),
    v4 = COALESCE(v4, ''),
    v5 = COALESCE(v5, '');

DELETE current_rule
FROM console_casbin_rules current_rule
JOIN console_casbin_rules duplicate_rule
  ON current_rule.id > duplicate_rule.id
 AND current_rule.ptype = duplicate_rule.ptype
 AND current_rule.v0 = duplicate_rule.v0
 AND current_rule.v1 = duplicate_rule.v1
 AND current_rule.v2 = duplicate_rule.v2
 AND current_rule.v3 = duplicate_rule.v3
 AND current_rule.v4 = duplicate_rule.v4
 AND current_rule.v5 = duplicate_rule.v5;

ALTER TABLE console_casbin_rules
    MODIFY COLUMN ptype VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v0 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v1 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v2 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v3 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v4 VARCHAR(100) NOT NULL DEFAULT '',
    MODIFY COLUMN v5 VARCHAR(100) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX idx_console_casbin_rules_unique
    ON console_casbin_rules (ptype, v0, v1, v2, v3, v4, v5);
