create table refresh_token(
    id uuid PRIMARY key,
    user_id uuid NOT NULL,
    hashed_value text NOT NULL,
    device_key text NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_refresh_token_device_key ON refresh_token(device_key);
CREATE INDEX idx_refresh_token_hashed_value ON refresh_token(hashed_value);

