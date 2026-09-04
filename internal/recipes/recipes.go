// Package recipes loads recipe content from Markdown files with YAML
// frontmatter. Recipes live as files under a directory (default: recipes/)
// so that anyone can add or edit one with a GitHub pull request — there is
// no database table for recipe content.
package recipes

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"
)

// Ingredient is one line of a recipe's ingredient list.
type Ingredient struct {
	Name     string `yaml:"name"`
	Quantity string `yaml:"quantity,omitempty"`
	Unit     string `yaml:"unit,omitempty"`
}

// frontmatter is the YAML block at the top of a recipe file.
type frontmatter struct {
	Title       string       `yaml:"title"`
	Slug        string       `yaml:"slug"`
	Category    string       `yaml:"category"`
	Description string       `yaml:"description"`
	Tags        []string     `yaml:"tags"`
	SourceURL   string       `yaml:"source_url"`
	Ingredients []Ingredient `yaml:"ingredients"`
}

// Recipe is a fully parsed recipe: frontmatter plus rendered instructions.
type Recipe struct {
	Title            string
	Slug             string
	Category         string
	Description      string
	Tags             []string
	SourceURL        string
	Ingredients      []Ingredient
	Instructions     string // raw markdown body
	InstructionsHTML template.HTML
	FileName         string
}

// ParseError describes a single recipe file that failed to load.
type ParseError struct {
	FileName string
	Err      error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s: %v", e.FileName, e.Err)
}

// Store is an in-memory, read-only index of all loaded recipes.
type Store struct {
	all    []*Recipe
	bySlug map[string]*Recipe
}

// Load reads every *.md file directly under dir and parses it into a Recipe.
// Files that fail to parse are skipped and returned as errors so the caller
// can decide whether to fail startup or just log a warning — a single bad
// community contribution shouldn't be able to take the whole site down.
func Load(dir string) (*Store, []ParseError) {
	s := &Store{bySlug: map[string]*Recipe{}}
	var errs []ParseError

	entries, err := os.ReadDir(dir)
	if err != nil {
		return s, []ParseError{{FileName: dir, Err: err}}
	}

	seenSlugs := map[string]string{} // slug -> filename, for duplicate detection

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}

		r, err := parseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, ParseError{FileName: e.Name(), Err: err})
			continue
		}

		if existing, dup := seenSlugs[r.Slug]; dup {
			errs = append(errs, ParseError{FileName: e.Name(), Err: fmt.Errorf("duplicate slug %q (already used by %s)", r.Slug, existing)})
			continue
		}
		seenSlugs[r.Slug] = e.Name()

		s.all = append(s.all, r)
		s.bySlug[r.Slug] = r
	}

	sort.Slice(s.all, func(i, j int) bool {
		return strings.ToLower(s.all[i].Title) < strings.ToLower(s.all[j].Title)
	})

	return s, errs
}

func parseFile(path string) (*Recipe, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, err
	}

	var fmData frontmatter
	if err := yaml.Unmarshal(fm, &fmData); err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}

	base := filepath.Base(path)
	slug := strings.TrimSpace(fmData.Slug)
	if slug == "" {
		slug = strings.TrimSuffix(base, filepath.Ext(base))
	}
	slug = slugify(slug)

	if strings.TrimSpace(fmData.Title) == "" {
		return nil, fmt.Errorf("missing required field: title")
	}
	if slug == "" {
		return nil, fmt.Errorf("missing required field: slug")
	}

	category := strings.ToLower(strings.TrimSpace(fmData.Category))
	if category == "" {
		category = "uncategorized"
	}

	var htmlBuf bytes.Buffer
	if err := goldmark.Convert(body, &htmlBuf); err != nil {
		return nil, fmt.Errorf("rendering instructions: %w", err)
	}

	return &Recipe{
		Title:            strings.TrimSpace(fmData.Title),
		Slug:             slug,
		Category:         category,
		Description:      strings.TrimSpace(fmData.Description),
		Tags:             fmData.Tags,
		SourceURL:        strings.TrimSpace(fmData.SourceURL),
		Ingredients:      fmData.Ingredients,
		Instructions:     string(body),
		InstructionsHTML: template.HTML(htmlBuf.String()), //nolint:gosec // rendered from trusted repo content, reviewed via PR
		FileName:         base,
	}, nil
}

