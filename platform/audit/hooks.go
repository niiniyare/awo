package audit

// No lifecycle hooks on the audit log.
//
// The audit log is written exclusively by the framework's AfterSave hooks on
// all other entities — adding hooks here would create a circular dependency
// (audit log writes triggering audit log writes).
//
// All entries are immutable after creation: no BeforeUpdate, BeforeDelete, or
// similar hooks are registered.
