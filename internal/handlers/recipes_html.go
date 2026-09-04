package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/recipes"
	"github.com/jharrington22/recipes/internal/search"
)

type RecipesHTML struct {
	DB      *db.DB
	R       *TemplateRenderer
	Recipes *recipes.Store
}

type RecipeListItem struct {
	Title       string
	Slug        string
	Category    string
	Description string
	IsFavorite  bool
	InWeeksPlan bool
}

// favoriteSlugs returns the set of recipe slugs the given user has favorited.
// Returns an empty set for anonymous visitors.
func favoriteSlugs(ctx context.Context, d *db.DB, uid int64, authed bool) (map[string]bool, error) {
	set := map[string]bool{}
	if !authed {
		return set, nil
	}
	rows, err := d.Pool.Query(ctx, `SELECT recipe_slug FROM user_favorites WHERE user_id=$1`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		set[slug] = true
	}
	return set, nil
}

// weekPlanSlugs returns the set of recipe slugs already planned for the
// given user in the given week (any day/slot). Returns an empty set for
// anonymous visitors.
func weekPlanSlugs(ctx context.Context, d *db.DB, uid int64, authed bool, weekStart time.Time) (map[string]bool, error) {
	set := map[string]bool{}
	if !authed {
		return set, nil
	}
	rows, err := d.Pool.Query(ctx, `SELECT DISTINCT recipe_slug FROM meal_plans WHERE user_id=$1 AND week_start=$2`, uid, weekStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		set[slug] = true
	}
	return set, nil
}

func toListItems(rs []*recipes.Recipe, favs, planned map[string]bool) []RecipeListItem {
	items := make([]RecipeListItem, 0, len(rs))
	for _, r := range rs {
		items = append(items, RecipeListItem{
			Title:       r.Title,
			Slug:        r.Slug,
			Category:    r.Category,
			Description: r.Description,
			IsFavorite:  favs[r.Slug],
			InWeeksPlan: planned[r.Slug],
		})
	}
	return items
}

func (h *RecipesHTML) Home(c *gin.Context) {
	ctx := context.Background()
	uid, _, authed := auth.CurrentUser(c)

	favs, err := favoriteSlugs(ctx, h.DB, uid, authed)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	planned, err := weekPlanSlugs(ctx, h.DB, uid, authed, startOfWeek(time.Now()))
	if err != nil {
		c.String(500, err.Error())
		return
	}

	// Recipes are files with no reliable "added" timestamp, so this is an
	// alphabetical sample rather than a true "latest" feed (see index.html).
	top := h.Recipes.Filter(recipes.FilterOpts{Limit: 12})

	h.R.Render(c, "index.html", gin.H{
		"Title":  "All recipes",
		"Items":  toListItems(top, favs, planned),
		"Authed": authed,
	})
}

func (h *RecipesHTML) List(c *gin.Context) {
	ctx := context.Background()
	uid, _, authed := auth.CurrentUser(c)

	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	q := strings.TrimSpace(c.Query("q"))
	include := search.SplitCSV(c.Query("include"))
	exclude := search.SplitCSV(c.Query("exclude"))

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	favs, err := favoriteSlugs(ctx, h.DB, uid, authed)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	planned, err := weekPlanSlugs(ctx, h.DB, uid, authed, startOfWeek(time.Now()))
	if err != nil {
		c.String(500, err.Error())
		return
	}

	matched := h.Recipes.Filter(recipes.FilterOpts{
		Category: category,
		Query:    q,
		Include:  include,
		Exclude:  exclude,
		Limit:    limit,
		Offset:   offset,
	})

	h.R.Render(c, "recipes.html", gin.H{
		"Title":  "Recipes",
		"Items":  toListItems(matched, favs, planned),
		"Authed": authed,
		"Query": gin.H{
			"category": category,
			"q":        q,
			"include":  c.Query("include"),
			"exclude":  c.Query("exclude"),
		},
	})
}

