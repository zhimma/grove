ALTER TABLE console_admins
    ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE;
