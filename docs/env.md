# Environment variables

.env.example holds all available keys. Not all variables are relevant for all packages.

## Environment Files

- `.env.local` - Local development
- `.env.production` - Production

Both are gitignored. Scripts/tools select the appropriate file based on environment.

## Usage

Node projects use dotenv to load the appropriate .env file.
