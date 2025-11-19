# Monorepo

This is a monorepo that is managed with pnpm. All packages lives in ./packages.

../pnpm-workspace.yml holds all packages that are known in this repository.

## Packages

- db
- app
- worker

## Installing packages
- Install: `pnpm add <pkg> --filter <package>`
- Default to pinned versions instead of using `^` or other version range definitions.

## Creating Scripts

- Scripts: define in package, alias in root with prefix (e.g., `generate` in db package.json and `db:generate` in the root package.json)

## Running Scripts

- Always run commands from root, never cd into packages
