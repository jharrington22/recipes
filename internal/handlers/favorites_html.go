package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
)

type FavoritesHTML struct {
	DB *db.DB
	R  *TemplateRenderer
}

func (h *FavoritesHTML) List(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	ctx := context.Background()
	rows, err := h.DB.Pool.Query(ctx, `
SELECT r.id, r.title, r.slug, r.category, COALESCE(r.description,''), r.created_at
FROM user_favorites uf
JOIN recipes r ON r.id = uf.recipe_id
WHERE uf.user_id=$1
ORDER BY uf.created_at DESC
`, uid)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	type Item struct {
		ID          int64
		Title       string
		Slug        string
		Category    string
		Description string
		CreatedAt   time.Time
	}
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Title, &it.Slug, &it.Category, &it.Description, &it.CreatedAt); err != nil {
			c.String(500, err.Error())
			return
		}
		items = append(items, it)
	}

	h.R.Render(c, "favorites.html", gin.H{
		"Title":  "Favorites",
		"Authed": true,
		"Items":  items,
	})
}
