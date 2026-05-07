INSERT INTO console_roles (id, name, code, display_name, description, menu_keys, is_super, status, sort)
VALUES (
    'console-role-admin',
    'Administrator',
    'admin',
    'System Administrator',
    'System administrator',
    '["ConsoleDashboard","ConsoleOverview","ConsoleConfigs","ConsoleSystemConfigs","ConsoleSystem","ConsoleAdmins","ConsoleRoles"]'::jsonb,
    false,
    1,
    10
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
    'console-admin-demo',
    'admin',
    'Admin',
    'admin@example.com',
    '',
    '$2y$10$lvdEdutfzqd7szQ9G064J.4ZvVcGOV3GQ/82RkIDgrLRsF5gCVfK.',
    'Console Admin',
    'Console Admin',
    '',
    'console-role-admin',
    1,
    false,
    false,
    'seeded console admin'
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
WHERE
    (ptype = 'p' AND v0 = 'console-role-admin')
    OR (ptype = 'g' AND v0 = 'console-admin-demo');

INSERT INTO console_casbin_rules (ptype, v0, v1)
VALUES
    ('p', 'console-role-admin', 'GET /console/v1/auth/me'),
    ('p', 'console-role-admin', 'GET /console/v1/auth/permissions'),
    ('p', 'console-role-admin', 'GET /console/v1/dashboard/summary'),
    ('p', 'console-role-admin', 'GET /console/v1/permissions/apis'),
    ('p', 'console-role-admin', 'GET /console/v1/roles'),
    ('p', 'console-role-admin', 'GET /console/v1/roles/:id'),
    ('p', 'console-role-admin', 'POST /console/v1/roles'),
    ('p', 'console-role-admin', 'PUT /console/v1/roles/:id'),
    ('p', 'console-role-admin', 'DELETE /console/v1/roles/:id'),
    ('p', 'console-role-admin', 'GET /console/v1/roles/:id/permissions'),
    ('p', 'console-role-admin', 'POST /console/v1/roles/:id/permissions'),
    ('p', 'console-role-admin', 'GET /console/v1/roles/:id/menus'),
    ('p', 'console-role-admin', 'POST /console/v1/roles/:id/menus'),
    ('p', 'console-role-admin', 'GET /console/v1/admins'),
    ('p', 'console-role-admin', 'GET /console/v1/admins/:id'),
    ('p', 'console-role-admin', 'POST /console/v1/admins'),
    ('p', 'console-role-admin', 'PUT /console/v1/admins/:id'),
    ('p', 'console-role-admin', 'PUT /console/v1/admins/:id/status'),
    ('p', 'console-role-admin', 'PUT /console/v1/admins/:id/reset-password'),
    ('p', 'console-role-admin', 'DELETE /console/v1/admins/:id'),
    ('p', 'console-role-admin', 'GET /console/v1/system-configs'),
    ('p', 'console-role-admin', 'GET /console/v1/system-configs/groups/:group'),
    ('p', 'console-role-admin', 'POST /console/v1/system-configs'),
    ('p', 'console-role-admin', 'PUT /console/v1/system-configs/:id'),
    ('p', 'console-role-admin', 'DELETE /console/v1/system-configs/:id'),
    ('p', 'console-role-admin', 'GET /console/v1/storage/config'),
    ('p', 'console-role-admin', 'GET /console/v1/storage/all-configs'),
    ('p', 'console-role-admin', 'POST /console/v1/storage/upload'),
    ('p', 'console-role-admin', 'GET /console/v1/logs/operations'),
    ('p', 'console-role-admin', 'GET /console/v1/logs/operations/:id'),
    ('p', 'console-role-admin', 'GET /console/v1/logs/logins'),
    ('g', 'console-admin-demo', 'console-role-admin');
