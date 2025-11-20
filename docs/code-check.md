# Code Quality Checks

## Overview

The project uses a dual-formatter setup:
- **Biome**: Formatting and linting for all files (TS, JS, JSON, etc.)
- **Prettier**: Formatting for `.astro` files only

## Running Checks

```bash
pnpm check
```

This runs both formatters in sequence:
1. `biome check --write .` - Formats/lints all files except `.astro`
2. `prettier --write '**/*.astro'` - Formats `.astro` files

## Why Dual Formatters?

Biome's experimental HTML support has a bug that adds excessive blank lines in Astro frontmatter sections. Until this is fixed, Prettier handles `.astro` files while Biome handles everything else.

## Configuration

### Biome (`biome.json`)
- Experimental HTML support enabled for linting
- Formatter explicitly disabled for `.astro` files via overrides
- Linting rules customized for Astro (e.g., `noUnusedImports` off)

### Prettier (`.prettierrc.json`)
- Uses `.editorconfig` for base formatting rules
- Only formats `.astro` files via `prettier-plugin-astro`

### Editor Config (`.editorconfig`)
- Single source of truth for formatting rules (indent, line endings, etc.)
- Automatically respected by both Biome and Prettier
