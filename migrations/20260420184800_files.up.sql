-- +migrate Up

CREATE TABLE files (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    bucket_name TEXT NOT NULL,
    object_key TEXT NOT NULL,
    original_file_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_files_user_id ON files(user_id);
CREATE INDEX idx_files_status ON files(status);

CREATE UNIQUE INDEX idx_files_bucket_object_key
ON files(bucket_name, object_key);