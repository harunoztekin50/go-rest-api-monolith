CREATE INDEX IF NOT EXISTS user_deleted_at_idx
    ON public.users (deleted_at)
    WHERE deleted_at IS NULL;