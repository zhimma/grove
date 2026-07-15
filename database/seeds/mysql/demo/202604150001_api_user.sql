INSERT INTO users (id, name, email)
VALUES ('api-user', 'API User', 'api-user@example.com')
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    email = VALUES(email),
    updated_at = CURRENT_TIMESTAMP(6);
