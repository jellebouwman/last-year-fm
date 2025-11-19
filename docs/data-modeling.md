# Data Modeling Decisions

## Timestamps (createdAt/updatedAt)

**Decision**: Not using automatic timestamp columns.

**Rationale**: Drizzle's `updatedAt` doesn't auto-update - requires manual updates or database triggers. Adding columns "just in case" without a clear use case adds maintenance burden. Can be added later via migration when needed.
