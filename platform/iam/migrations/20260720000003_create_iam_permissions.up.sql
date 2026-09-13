-- iam_permissions: permission identifier registry.
-- Global table: no tenant_id, no RLS.
-- Populated at startup by the compiler output (CapabilityGrants → permission identifiers).
-- Format: "{module}.{entity}.{operation}" e.g. "finance.invoice.create"

CREATE TABLE iam_permissions (
    identifier  VARCHAR(255) NOT NULL,
    label       VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT,
    entity_name VARCHAR(255) NOT NULL DEFAULT '',
    action      VARCHAR(128) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_permissions_pkey PRIMARY KEY (identifier)
);

COMMENT ON TABLE  iam_permissions            IS 'Permission identifier registry. Global table — no RLS. Populated from compiler CapabilityGrants.';
COMMENT ON COLUMN iam_permissions.identifier IS 'Format: {module}.{entity}.{operation}. e.g. "finance.invoice.create"';
COMMENT ON COLUMN iam_permissions.entity_name IS 'Qualified entity name. e.g. "finance_invoice"';
COMMENT ON COLUMN iam_permissions.action      IS 'Operation name. e.g. "create", "read", "submit"';
