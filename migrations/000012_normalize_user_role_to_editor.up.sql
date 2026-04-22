-- Migration: normalize legacy "user" role to "editor" (SAFE VERSION)

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'users'
    )
    AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users'
        AND column_name = 'role'
    )
    THEN
UPDATE users
SET role = 'editor'
WHERE role = 'user';
END IF;
END $$;