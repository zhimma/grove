INSERT INTO users (id, name, email)
VALUES ('api-user', 'API User', 'api-user@example.com')
ON CONFLICT (id) DO UPDATE
SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    updated_at = NOW();
