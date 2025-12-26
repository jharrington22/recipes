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
	"github.com/jharrington22/recipes/internal/search"
)

type RecipesHTML struct {
	DB *db.DB
	R  *TemplateRenderer
}

type RecipeListItem struct {
	ID          int64
	Title       string
	Slug        string
	Category    string
	Description string
	CreatedAt   time.Time
	IsFavorite  bool
}

func (h *RecipesHTML) Home(c *gin.Context) {
	// Latest recipes
	ctx := context.Background()

	uid, _, authed := auth.CurrentUser(c)

	rows, err := h.DB.Pool.Query(ctx, `
SELECT r.id, r.title, r.slug, r.category, COALESCE(r.description,''), r.created_at,
       CASE WHEN $1::bigint IS NULL THEN false
            ELSE EXISTS (SELECT 1 FROM user_favorites uf WHERE uf.user_id=$1 AND uf.recipe_id=r.id)
       END AS is_fav
FROM recipes r
ORDER BY r.created_at DESC
LIMIT 12
`, func() any {
		if authed {
			return uid
		}
		return nil
	}())
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	items := []RecipeListItem{}
	for rows.Next() {
		var it RecipeListItem
		if err := rows.Scan(&it.ID, &it.Title, &it.Slug, &it.Category, &it.Description, &it.CreatedAt, &it.IsFavorite); err != nil {
			c.String(500, err.Error())
			return
		}
		items = append(items, it)
	}

	h.R.Render(c, "index.html", gin.H{
		"Title":  "Latest recipes",
		"Items":  items,
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

	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	where := []string{"1=1"}

	if category != "" {
		where = append(where, "r.category = "+arg(category))
	}
	if q != "" {
		where = append(where, "r.title ILIKE "+arg("%"+q+"%"))
	}

	includeJoin := ""
	includeHaving := ""
	if len(include) > 0 {
		includeJoin = `
JOIN recipe_ingredients ri_in ON ri_in.recipe_id = r.id
JOIN ingredients i_in ON i_in.id = ri_in.ingredient_id
`
		where = append(where, "lower(i_in.name) = ANY("+arg(include)+")")
		includeHaving = "HAVING COUNT(DISTINCT lower(i_in.name)) = " + arg(len(include))
	}

	excludeClause := ""
	if len(exclude) > 0 {
		excludeClause = `
AND NOT EXISTS (
  SELECT 1
  FROM recipe_ingredients ri_ex
  JOIN ingredients i_ex ON i_ex.id = ri_ex.ingredient_id
  WHERE ri_ex.recipe_id = r.id
    AND lower(i_ex.name) = ANY(` + arg(exclude) + `)
)
`
	}

	// user id argument for favorites existence
	var uidArg string
	if authed {
		uidArg = arg(uid)
	} else {
		uidArg = "NULL"
	}

	sql := `
SELECT r.id, r.title, r.slug, r.category, COALESCE(r.description,''), r.created_at,
       CASE WHEN ` + uidArg + `::bigint IS NULL THEN false
            ELSE EXISTS (SELECT 1 FROM user_favorites uf WHERE uf.user_id=` + uidArg + ` AND uf.recipe_id=r.id)
       END AS is_fav
FROM recipes r
` + includeJoin + `
WHERE ` + strings.Join(where, " AND ") + `
` + excludeClause + `
GROUP BY r.id
` + includeHaving + `
ORDER BY r.created_at DESC
LIMIT ` + arg(limit) + ` OFFSET ` + arg(offset) + `
`

	rows, err := h.DB.Pool.Query(ctx, sql, args...)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	items := []RecipeListItem{}
	for rows.Next() {
		var it RecipeListItem
		if err := rows.Scan(&it.ID, &it.Title, &it.Slug, &it.Category, &it.Description, &it.CreatedAt, &it.IsFavorite); err != nil {
			c.String(500, err.Error())
			return
		}
		items = append(items, it)
	}

	h.R.Render(c, "recipes.html", gin.H{
		"Title":  "Recipes",
		"Items":  items,
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

	var (
		rid          int64
		title        string
		category     string
		description  string
		instructions string
		createdAt    time.Time
		isFav        bool
	)

	err := h.DB.Pool.QueryRow(ctx, `
SELECT r.id, r.title, r.category, COALESCE(r.description,''), COALESCE(r.instructions,''), r.created_at,
       CASE WHEN $2::bigint IS NULL THEN false
            ELSE EXISTS (SELECT 1 FROM user_favorites uf WHERE uf.user_id=$2 AND uf.recipe_id=r.id)
       END AS is_fav
FROM recipes r
WHERE r.slug=$1
`, slug, func() any {
		if authed {
			return uid
		}
		return nil
	}()).Scan(&rid, &title, &category, &description, &instructions, &createdAt, &isFav)

	if err != nil {
		c.String(404, "recipe not found")
		return
	}

	ingRows, err := h.DB.Pool.Query(ctx, `
SELECT i.name, ri.quantity, ri.unit
FROM recipe_ingredients ri
JOIN ingredients i ON i.id = ri.ingredient_id
WHERE ri.recipe_id=$1
ORDER BY i.name
`, rid)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer ingRows.Close()

	type Ingredient struct {
		Name     string
		Quantity string
		Unit     string
	}
	ings := []Ingredient{}
	for ingRows.Next() {
		var name string
		var qty *string
		var unit *string
		if err := ingRows.Scan(&name, &qty, &unit); err != nil {
			c.String(500, err.Error())
			return
		}
		ings = append(ings, Ingredient{
			Name: name,
			Quantity: func() string {
				if qty == nil {
					return ""
				}
				return *qty
			}(),
			Unit: func() string {
				if unit == nil {
					return ""
				}
				return *unit
			}(),
		})
	}

	h.R.Render(c, "recipe.html", gin.H{
		"Title":  title,
		"Authed": authed,
		"Recipe": gin.H{
			"id":           rid,
			"title":        title,
			"slug":         slug,
			"category":     category,
			"description":  description,
			"instructions": instructions,
			"created_at":   createdAt,
			"is_favorite":  isFav,
		},
		"Ingredients": ings,
	})
}

func (h *RecipesHTML) ToggleFavorite(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	ctx := context.Background()
	recipeID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	// Toggle: if exists delete else insert
	_, err := h.DB.Pool.Exec(ctx, `
WITH existing AS (
  SELECT 1 FROM user_favorites WHERE user_id=$1 AND recipe_id=$2
)
INSERT INTO user_favorites(user_id, recipe_id)
SELECT $1,$2
WHERE NOT EXISTS (SELECT 1 FROM existing)
`, uid, recipeID)
	if err == nil {
		// If it already existed, delete it
		_, _ = h.DB.Pool.Exec(ctx, `DELETE FROM user_favorites WHERE user_id=$1 AND recipe_id=$2`, uid, recipeID)
	}

	// go back
	ref := c.GetHeader("Referer")
	if ref == "" {
		ref = "/recipes"
	}
	c.Redirect(http.StatusFound, ref)
}
