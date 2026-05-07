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
    '["*"]'::jsonb,
    true,
    1,
    0
)
ON CONFLICT (id) DO UPDATE
SET
    name = EXCLUDED.name,
    code = EXCLUDED.code,
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    menu_keys = EXCLUDED.menu_keys,
    is_super = EXCLUDED.is_super,
    status = EXCLUDED.status,
    sort = EXCLUDED.sort,
    updated_at = NOW();

INSERT INTO console_admins (
    id,
    account,
    username,
    email,
    phone,
    password,
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
    '$2y$10$3MEQrW2sinDcE35UO9IVS.DmYStpeCZ4U8DpH/DkhoUN5OQmVf.HS',
    'Root Super Admin',
    'Root',
    '',
    'console-role-root',
    1,
    false,
    false,
    'seeded root super admin'
)
ON CONFLICT (id) DO UPDATE
SET
    account = EXCLUDED.account,
    username = EXCLUDED.username,
    email = EXCLUDED.email,
    phone = EXCLUDED.phone,
    password = EXCLUDED.password,
    real_name = EXCLUDED.real_name,
    display_name = EXCLUDED.display_name,
    avatar = EXCLUDED.avatar,
    role_id = EXCLUDED.role_id,
    status = EXCLUDED.status,
    email_verified = EXCLUDED.email_verified,
    phone_verified = EXCLUDED.phone_verified,
    remark = EXCLUDED.remark,
    updated_at = NOW();

DELETE FROM console_casbin_rules
WHERE ptype = 'g'
  AND v0 = 'console-admin-root';

INSERT INTO console_casbin_rules (ptype, v0, v1)
VALUES
    ('g', 'console-admin-root', 'console-role-root')
ON CONFLICT DO NOTHING;
