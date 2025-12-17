# Local Development

## Database

Start PostgreSQL:

```bash
docker compose up -d
```

Connection: `postgresql://postgres:postgres@localhost:5432/lastyearfm`

Stop:

```bash
docker compose down      # Keep data
docker compose down -v   # Delete data
```

Access psql:

```bash
docker compose exec db psql -U postgres -d lastyearfm
```

Run a single query:

```bash
docker compose exec db psql -U postgres -d lastyearfm -c "SELECT version();"
```

## Database Setup Workflow

After starting the database:

```bash
pnpm db:generate   # Generate migrations from schema changes
pnpm db:migrate    # Apply migrations to database
pnpm db:seed       # Insert test data
pnpm db:studio     # Open Drizzle Studio (database GUI)
pnpm db:flush      # Truncate all tables (local only)
pnpm db:reset      # Flush and reseed (local only)
```

Full reset workflow:

```bash
docker compose down -v   # Delete data
docker compose up -d     # Start fresh
pnpm db:migrate          # Apply all migrations
pnpm db:seed             # Seed test data
```

## Drizzle Studio

Browse and edit your database with a web UI:

```bash
pnpm db:studio
```

Opens at `https://local.drizzle.studio`

## Flush and Reset

```bash
pnpm db:flush   # Truncate all tables (keeps schema)
pnpm db:reset   # Flush + reseed
```

Safety: Only works on local databases (localhost/127.0.0.1).

## Worker (Go + sqlc)

The `packages/worker/` directory uses [sqlc](https://sqlc.dev/) to generate type-safe Go code from SQL queries.

sqlc reads schema from `packages/db/drizzle/` (Drizzle migrations). Queries must match that schema.

sqlc is managed as a Go tool dependency via `tools.go` (version-locked in `go.mod`).

**Type Generation:**

```bash
# 1. Write SQL queries in packages/worker/queries/*.sql
#    (must match schema in packages/db/drizzle/)
# 2. Generate Go types
pnpm worker:generate

# 3. Generated types appear in packages/worker/db/
```

**Formatting & Linting:**

```bash
pnpm worker:format  # Format Go code with goimports
pnpm worker:lint    # Lint Go code with golangci-lint
pnpm check          # Format and lint all packages (TS, Astro, Go)
```

**Building:**

```bash
pnpm worker:build   # Build binary to dist/worker
```

**Running the Worker:**

In development, the worker loads `.env.local` or `.env` automatically. In production (`GO_ENV=production`), it expects system environment variables.

```bash
pnpm worker:dev  # Runs on port 8080
```

**Import Scrobbles:**

```bash
# Import scrobbles for a user and year (defaults: jellebouwman, 2025)
curl -X POST http://localhost:8080/import \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2025}'
```

**Find Release Years:**

```bash
# Find release years using MusicBrainz database (defaults: jellebouwman, 2025)
curl -X POST http://localhost:8080/find-release-years \
  -H "Content-Type: application/json" \
  -d '{}'

# With custom username and year
curl -X POST http://localhost:8080/find-release-years \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2024}'
```

**Full Workflow:**

```bash
# 1. Import scrobbles from Last.fm
curl -X POST http://localhost:8080/import \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2025}'

# 2. Find release years from MusicBrainz
curl -X POST http://localhost:8080/find-release-years \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2025}'
```

Configuration: `packages/worker/sqlc.yaml`
