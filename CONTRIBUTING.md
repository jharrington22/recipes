# Contributing a recipe

Every recipe on this site is a single Markdown file in [`recipes/`](./recipes). There's no
database or admin panel involved — adding, editing, or fixing a recipe is a
normal GitHub pull request against a `.md` file.

## Add a new recipe

1. Copy an existing file in `recipes/` as a starting point (e.g.
   [`recipes/tomato-basil-pasta.md`](./recipes/tomato-basil-pasta.md)).
2. Name the new file `<slug>.md`, where `<slug>` is a lowercase, dash-separated
   version of the title (e.g. `spicy-peanut-noodles.md`).
3. Fill in the frontmatter and instructions using the format below.
4. Open a pull request. A CI check validates every file in `recipes/` against
   the schema below and posts specific errors as inline comments on the exact
   line in your PR if something's wrong — see [Validation](#validation).

## File format

```markdown
---
title: "Tomato Basil Pasta"
slug: tomato-basil-pasta
category: dinner
description: "Simple weeknight pasta with tomato, garlic, and basil."
tags: [vegetarian, quick]
source_url: https://example.com/original-recipe   # optional
ingredients:
  - name: "pasta"
    quantity: "400"
    unit: "g"
  - name: "garlic"
    quantity: "2"
    unit: "cloves"
  - name: "basil"          # quantity/unit are optional — "to taste" ingredients
                            # can omit them entirely
---

Cook pasta. Sauté garlic, add tomatoes, simmer. Toss with pasta, basil, parmesan.

Instructions are plain Markdown, so `##` headings, numbered steps, and lists
all work here.
```

## Schema

Every field the validator checks, and exactly what it requires:

| Field          | Required | Rules |
|----------------|----------|-------|
| `title`        | yes      | non-empty, under 200 characters |
| `slug`         | no       | if given, must be lowercase letters/digits/single-hyphens, **and must match the filename** (minus `.md`); if omitted, the filename is used. Must be unique across `recipes/`. |
| `category`     | no       | if given, lowercase letters and single hyphens only (e.g. `dinner`, `slow-cooker`). Empty defaults to `uncategorized`. `breakfast`/`lunch`/`dinner` get their own quick-filter in the nav; anything else still shows up under "All" and in search. |
| `description`  | no       | under 400 characters — keep it a one-line summary, put detail in the instructions |
| `tags`         | no       | list of freeform strings |
| `source_url`   | no       | if given, must be a valid `http(s)://` URL |
| `ingredients`  | yes      | at least one entry; each entry needs a non-empty `name`. Add `quantity` and `unit` when you can — they're what the shopping-list page sums across a week's planned meals. Keep `quantity` as a plain number (`2`, `1.5`, `1/2`) so it can be added up; use free text like "to taste" only when there really isn't a quantity. |
| instructions (the Markdown body below the closing `---`) | yes | non-empty |

The frontmatter is also checked for **unknown fields** — a typo like `titel:`
instead of `title:` is a validation error, not a silently-ignored field.

## Validation

A dedicated tool checks every file in `recipes/` against the schema above:

```bash
make validate-recipes
# or: go run ./cmd/validate-recipes
```

Run it locally before opening a PR — it prints one line per problem with the
exact file, line, and field, e.g.:

```
spicy-peanut-noodles.md:6: [ingredients] at least one ingredient is required
```

The same check runs in CI on every pull request (`validate-recipes` job) and
reports each issue as an inline annotation on the exact line in the PR's
"Files changed" tab, so you get feedback without needing to read CI logs.
Once a PR's recipes all validate, CI also compiles them into a `recipes.json`
artifact attached to the workflow run, if you want to see exactly what your
recipe compiles to.

`go test ./...` additionally runs a lighter-weight check (`internal/recipes`'s
own test suite) that loads every recipe the same way the running site does —
useful as a quick local sanity check, but `validate-recipes` is the
authoritative, PR-annotated gate.