func (h *RecipesHTML) Detail(c *gin.Context) {
	ctx := context.Background()
	slug := c.Param("slug")

	uid, _, authed := auth.CurrentUser(c)

	r, ok := h.Recipes.BySlug(slug)
	if !ok {
		c.String(404, "recipe not found")
		return
	}

	isFav := false
	inWeek := false
	if authed {
		err := h.DB.Pool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM user_favorites WHERE user_id=$1 AND recipe_slug=$2),
       EXISTS(SELECT 1 FROM meal_plans WHERE user_id=$1 AND week_start=$3 AND recipe_slug=$2)
`, uid, slug, startOfWeek(time.Now())).Scan(&isFav, &inWeek)
		if err != nil {
			c.String(500, err.Error())
			return
		}
	}

	h.R.Render(c, "recipe.html", gin.H{
		"Title":  r.Title,
		"Authed": authed,
		"Recipe": gin.H{
			"title":             r.Title,
			"slug":              r.Slug,
			"category":          r.Category,
			"description":       r.Description,
			"tags":              r.Tags,
			"source_url":        r.SourceURL,
			"instructions_html": r.InstructionsHTML,
			"is_favorite":       isFav,
			"in_weeks_plan":     inWeek,
		},
		"Ingredients": r.Ingredients,
	})
}

func (h *RecipesHTML) ToggleFavorite(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	slug := c.Param("slug")
	if _, exists := h.Recipes.BySlug(slug); !exists {
		c.String(404, "recipe not found")
		return
	}

	ctx := context.Background()
	var exists bool
	if err := h.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_favorites WHERE user_id=$1 AND recipe_slug=$2)`, uid, slug).Scan(&exists); err != nil {
		c.String(500, err.Error())
		return
	}
	if exists {
		_, err := h.DB.Pool.Exec(ctx, `DELETE FROM user_favorites WHERE user_id=$1 AND recipe_slug=$2`, uid, slug)
		if err != nil {
			c.String(500, err.Error())
			return
		}
	} else {
		_, err := h.DB.Pool.Exec(ctx, `INSERT INTO user_favorites(user_id, recipe_slug) VALUES ($1,$2)`, uid, slug)
		if err != nil {
			c.String(500, err.Error())
			return
		}
	}

	ref := c.GetHeader("Referer")
	if ref == "" {
		ref = "/recipes"
	}
	c.Redirect(http.StatusFound, ref)
}

// ToggleWeekPlan is the one-click "add to this week" / "remove" button on
// recipe cards. It's a shortcut into the same meal_plans table the
// day-by-day /mealplan grid uses, so anything added here shows up there too
// (and feeds the same shopping-list aggregation). Adding auto-picks the
// first free day for a slot inferred from the recipe's category (falling
// back to "dinner"); removing clears every day this recipe was planned for
// this week.
func (h *RecipesHTML) ToggleWeekPlan(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	slug := c.Param("slug")
	r, exists := h.Recipes.BySlug(slug)
	if !exists {
		c.String(404, "recipe not found")
		return
	}

	ctx := context.Background()
	weekStart := startOfWeek(time.Now())

	var alreadyPlanned bool
	if err := h.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM meal_plans WHERE user_id=$1 AND week_start=$2 AND recipe_slug=$3)`, uid, weekStart, slug).Scan(&alreadyPlanned); err != nil {
		c.String(500, err.Error())
		return
	}

	if alreadyPlanned {
		if _, err := h.DB.Pool.Exec(ctx, `DELETE FROM meal_plans WHERE user_id=$1 AND week_start=$2 AND recipe_slug=$3`, uid, weekStart, slug); err != nil {
			c.String(500, err.Error())
			return
		}
	} else {
		slot := r.Category
		if slot != "breakfast" && slot != "lunch" && slot != "dinner" {
			slot = "dinner"
		}

		rows, err := h.DB.Pool.Query(ctx, `SELECT day_of_week FROM meal_plans WHERE user_id=$1 AND week_start=$2 AND slot=$3`, uid, weekStart, slot)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		used := map[int]bool{}
		for rows.Next() {
			var d int
			if err := rows.Scan(&d); err != nil {
				rows.Close()
				c.String(500, err.Error())
				return
			}
			used[d] = true
		}
		rows.Close()

		day := -1
		for d := 0; d < 7; d++ {
			if !used[d] {
				day = d
				break
			}
		}

		// Every day already has a recipe planned for this slot. Rather than
		// silently overwriting one of them, leave the plan untouched — the
		// user can free up a day from the /mealplan grid first.
		if day != -1 {
			_, err = h.DB.Pool.Exec(ctx, `
INSERT INTO meal_plans(user_id, week_start, day_of_week, slot, recipe_slug)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (user_id, week_start, day_of_week, slot)
DO UPDATE SET recipe_slug=EXCLUDED.recipe_slug
`, uid, weekStart, day, slot, slug)
			if err != nil {
				c.String(500, err.Error())
				return
			}
		}
	}

	ref := c.GetHeader("Referer")
	if ref == "" {
		ref = "/recipes"
	}
	c.Redirect(http.StatusFound, ref)
}
