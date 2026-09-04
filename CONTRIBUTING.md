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
4. Open a pull request. CI will run the site's recipe loader against your file
   and fail the build if it doesn't parse — see [Validation](#validation).

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

Field notes:

- **title / slug** — required. `slug` should match the filename (minus `.md`)
  and must be unique across `recipes/`.
- **category** — free text. `breakfast`, `lunch`, and `dinner` get their own
  quick-filter in the nav; anything else (e.g. `dessert`, `drink`, or
  `uncategorized`) still shows up under "All" and in search.
- **ingredients** — each entry needs at least `name`. Add `quantity` and
  `unit` when you can — they're what the shopping-list page sums across a
  week's planned meals. Keep `quantity` as a plain number (`2`, `1.5`,
  `1/2`) so it can be added up; use free text like "to taste" only when there
  really isn't a quantity.
- **tags** — optional, freeform.
- **source_url** — optional link back to where the recipe came from.

## Validation

`go test ./internal/recipes/...` loads every file in `recipes/` the same way
the running site does, and fails on any file that doesn't parse (bad
frontmatter, missing title/slug, duplicate slug). Run it locally before
opening a PR:

```bash
go test ./internal/recipes/...
```

The same check runs in CI on every pull request that touches `recipes/**`.
