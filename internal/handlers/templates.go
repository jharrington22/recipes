package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// TemplateRenderer holds one *template.Template per page, each composed
// from base.html + that page's own file.
//
// Page files all define a block named "content" (see web/templates/*.html).
// html/template shares one namespace across every file parsed together, so
// parsing them all in a single ParseGlob would let only the
// alphabetically-last file's "content" definition win for every page.
// Pairing each page with base.html in its own *template.Template avoids
// that collision.
type TemplateRenderer struct {
	pages map[string]*template.Template
}

func NewTemplateRenderer(glob string) (*TemplateRenderer, error) {
	files, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}

	var baseFile string
	var pageFiles []string
	for _, f := range files {
		if filepath.Base(f) == "base.html" {
			baseFile = f
		} else {
			pageFiles = append(pageFiles, f)
		}
	}
	if baseFile == "" {
		return nil, fmt.Errorf("templates: base.html not found in %s", glob)
	}

	pages := map[string]*template.Template{}
	for _, pf := range pageFiles {
		name := filepath.Base(pf)
		t, err := template.New(name).ParseFiles(baseFile, pf)
		if err != nil {
			return nil, fmt.Errorf("templates: parsing %s: %w", name, err)
		}
		pages[name] = t
	}

	return &TemplateRenderer{pages: pages}, nil
}

func (r *TemplateRenderer) Render(c *gin.Context, name string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	// common helpers
	data["Now"] = time.Now()

	t, ok := r.pages[name]
	if !ok {
		c.String(http.StatusInternalServerError, "template not found: %s", name)
		return
	}

	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(c.Writer, name, data); err != nil {
		// Headers/status are already sent; log-and-swallow is the best we
		// can do here, matching the previous renderer's behavior.
		_ = err
	}
}
