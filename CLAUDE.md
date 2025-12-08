- On a VPS, always use vim. Never nano.
- Local DB: `PGPASSWORD=postgres psql -h localhost -U postgres -d lastyearfm`

## Node

- Node 22 across all Node packages (`.node-version`, `engines.node >= 22`)

## Go

- Use modern Go syntax (Go 1.18+): `any` instead of `interface{}`, generics where appropriate
- Format: `pnpm worker:format` (goimports)
- Lint: `pnpm worker:lint` (golangci-lint)

## Code Quality

- Always run `pnpm check` for formatting/linting across all packages (TypeScript, Astro, Go)
- Never raw `npx biome`, `goimports`, or `golangci-lint` commands

## Monorepo Conventions

- Check ./docs/monorepo.md

## CLAUDE.md Style

- Keep rules compact, one-liner style
- No verbose examples - Claude knows the syntax
