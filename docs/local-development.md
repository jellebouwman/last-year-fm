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
