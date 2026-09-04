// Command validate-recipes checks every file under recipes/ against the
// recipe schema documented in CONTRIBUTING.md, and reports problems
// precisely enough to annotate the offending line on a GitHub pull request.
//
// Run it locally exactly the way CI does:
//
//	go run ./cmd/validate-recipes
//
// To also emit the compiled recipe list as JSON once validation passes:
//
//	go run ./cmd/validate-recipes -json build/recipes.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jharrington22/recipes/internal/recipes"
)

func main() {
	dir := flag.String("dir", "recipes", "directory of recipe Markdown files to validate")
	format := flag.String("format", "text", `issue output format: "text" (human-readable) or "github" (workflow annotations)`)
	jsonOut := flag.String("json", "", "if set, write the compiled recipe list as JSON to this path once validation passes")
	flag.Parse()

	recs, issues := recipes.ValidateDir(*dir)

	for _, issue := range issues {
		switch *format {
		case "github":
			// https://docs.github.com/actions/using-workflows/workflow-commands-for-github-actions#setting-an-error-message
			// Issue content ultimately derives from an unreviewed PR's file
			// contents, so both the property (file path) and the message
			// must be percent-encoded per GitHub's rules — otherwise a
			// crafted value (e.g. embedding a newline) could break out of
			// this line and forge extra workflow-command output.
			file := escapeProperty(fmt.Sprintf("%s/%s", *dir, issue.File))
			msg := escapeData(annotationMessage(issue))
			fmt.Printf("::error file=%s,line=%d::%s\n", file, issue.Line, msg)
		default:
			fmt.Fprintln(os.Stderr, issue.String())
		}
	}

	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d issue(s) found in %s\n", len(issues), *dir)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "%d recipe file(s) in %s, all valid\n", len(recs), *dir)

	if *jsonOut != "" {
		if err := writeJSON(*jsonOut, recs); err != nil {
			fmt.Fprintf(os.Stderr, "writing %s: %v\n", *jsonOut, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *jsonOut)
	}
}

func annotationMessage(issue recipes.Issue) string {
	if issue.Field != "" {
		return fmt.Sprintf("[%s] %s", issue.Field, issue.Message)
	}
	return issue.Message
}

// escapeData and escapeProperty implement GitHub Actions' required
// percent-encoding for workflow command data and property values,
// respectively. See:
// https://docs.github.com/actions/using-workflows/workflow-commands-for-github-actions#escaping-properties
func escapeData(s string) string {
	r := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	return r.Replace(s)
}

func escapeProperty(s string) string {
	r := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
	return r.Replace(s)
}

func writeJSON(path string, recs []*recipes.Recipe) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(recs)
}
