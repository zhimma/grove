DROP INDEX idx_console_casbin_rules_unique ON console_casbin_rules;
DROP INDEX idx_casbin_rules_unique ON casbin_rules;

ALTER TABLE console_admins DROP FOREIGN KEY fk_console_admins_role;
ALTER TABLE console_admins DROP CHECK chk_console_admins_status;
ALTER TABLE console_roles DROP CHECK chk_console_roles_status;
ALTER TABLE system_configs DROP CHECK chk_system_configs_value_type;

DROP INDEX idx_users_email_active ON users;
DROP INDEX idx_console_roles_code_active ON console_roles;
DROP INDEX idx_console_admins_account_active ON console_admins;
DROP INDEX idx_console_admins_email_active ON console_admins;
DROP INDEX idx_console_admins_phone_active ON console_admins;
DROP INDEX idx_system_configs_group_key ON system_configs;

ALTER TABLE users DROP COLUMN active_email;
ALTER TABLE console_roles DROP COLUMN active_code;
ALTER TABLE console_admins
    DROP COLUMN active_account,
    DROP COLUMN active_email,
    DROP COLUMN active_phone;
ALTER TABLE system_configs
    DROP COLUMN active_config_group,
    DROP COLUMN active_config_key;

ALTER TABLE users ADD UNIQUE INDEX users_email_key (email);
ALTER TABLE console_roles ADD UNIQUE INDEX console_roles_code_key (code);
ALTER TABLE console_admins ADD UNIQUE INDEX console_admins_account_key (account);
ALTER TABLE system_configs ADD UNIQUE INDEX idx_system_configs_group_key (config_group, config_key);

ALTER TABLE console_operation_logs ADD COLUMN deleted_at DATETIME(6) NULL;
ALTER TABLE console_login_logs ADD COLUMN deleted_at DATETIME(6) NULL;

ALTER TABLE casbin_rules
    MODIFY COLUMN ptype VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v0 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v1 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v2 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v3 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v4 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v5 VARCHAR(100) NULL DEFAULT NULL;

ALTER TABLE console_casbin_rules
    MODIFY COLUMN ptype VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v0 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v1 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v2 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v3 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v4 VARCHAR(100) NULL DEFAULT NULL,
    MODIFY COLUMN v5 VARCHAR(100) NULL DEFAULT NULL;
