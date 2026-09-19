-- events_outbox: the transactional outbox table for domain events and
-- workflow-trigger dispatch (ADR-025, superseding ADR-007's workflow_outbox
-- and ADR-008's event_outbox — see
-- docs/adr/ADR-025-transactional-events-and-workflow-durability.md §7).
--
-- Global table — no row-level security. Tenant isolation for delivery is the
-- outbox relay's responsibility (restoring tenant.TenantContext from this
-- table's own tenant_id column before dispatch, ADR-025 §13), not a
-- PostgreSQL policy enforced on this table; tenant_id here is data the event
-- carries, not an access-control boundary the database enforces (ADR-025
-- §7, §21).
--
-- This is the one authoritative production schema for events_outbox. It
-- supersedes, for this table only, the orphaned/unapplied events_outbox
-- definition inside migrations/20260706000003_create_platform_support.up.sql
-- — that top-level migrations/ directory is never embedded or registered by
-- this framework's live migration system (see that file's own header
-- comment, and PHASE2_ARCHITECTURE_PLAN.md §10/§21 Step 2 for the full
-- reconciliation).

CREATE TABLE events_outbox (
    id              uuid PRIMARY KEY,
    tenant_id       uuid NOT NULL,
    type            text NOT NULL,
    entity_name     text NOT NULL,
    record_id       uuid NOT NULL,
    actor_id        uuid,
    system_actor    text,
    action_name     text,
    correlation_id  text,
    payload         jsonb,
    occurred_at     timestamptz NOT NULL,
    delivered_at    timestamptz,
    attempts        int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text,
    dead_at         timestamptz
);

COMMENT ON TABLE events_outbox IS
    'Transactional outbox for domain events and workflow-trigger dispatch (ADR-025). '
    'Global table, no RLS -- tenant_id is event data restored by the relay before '
    'dispatch, not a database-enforced access boundary. See ADR-025 sections 7, 13, 21.';

-- Pending-work lookup: an event is pending exactly when it has not been
-- delivered and has not been moved to the terminal dead-letter state.
-- Ordered by next_attempt_at so the relay can efficiently claim only rows
-- whose backoff window has elapsed (ADR-025 §9.4) -- this replaces the flat
-- "attempts < 5" cutoff the orphaned migration's own index used.
CREATE INDEX events_outbox_pending
    ON events_outbox (next_attempt_at)
    WHERE delivered_at IS NULL AND dead_at IS NULL;

-- awo_app needs INSERT (the in-transaction outbox write, Step 1) and
-- SELECT/UPDATE (the relay's poll/deliver/attempt-tracking cycle, Step 3B).
-- No DELETE is granted -- dead-lettered rows remain queryable (ADR-025
-- §9.4/§16), never removed by the application role. Wrapped in an exception
-- handler to be safe in environments where awo_app is not yet provisioned,
-- matching platform/audit/migrations/001_platform_audit_log.up.sql's
-- established convention.
DO $$
BEGIN
    GRANT SELECT, INSERT, UPDATE ON events_outbox TO awo_app;
EXCEPTION
    WHEN undefined_object THEN NULL;
END
$$;
