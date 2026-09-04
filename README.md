# Recipes

A weekly dinner planner: browse recipes, pick what you're cooking each day of
the week, and get a shopping list generated from the ingredients of whatever
you picked.

Recipe content lives as **Markdown files in [`recipes/`](./recipes)** — there's
no admin panel or database table for recipes. Anyone can add or fix a recipe
with a normal GitHub pull request; see [CONTRIBUTING.md](./CONTRIBUTING.md).
Everything else (accounts, favorites, the weekly plan) is private per-user
state kept in Postgres.

## What you get
- Browse recipes, filter by category, search by name, include/exclude by ingredient
- Login + register (JWT stored in an httpOnly cookie)
- Per-user favorites
- Weekly meal planning (Sun..Sat × breakfast/lunch/dinner)
- Shopping list aggregated (with quantities summed where units match) from the weekly plan
- Recipe content as Markdown files — open to GitHub contributions
- Postgres for accounts/favorites/meal-plans only
- Podman local/prod-ish support

## Run it locally (Podman)

This is the easiest way to get the full stack (app + Postgres) running to test.

Prereqs: `podman` and `podman-compose` (`psql` optional, for poking at the DB).

```bash
make up        # builds the image and starts api + db
make seed       # applies migrations and creates a demo login
```

Then open **http://localhost:8080**.

Demo login:
- Email: `demo@example.com`
- Password: `password123`

Try it: log in → open a recipe and hit ☆ Favorite → go to **Meal plan**,
assign a couple of recipes to days this week → open **Shopping list** to see
the aggregated ingredients.

Useful commands:
```bash
make logs    # follow container logs
make down    # stop containers (keeps the DB volume)
```

If `api` starts before Postgres has finished its first-time initialization,
it'll exit with a connection error — `podman-compose ps` will show it
`Exited`. Just restart it once the `db` container is healthy:
```bash
podman start recipes_api_1
```
(Only happens on the very first `make up` against a brand-new volume; normal
restarts are fine.)

To wipe local data and start clean (e.g. after a schema change):
```bash
podman-compose down -v
make up
make seed
```

## Run it locally (without Podman)

If you'd rather run Postgres yourself and the Go binary directly:

```bash
# 1. Point DATABASE_URL at your own Postgres instance, e.g.:
export DATABASE_URL='postgres://recipes:recipes@localhost:5432/recipes?sslmode=disable'
export JWT_SECRET='dev-secret-change-me'

# 2. Apply migrations + create the demo user
go run ./cmd/seed

# 3. Run the app (AUTO_MIGRATE isn't needed since seed already migrated)
go run ./cmd/api
```

Or use `make run`, which sets those env vars for you (assumes Postgres is
already reachable at the default local URL — run `make seed` against it
first).

Then open http://localhost:8080.

## Adding a recipe

See [CONTRIBUTING.md](./CONTRIBUTING.md) — short version: copy an existing
file in `recipes/`, edit the frontmatter and instructions, open a PR. Restart
the server (or just re-run `go run ./cmd/api`) to pick it up locally; there's
no rebuild step beyond that.

`go test ./internal/recipes/...` loads every file in `recipes/` the way the
running site does and fails on anything that doesn't parse — run it before
pushing a recipe change.

## Environment variables (api container)
- `DATABASE_URL` (required)
- `JWT_SECRET` (required for anything beyond demo)
- `PORT` (default 8080)
- `AUTO_MIGRATE` (`true` to apply `migrations/` on startup)

## Notes
This repo intentionally uses a simple migration runner (executes SQL files in
`migrations/` in lexical order, with no rollback/versioning). For real
production you'd likely switch to goose/atlas or your platform's migration
job.
