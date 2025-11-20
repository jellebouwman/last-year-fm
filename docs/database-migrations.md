# Database Migrations

## Environments

- **Local**: Destructive operations OK, use `drizzle-kit push` for fast iteration
- **Supabase (Production)**: Conservative, use generated migrations with review

## Scripts to Add

```json
{
  "push": "drizzle-kit push",
  "migrate:prod": "DATABASE_URL=$SUPABASE_DATABASE_URL drizzle-kit migrate"
}
```

## CI/CD Pipeline (TODO)

Auto-migrate on push to main via GitHub Actions:

```yaml
- name: Run migrations
  run: pnpm db:migrate:prod
  env:
    SUPABASE_DATABASE_URL: ${{ secrets.SUPABASE_DATABASE_URL }}

- name: Deploy app
  # Vercel handles this automatically
```

### Safeguards

- Migrations run BEFORE app deploy
- Failed migration blocks deployment
- Test migrations locally before pushing

### Best Practices

- Add columns as nullable first, backfill, then add constraints
- Avoid destructive migrations (drop column/table) unless certain
- For breaking changes, do multi-step deploys
- Manual risky migrations via Supabase SQL editor if needed

## Workflow

1. Modify schema in `packages/db/src/schema.ts`
2. Local: `pnpm db:push`
3. Ready for prod: `pnpm db:generate` → review SQL
4. Test: `pnpm db:migrate` (local)
5. Push to main → CI runs migration → Vercel deploys
