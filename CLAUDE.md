- On a VPS, always use vim. Never nano.
- Local DB: `PGPASSWORD=postgres psql -h localhost -U postgres -d lastyearfm`

## Node

- Node 22 across all Node packages (`.node-version`, `engines.node >= 22`)

## Code Quality

- Always run `pnpm check` for formatting/linting (never raw `npx biome` commands)

## Monorepo Conventions

- Check ./docs/monorepo.md

## CLAUDE.md Style

- Keep rules compact, one-liner style
- No verbose examples - Claude knows the syntax
