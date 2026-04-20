-- Normalize legacy "user" role to "editor".
--
-- The "user" role was a backward-compatibility alias for "editor" that was
-- assigned to auto-provisioned members. The frontend does not recognise "user"
-- as a display label, so those users saw an empty role in the top bar.
-- This migration rewrites every non-owner, non-admin occurrence of "user" to
-- "editor", which is the canonical role going forward.
--
-- Safe to re-run (idempotent): rows already set to "editor" are unaffected.

UPDATE users
SET    role = 'editor'
WHERE  role = 'user';
