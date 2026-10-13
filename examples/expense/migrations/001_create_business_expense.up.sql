-- business_expense: showcase business entity (examples/expense).
--
-- Mirrors the conventions already established by the real, production-wired
-- platform/iam migrations (tenant_isolation RLS policy, set_updated_at()
-- trigger defined in migration/bootstrap, standard column ordering).

CREATE TABLE IF NOT EXISTS business_expense (
    id            uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid         NOT NULL REFERENCES platform_tenant(id),
    created_at    timestamptz  NOT NULL DEFAULT NOW(),
    updated_at    timestamptz  NOT NULL DEFAULT NOW(),

    description   varchar(255) NOT NULL,
    amount        numeric(20,4) NOT NULL,
    category      varchar(20)  NOT NULL
                      CHECK (category IN ('travel', 'meals', 'supplies', 'other')),
    status        varchar(20)  NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft', 'submitted', 'approved', 'rejected')),
    submitted_by  uuid         NOT NULL REFERENCES iam_user(id),
    notes         text,

    -- Every SystemDefinition's generic repository scanner
    -- (contrib/pgx.Repository.systemColumnsAndScanner) unconditionally
    -- selects a custom_fields column — required on every system entity
    -- table, not optional.
    custom_fields jsonb        NOT NULL DEFAULT '{}'
);

COMMENT ON TABLE business_expense IS
    'Showcase business entity: an employee-submitted expense, tracked from draft through approval.';

ALTER TABLE business_expense ENABLE ROW LEVEL SECURITY;
ALTER TABLE business_expense FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON business_expense
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());

CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON business_expense
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_business_expense_tenant_status   ON business_expense (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_business_expense_submitted_by    ON business_expense (submitted_by);
