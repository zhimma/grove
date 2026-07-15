INSERT INTO console_roles (
    id,
    name,
    code,
    display_name,
    description,
    menu_keys,
    is_super,
    status,
    sort
)
VALUES (
    'console-role-root',
    'Root',
    'root',
    'Root Super Admin',
    'Built-in root super administrator role',
    JSON_ARRAY('*'),
    TRUE,
    1,
    0
)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    code = VALUES(code),
    display_name = VALUES(display_name),
    description = VALUES(description),
    menu_keys = VALUES(menu_keys),
    is_super = VALUES(is_super),
    status = VALUES(status),
    sort = VALUES(sort),
    updated_at = CURRENT_TIMESTAMP(6);

INSERT INTO console_admins (
    id,
    account,
    username,
    email,
    phone,
    password,
    must_change_password,
    real_name,
    display_name,
    avatar,
    role_id,
    status,
    email_verified,
    phone_verified,
    remark
)
VALUES (
    'console-admin-root',
    'root',
    'Root',
    'root@example.com',
    '',
    '{{GROVE_ROOT_PASSWORD_HASH}}',
    TRUE,
    'Root Super Admin',
    'Root',
    '',
    'console-role-root',
    1,
    FALSE,
    FALSE,
    'seeded root super admin'
)
ON DUPLICATE KEY UPDATE
    role_id = VALUES(role_id),
    updated_at = CURRENT_TIMESTAMP(6);

UPDATE console_admins
SET role_id = 'console-role-root',
    updated_at = CURRENT_TIMESTAMP(6)
WHERE id = 'console-admin-root';

DELETE FROM console_casbin_rules
WHERE ptype = 'g'
  AND v0 = 'console-admin-root';

INSERT INTO console_casbin_rules (ptype, v0, v1)
VALUES ('g', 'console-admin-root', 'console-role-root');
