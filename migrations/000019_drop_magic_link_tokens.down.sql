-- Recreates the table as defined in the baseline (000000). Its rows are not
-- restored: they were single-use tokens with a 15-minute lifetime.
CREATE TABLE IF NOT EXISTS magic_link_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email character varying(255) NOT NULL,
    token_hash character varying(64) NOT NULL,
    redirect_url text,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone,
    CONSTRAINT magic_link_tokens_pkey PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_email ON magic_link_tokens USING btree (email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_magic_link_tokens_token_hash ON magic_link_tokens USING btree (token_hash);
CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_used_at ON magic_link_tokens USING btree (used_at);
