INSERT INTO console_roles (id, name, code, display_name, description, menu_keys, is_super, status, sort)
VALUES (
    'console-role-admin',
    'Administrator',
    'admin',
    'System Administrator',
    'System administrator',
    JSON_ARRAY('ConsoleDashboard','ConsoleOverview','ConsoleConfigs','ConsoleSystemConfigs','ConsoleSystem','ConsoleAdmins','ConsoleRoles','ConsoleSessions'),
    FALSE,
    1,
    10
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
    'console-admin-demo',
    'admin',
    'Admin',
    'admin@example.com',
    '',
    '$2y$10$lvdEdutfzqd7szQ9G064J.4ZvVcGOV3GQ/82RkIDgrLRsF5gCVfK.',
    FALSE,
    'Console Admin',
    'Console Admin',
    '',
    'console-role-admin',
    1,
    FALSE,
    FALSE,
    'seeded console admin'
)
ON DUPLICATE KEY UPDATE
    account = VALUES(account),
    username = VALUES(username),
    email = VALUES(email),
    role_id = VALUES(role_id),
    status = VALUES(status),
    email_verified = VALUES(email_verified),
    phone_verified = VALUES(phone_verified),
    remark = VALUES(remark),
    updated_at = CURRENT_TIMESTAMP(6);

DELETE FROM console_casbin_rules
WHERE (ptype = 'p' AND v0 = 'console-role-admin')
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
    ('p', 'console-role-admin', 'GET /console/v1/sessions'),
    ('p', 'console-role-admin', 'DELETE /console/v1/sessions/:id'),
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
