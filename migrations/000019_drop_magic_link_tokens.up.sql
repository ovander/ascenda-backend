-- Magic-link sign-in now uses Socrate's own single-use tokens: Socrate e-mails
-- the link and the backend redeems it with POST /api/auth/magic-link/verify.
-- Ascenda's parallel token table was never reached in production (the e-mail
-- always carried Socrate's link, not Ascenda's verify URL) and is no longer
-- written or read.
DROP TABLE IF EXISTS magic_link_tokens;
