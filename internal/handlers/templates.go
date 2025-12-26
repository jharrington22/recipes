package handlers

import (
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type TemplateRenderer struct {
	t *template.Template
}

func NewTemplateRenderer(glob string) (*TemplateRenderer, error) {
	t, err := template.ParseGlob(glob)
	if err != nil {
		return nil, err
	}
	return &TemplateRenderer{t: t}, nil
}

func (r *TemplateRenderer) Render(c *gin.Context, name string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	// common helpers
	data["Now"] = time.Now()
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = r.t.ExecuteTemplate(c.Writer, name, data)
}
