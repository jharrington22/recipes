package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/recipes"
	"github.com/jharrington22/recipes/internal/search"
)

type API struct {
	DB          *db.DB
	RecipeStore *recipes.Store
}

func (a *API) Me(c *gin.Context) {
	uid, email, _ := auth.CurrentUser(c)
	c.JSON(http.StatusOK, gin.H{"user_id": uid, "email": email})
}

func (a *API) Recipes(c *gin.Context) {
	// Simple JSON list mirrors HTML list endpoint
	h := &RecipesHTML{DB: a.DB, Recipes: a.RecipeStore}
	h.List(c)
}

func (a *API) Search(c *gin.Context) {
	// Dedicated JSON search endpoint (optional); demonstrate include/exclude parsing
	_ = search.SplitCSV(c.Query("include"))
	_ = search.SplitCSV(c.Query("exclude"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) MealPlanWeek(c *gin.Context) {
	uid, _, _ := auth.CurrentUser(c)
	ws := c.Query("week_start")
	weekStart, err := time.Parse("2006-01-02", ws)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "week_start must be YYYY-MM-DD"})
		return
	}
	ctx := context.Background()
	rows, err := a.DB.Pool.Query(ctx, `
SELECT day_of_week, slot, recipe_slug
FROM meal_plans
WHERE user_id=$1 AND week_start=$2
ORDER BY day_of_week, slot
`, uid, weekStart)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Item struct {
		DayOfWeek  int    `json:"day_of_week"`
		Slot       string `json:"slot"`
		RecipeSlug string `json:"recipe_slug"`
	}
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.DayOfWeek, &it.Slot, &it.RecipeSlug); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		items = append(items, it)
	}
	c.JSON(200, gin.H{"week_start": ws, "items": items})
}
