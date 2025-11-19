- On a VPS, always use vim. Never nano.

## Monorepo Conventions

- Always run commands from root, never cd into packages
- Package prefixes: `db:`, `app:`, `worker:`
- Install: `pnpm add <pkg> --filter <package>`
- Scripts: define in package, alias in root with prefix (e.g., `db:generate`)
- When installing packages, check https://www.npmjs.com/ for latest version

## CLAUDE.md Style

- Keep rules compact, one-liner style
- No verbose examples - Claude knows the syntax

## Linear Project

- **Project**: Last Year FM
- **ID**: `4b24f562-32a0-4b7f-8790-3cb5b2c45a20`
- **URL**: https://linear.app/yousable-homelab/project/last-year-fm-7c05f8bec750
- **Team**: Yousable Homelab
