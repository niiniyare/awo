-- platform_custom_field: tenant-defined extension fields on any entity.
-- Custom field values are stored in the custom_fields JSONB column on each
-- system entity table — no schema changes are needed when fields are added.
CREATE TABLE IF NOT EXISTS platform_custom_field (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_name     varchar(100) NOT NULL,
    field_name      varchar(100) NOT NULL,  -- must start with cf_
    label           varchar(255),
    field_type      varchar(20)  NOT NULL
                        CHECK (field_type IN (
                            'data','small_text','long_text',
                            'int','float','currency',
                            'bool','date','datetime','time',
                            'select','json'
                        )),
    options         jsonb,          -- non-null when field_type = 'select'
    required        boolean  NOT NULL DEFAULT false,
    default_value   jsonb,
    active          boolean  NOT NULL DEFAULT true,
    sort_order      integer  NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT platform_custom_field_unique UNIQUE (entity_name, field_name)
);

ALTER TABLE platform_custom_field ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_custom_field FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_custom_field
    USING (tenant_id = current_tenant_id());

-- Add tenant_id column used by RLS policy.
ALTER TABLE platform_custom_field
    ADD COLUMN IF NOT EXISTS tenant_id uuid REFERENCES platform_tenant (id);

CREATE INDEX IF NOT EXISTS platform_custom_field_entity_idx ON platform_custom_field (entity_name);
CREATE INDEX IF NOT EXISTS platform_custom_field_name_trgm  ON platform_custom_field USING GIN (field_name gin_trgm_ops);
