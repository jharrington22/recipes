# Recipes (Gin + Postgres) - runnable starter repo

## What you get
- Blog-style home page showing latest recipes
- Browse by category: breakfast / lunch / dinner
- Search by recipe name
- Filter by ingredient include/exclude (CSV)
- Login + register (JWT stored in an httpOnly cookie)
- Per-user favorites
- Weekly meal planning (Sun..Sat x breakfast/lunch/dinner)
- Shopping list aggregation from the weekly plan
- Postgres storage
- Podman local/prod-ish support

## Prereqs
- podman + podman-compose
- (optional) psql if you want to inspect DB manually

## Run locally with Podman
```bash
make up
make migrate
make seed
open http://localhost:8080
```

Default credentials after seed:
- Email: demo@example.com
- Password: password123

## Useful commands
```bash
make logs
make down
```

## Environment variables (api container)
- DATABASE_URL (required)
- JWT_SECRET (required for anything beyond demo)
- PORT (default 8080)

## Notes
This repo intentionally uses a simple migration runner (executes SQL files in `migrations/` in lexical order).
For real production you’ll likely switch to goose/atlas or your platform’s migration job.
