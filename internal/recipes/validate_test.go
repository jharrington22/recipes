package recipes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDirOnRealRecipes(t *testing.T) {
	_, issues := ValidateDir("../../recipes")
	for _, iss := range issues {
		t.Errorf("unexpected validation issue: %s", iss)
	}
}

func TestValidateDirCatchesProblems(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		content     string
		wantField   string // substring expected in some issue's Field
		wantMessage string // substring expected in some issue's Message
	}{
		{
			name:     "missing title",
			filename: "no-title.md",
			content: `---
slug: no-title
ingredients:
  - name: "flour"
---
Mix it.
`,
			wantField:   "title",
			wantMessage: "required",
		},
		{
			name:     "slug does not match filename",
			filename: "mismatch.md",
			content: `---
title: "Mismatch"
slug: something-else
ingredients:
  - name: "flour"
---
Mix it.
`,
			wantField:   "slug",
			wantMessage: "must match the filename",
		},
		{
			name:     "bad category format",
			filename: "bad-category.md",
			content: `---
title: "Bad Category"
category: "Main Course"
ingredients:
  - name: "flour"
---
Mix it.
`,
			wantField:   "category",
			wantMessage: "lowercase",
		},
		{
			name:     "no ingredients",
			filename: "no-ingredients.md",
			content: `---
title: "No Ingredients"
ingredients: []
---
Mix it.
`,
			wantField:   "ingredients",
			wantMessage: "at least one ingredient",
		},
		{
			name:     "ingredient missing name",
			filename: "blank-ingredient.md",
			content: `---
title: "Blank Ingredient"
ingredients:
  - name: ""
    quantity: "2"
---
Mix it.
`,
			wantField:   "ingredients[0].name",
			wantMessage: "required",
		},
		{
			name:     "empty instructions",
			filename: "empty-instructions.md",
			content: `---
title: "Empty Instructions"
ingredients:
  - name: "flour"
---
`,
			wantField:   "instructions",
			wantMessage: "empty",
		},
		{
			name:     "invalid source_url",
			filename: "bad-url.md",
			content: `---
title: "Bad URL"
source_url: not-a-url
ingredients:
  - name: "flour"
---
Mix it.
`,
			wantField:   "source_url",
			wantMessage: "valid http",
		},
		{
			name:     "unknown frontmatter field",
			filename: "typo.md",
			content: `---
title: "Typo"
titel: "oops"
ingredients:
  - name: "flour"
---
Mix it.
`,
			wantField:   "", // structural failure, reported without a Field
			wantMessage: "titel",
		},
		{
			name:     "missing frontmatter entirely",
			filename: "no-frontmatter.md",
			content:  "Just a paragraph, no --- block at all.\n",

			wantField:   "",
			wantMessage: "frontmatter",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.filename), []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}

			_, issues := ValidateDir(dir)
			if len(issues) == 0 {
				t.Fatalf("expected at least one issue, got none")
			}

			found := false
			for _, iss := range issues {
				if iss.Field == tc.wantField && strings.Contains(iss.Message, tc.wantMessage) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected an issue with field %q containing %q, got: %v", tc.wantField, tc.wantMessage, issues)
			}
		})
	}
}

func TestValidateDirCatchesDuplicateSlugs(t *testing.T) {
	dir := t.TempDir()
	const recipe = `---
title: %q
slug: shared-slug
ingredients:
  - name: "flour"
---
Mix it.
`
	if err := os.WriteFile(filepath.Join(dir, "first.md"), []byte(fmt.Sprintf(recipe, "First")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "second.md"), []byte(fmt.Sprintf(recipe, "Second")), 0o644); err != nil {
		t.Fatal(err)
	}

	_, issues := ValidateDir(dir)
	dupCount := 0
	for _, iss := range issues {
		if iss.Field == "slug" && strings.Contains(iss.Message, "duplicate slug") {
			dupCount++
		}
	}
	if dupCount != 2 {
		t.Errorf("expected 2 duplicate-slug issues (one per file), got %d: %v", dupCount, issues)
	}
}
