# Migrations

Embedded numbered migrations live in `internal/database/migrations`. They are applied transactionally at process startup and recorded in `schema_migrations`. The single-local-admin migration preserves SMS, settings, notifications, device history, and an existing administrator hash while revoking legacy sessions. Future migrations must be numbered, reviewed for downgrade/backup impact, and must not destructively rewrite user SMS data.
