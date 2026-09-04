package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/recipes"
)

type FavoritesHTML struct {
	DB      *db.DB
	R       *TemplateRenderer
	Recipes *recipes.Store
}

func (h *FavoritesHTML) List(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	ctx := context.Background()
	rows, err := h.DB.Pool.Query(ctx, `
SELECT recipe_slug FROM user_favorites WHERE user_id=$1 ORDER BY created_at DESC
`, uid)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	planned, err := weekPlanSlugs(ctx, h.DB, uid, true, startOfWeek(time.Now()))
	if err != nil {
		c.String(500, err.Error())
		return
	}

	items := []RecipeListItem{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			c.String(500, err.Error())
			return
		}
		r, ok := h.Recipes.BySlug(slug)
		if !ok {
			// Recipe file was removed/renamed since it was favorited; skip it.
			continue
		}
		items = append(items, RecipeListItem{
			Title:       r.Title,
			Slug:        r.Slug,
			Category:    r.Category,
			Description: r.Description,
			IsFavorite:  true,
			InWeeksPlan: planned[r.Slug],
		})
	}

	h.R.Render(c, "favorites.html", gin.H{
		"Title":  "Favorites",
		"Authed": true,
		"Items":  items,
	})
}
