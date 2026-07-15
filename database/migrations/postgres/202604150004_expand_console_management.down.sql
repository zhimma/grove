UPDATE console_admins
SET email = ''
WHERE email IS NULL;

ALTER TABLE console_admins
    ALTER COLUMN email SET NOT NULL;
