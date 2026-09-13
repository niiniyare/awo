-- platform_notification_template: server-side notification templates.
-- Global table — platform admins manage templates; tenants read them.
CREATE TABLE IF NOT EXISTS platform_notification_template (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key              varchar(100) NOT NULL UNIQUE,
    label            varchar(255),
    channel          varchar(20)  NOT NULL
                         CHECK (channel IN ('in_app','email','sms','push')),
    subject_template varchar(255),
    body_template    text         NOT NULL,
    active           boolean      NOT NULL DEFAULT true,
    metadata         jsonb,
    created_at       timestamptz  NOT NULL DEFAULT now(),
    updated_at       timestamptz  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS platform_notification_template_key_trgm
    ON platform_notification_template USING GIN (key gin_trgm_ops);

-- platform_notification: delivery records — one per channel per recipient.
-- RLS enabled — tenant-scoped.
CREATE TABLE IF NOT EXISTS platform_notification (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES platform_tenant (id),
    recipient_id uuid NOT NULL REFERENCES iam_user (id),
    channel     varchar(20) NOT NULL
                    CHECK (channel IN ('in_app','email','sms','push')),
    title       varchar(255) NOT NULL,
    body        varchar(1024) NOT NULL,
    template_id uuid REFERENCES platform_notification_template (id),
    data        jsonb,
    sent_at     timestamptz,
    failed_at   timestamptz,
    error_message varchar(1024),
    read_at     timestamptz,
    channel_ref varchar(255),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE platform_notification ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_notification FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_notification
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS platform_notification_tenant_idx    ON platform_notification (tenant_id);
CREATE INDEX IF NOT EXISTS platform_notification_recipient_idx ON platform_notification (recipient_id);
CREATE INDEX IF NOT EXISTS platform_notification_channel_idx   ON platform_notification (channel);
CREATE INDEX IF NOT EXISTS platform_notification_unread_idx    ON platform_notification (recipient_id, read_at)
    WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS platform_notification_created_idx   ON platform_notification (created_at DESC);