// splitFrontmatter splits a file of the form:
//
//	---
//	yaml: here
//	---
//	markdown body
//
// into the YAML block and the remaining body.
func splitFrontmatter(raw []byte) (fm []byte, body []byte, err error) {
	const delim = "---"
	s := string(raw)
	s = strings.TrimPrefix(s, "\uFEFF") // strip BOM if present

	if !strings.HasPrefix(strings.TrimLeft(s, "\r\n"), delim) {
		return nil, nil, fmt.Errorf("missing frontmatter (file must start with a --- block)")
	}
	s = strings.TrimLeft(s, "\r\n")
	s = strings.TrimPrefix(s, delim)
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimPrefix(s, "\r\n")

	idx := strings.Index(s, "\n"+delim)
	if idx == -1 {
		return nil, nil, fmt.Errorf("frontmatter not closed with ---")
	}

	fmPart := s[:idx]
	rest := s[idx+len("\n"+delim):]
	rest = strings.TrimPrefix(rest, "\r\n")
	rest = strings.TrimPrefix(rest, "\n")

	return []byte(fmPart), []byte(rest), nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// All returns every loaded recipe, sorted by title.
func (s *Store) All() []*Recipe {
	return s.all
}

// BySlug looks up a single recipe.
func (s *Store) BySlug(slug string) (*Recipe, bool) {
	r, ok := s.bySlug[slug]
	return r, ok
}

// FilterOpts narrows the recipe list, mirroring the old SQL WHERE clauses.
type FilterOpts struct {
	Category string
	Query    string
	Include  []string // ingredient names that must ALL be present
	Exclude  []string // ingredient names that must NOT be present
	Limit    int
	Offset   int
}

// Filter returns the recipes matching opts, most-recently-added-looking
// order isn't meaningful for files so results are alphabetical by title.
func (s *Store) Filter(opts FilterOpts) []*Recipe {
	include := toSet(opts.Include)
	exclude := toSet(opts.Exclude)
	q := strings.ToLower(strings.TrimSpace(opts.Query))
	category := strings.ToLower(strings.TrimSpace(opts.Category))

	matched := make([]*Recipe, 0, len(s.all))
	for _, r := range s.all {
		if category != "" && r.Category != category {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Title), q) {
			continue
		}

		names := ingredientNameSet(r)

		if len(include) > 0 {
			ok := true
			for name := range include {
				if !names[name] {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}

		if len(exclude) > 0 {
			ok := true
			for name := range exclude {
				if names[name] {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}

		matched = append(matched, r)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 30
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(matched) {
		return []*Recipe{}
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[offset:end]
}

func ingredientNameSet(r *Recipe) map[string]bool {
	set := make(map[string]bool, len(r.Ingredients))
	for _, ing := range r.Ingredients {
		set[strings.ToLower(strings.TrimSpace(ing.Name))] = true
	}
	return set
}

func toSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" {
			set[n] = true
		}
	}
	return set
}

// ParseQuantity attempts to read a decimal quantity so shopping-list
// aggregation can sum matching ingredients. Non-numeric quantities (e.g.
// "to taste", "a splash") return ok=false and should be listed as-is.
func ParseQuantity(q string) (float64, bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return 0, false
	}
	// Support simple "a/b" fractions like "1/2".
	if strings.Contains(q, "/") {
		parts := strings.SplitN(q, "/", 2)
		num, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		den, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil && den != 0 {
			return num / den, true
		}
		return 0, false
	}
	v, err := strconv.ParseFloat(q, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
