package recipes

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"
)

// Issue is one schema or content problem found in a recipe file, precise
// enough to annotate the offending line in a GitHub pull request.
type Issue struct {
	File    string // path relative to the recipes directory, e.g. "my-recipe.md"
	Line    int    // best-effort 1-indexed line; falls back to the frontmatter's opening line
	Field   string // frontmatter field the issue concerns; "" for whole-file issues
	Message string
}

func (i Issue) String() string {
	if i.Field != "" {
		return fmt.Sprintf("%s:%d: [%s] %s", i.File, i.Line, i.Field, i.Message)
	}
	return fmt.Sprintf("%s:%d: %s", i.File, i.Line, i.Message)
}

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	categoryPattern = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)
)

const (
	maxTitleLen       = 200
	maxDescriptionLen = 400
)

// ValidateDir validates every *.md file directly under dir against the
// recipe schema documented in CONTRIBUTING.md. It returns every recipe that
// parsed well enough to build (even if it has issues, e.g. a too-long
// description) plus the full list of issues found across all files,
// including cross-file problems like duplicate slugs.
//
// Unlike Load, this is intentionally strict — it's meant to run in CI
// before a contribution is merged, not to keep a running site up despite a
// single bad file.
func ValidateDir(dir string) ([]*Recipe, []Issue) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []Issue{{File: dir, Line: 1, Message: err.Error()}}
	}

	var recipes []*Recipe
	var issues []Issue
	slugFiles := map[string][]string{}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		r, fileIssues := validateFile(filepath.Join(dir, e.Name()), e.Name())
		issues = append(issues, fileIssues...)
		if r != nil {
			recipes = append(recipes, r)
			slugFiles[r.Slug] = append(slugFiles[r.Slug], e.Name())
		}
	}

	for slug, files := range slugFiles {
		if len(files) < 2 {
			continue
		}
		sort.Strings(files)
		for _, f := range files {
			others := make([]string, 0, len(files)-1)
			for _, o := range files {
				if o != f {
					others = append(others, o)
				}
			}
			issues = append(issues, Issue{
				File: f, Line: 1, Field: "slug",
				Message: fmt.Sprintf("duplicate slug %q also used by %s", slug, strings.Join(others, ", ")),
			})
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].File != issues[j].File {
			return issues[i].File < issues[j].File
		}
		return issues[i].Line < issues[j].Line
	})

	return recipes, issues
}

// validateFile fully validates one recipe file. It returns a nil Recipe
// only when the file has a structural problem (unreadable, no frontmatter,
// invalid YAML) that makes it unsafe to build at all.
func validateFile(path, relName string) (*Recipe, []Issue) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, []Issue{{File: relName, Line: 1, Message: err.Error()}}
	}

	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	lines[0] = strings.TrimPrefix(lines[0], "\uFEFF") // strip BOM if present

	openIdx := -1
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue // tolerate stray blank lines before the delimiter
		}
		if trimmed == "---" {
			openIdx = i
		}
		break
	}
	if openIdx == -1 {
		return nil, []Issue{{File: relName, Line: 1, Message: "file must start with a YAML frontmatter block delimited by ---"}}
	}

	closeIdx := -1
	for i := openIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return nil, []Issue{{File: relName, Line: openIdx + 1, Message: "frontmatter block is never closed with a second ---"}}
	}

	fmText := strings.Join(lines[openIdx+1:closeIdx], "\n")
	bodyText := strings.TrimLeft(strings.Join(lines[closeIdx+1:], "\n"), "\n")

	fieldLine := func(field string) int {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field) + `\s*:`)
		if loc := re.FindStringIndex(fmText); loc != nil {
			return openIdx + 2 + strings.Count(fmText[:loc[0]], "\n")
		}
		return openIdx + 1
	}

	dec := yaml.NewDecoder(strings.NewReader(fmText))
	dec.KnownFields(true)
	var fmData frontmatter
	if err := dec.Decode(&fmData); err != nil {
		return nil, []Issue{{File: relName, Line: openIdx + 1, Message: "frontmatter: " + err.Error()}}
	}

	var issues []Issue
	addIssue := func(field, format string, args ...any) {
		issues = append(issues, Issue{File: relName, Line: fieldLine(field), Field: field, Message: fmt.Sprintf(format, args...)})
	}

	title := strings.TrimSpace(fmData.Title)
	if title == "" {
		addIssue("title", "required field is missing or empty")
	} else if len(title) > maxTitleLen {
		addIssue("title", "is %d characters, keep it under %d", len(title), maxTitleLen)
	}

	fileSlug := strings.TrimSuffix(relName, filepath.Ext(relName))
	slug := strings.TrimSpace(fmData.Slug)
	switch {
	case slug == "":
		slug = fileSlug
	case !slugPattern.MatchString(slug):
		addIssue("slug", "must be lowercase letters, digits, and single hyphens (e.g. %q)", slugify(slug))
	case slug != fileSlug:
		addIssue("slug", "must match the filename: expected %q, got %q", fileSlug, slug)
	}

	category := strings.ToLower(strings.TrimSpace(fmData.Category))
	if category != "" && !categoryPattern.MatchString(category) {
		addIssue("category", "must be lowercase letters and single hyphens only, got %q", fmData.Category)
	}
	if category == "" {
		category = "uncategorized"
	}

	if len(strings.TrimSpace(fmData.Description)) > maxDescriptionLen {
		addIssue("description", "is over %d characters; keep the summary short and put detail in the instructions", maxDescriptionLen)
	}

	if fmData.SourceURL != "" {
		u, err := url.ParseRequestURI(strings.TrimSpace(fmData.SourceURL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			addIssue("source_url", "must be a valid http(s) URL, got %q", fmData.SourceURL)
		}
	}

	if len(fmData.Ingredients) == 0 {
		addIssue("ingredients", "at least one ingredient is required")
	}
	for idx, ing := range fmData.Ingredients {
		if strings.TrimSpace(ing.Name) == "" {
			issues = append(issues, Issue{
				File: relName, Line: fieldLine("ingredients"), Field: fmt.Sprintf("ingredients[%d].name", idx),
				Message: "ingredient name is required",
			})
		}
	}

	var htmlBuf bytes.Buffer
	if strings.TrimSpace(bodyText) == "" {
		issues = append(issues, Issue{File: relName, Line: closeIdx + 1, Field: "instructions", Message: "instructions body is empty — add the method below the closing ---"})
	} else if err := goldmark.Convert([]byte(bodyText), &htmlBuf); err != nil {
		issues = append(issues, Issue{File: relName, Line: closeIdx + 1, Field: "instructions", Message: "rendering markdown: " + err.Error()})
	}

	r := &Recipe{
		Title:            title,
		Slug:             slug,
		Category:         category,
		Description:      strings.TrimSpace(fmData.Description),
		Tags:             fmData.Tags,
		SourceURL:        strings.TrimSpace(fmData.SourceURL),
		Ingredients:      fmData.Ingredients,
		Instructions:     bodyText,
		InstructionsHTML: template.HTML(htmlBuf.String()), //nolint:gosec // trusted repo content, reviewed via PR
		FileName:         relName,
	}

	return r, issues
}
