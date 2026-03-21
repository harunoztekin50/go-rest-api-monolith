CREATE TABLE IF NOT EXISTS public.users
(
    id uuid PRIMARY KEY NOT NULL,
    name varchar(50) NOT NULL,
    customer_id uuid NOT NULL,
    auth_method varchar(20) NOT NULL,
    auth_id text NOT NULL,
    fcm_token text NULL,
    credits bigint NOT NULL,
    credits_expires_at TIMESTAMPTZ NULL,
    subscription_plan varchar NULL,
    subscription_period varchar NULL,
    subscription_type varchar NULL,
    subscription_status varchar NULL,
    subscription_expires_at TIMESTAMPTZ NULL,
    is_new_user boolean NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ NULL
);

-- auth_id index: login sorgularında WHERE auth_id = ? hızlı çalışsın
CREATE INDEX IF NOT EXISTS user_auth_id_idx
    ON public.users USING btree (auth_id)
    WITH (fillfactor=100, deduplicate_items=True);

-- customer_id index: RevenueCat webhook'larında hızlı kullanıcı bulmak için
CREATE INDEX IF NOT EXISTS user_customer_id_idx
    ON public.users USING btree (customer_id)
    WITH (fillfactor=100, deduplicate_items=True);