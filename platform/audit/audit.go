// Package audit registers the platform_audit_log entity definition and
// configures its EntityAuditConfig. This package exists so that main.go can
// import it as a blank import to trigger entity registration via init().
//
// Phase 2 will add the actual entity registration and migration wiring once
// the platform_audit_log DDL is applied.
package audit
