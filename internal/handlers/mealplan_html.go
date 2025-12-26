package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
)

type MealPlanHTML struct {
	DB *db.DB
	R  *TemplateRenderer
}

var Days = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
var Slots = []string{"breakfast", "lunch", "dinner"}

func startOfWeek(t time.Time) time.Time {
	// Sunday start
	wd := int(t.Weekday()) // Sun=0
	st := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, -wd)
	return st
}

func (h *MealPlanHTML) Page(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	ctx := context.Background()

	ws := c.Query("week_start")
	var weekStart time.Time
	var err error
	if ws == "" {
		weekStart = startOfWeek(time.Now())
		ws = weekStart.Format("2006-01-02")
	} else {
		weekStart, err = time.Parse("2006-01-02", ws)
		if err != nil {
			c.String(400, "week_start must be YYYY-MM-DD")
			return
		}
	}

	// Pull plan rows with recipe titles
	type PlanRow struct {
		Day         int
		Slot        string
		RecipeID    int64
		RecipeTitle string
		RecipeSlug  string
	}
	rows, err := h.DB.Pool.Query(ctx, `
SELECT mp.day_of_week, mp.slot, r.id, r.title, r.slug
FROM meal_plans mp
JOIN recipes r ON r.id = mp.recipe_id
WHERE mp.user_id=$1 AND mp.week_start=$2
`, uid, weekStart)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	grid := map[string]PlanRow{}
	for rows.Next() {
		var pr PlanRow
		if err := rows.Scan(&pr.Day, &pr.Slot, &pr.RecipeID, &pr.RecipeTitle, &pr.RecipeSlug); err != nil {
			c.String(500, err.Error())
			return
		}
		grid[key(pr.Day, pr.Slot)] = pr
	}

	// recipe options for dropdown
	rr, err := h.DB.Pool.Query(ctx, `SELECT id, title, slug FROM recipes ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rr.Close()

	type Opt struct {
		ID    int64
		Title string
		Slug  string
	}
	opts := []Opt{}
	for rr.Next() {
		var o Opt
		if err := rr.Scan(&o.ID, &o.Title, &o.Slug); err != nil {
			c.String(500, err.Error())
			return
		}
		opts = append(opts, o)
	}
	sort.SliceStable(opts, func(i, j int) bool { return opts[i].Title < opts[j].Title })

	h.R.Render(c, "mealplan.html", gin.H{
		"Title":     "Weekly meal plan",
		"Authed":    true,
		"WeekStart": ws,
		"Days":      Days,
		"Slots":     Slots,
		"Grid":      grid,
		"Options":   opts,
	})
}

func (h *MealPlanHTML) Set(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	ctx := context.Background()

	ws := c.PostForm("week_start")
	dayStr := c.PostForm("day_of_week")
	slot := c.PostForm("slot")
	recipeID := c.PostForm("recipe_id")

	weekStart, err := time.Parse("2006-01-02", ws)
	if err != nil {
		c.String(400, "bad week_start")
		return
	}
	day, err := parseInt(dayStr)
	if err != nil || day < 0 || day > 6 {
		c.String(400, "bad day_of_week")
		return
	}
	if slot != "breakfast" && slot != "lunch" && slot != "dinner" {
		c.String(400, "bad slot")
		return
	}
	rid, err := parseInt64(recipeID)
	if err != nil || rid <= 0 {
		c.String(400, "bad recipe_id")
		return
	}

	_, err = h.DB.Pool.Exec(ctx, `
INSERT INTO meal_plans(user_id, week_start, day_of_week, slot, recipe_id)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (user_id, week_start, day_of_week, slot)
DO UPDATE SET recipe_id=EXCLUDED.recipe_id
`, uid, weekStart, day, slot, rid)
	if err != nil {
		c.String(500, err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/mealplan?week_start="+ws)
}

func (h *MealPlanHTML) ShoppingList(c *gin.Context) {
	uid, _, ok := auth.CurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	ctx := context.Background()

	ws := c.Query("week_start")
	weekStart, err := time.Parse("2006-01-02", ws)
	if err != nil {
		c.String(400, "week_start must be YYYY-MM-DD")
		return
	}

	type Row struct {
		Name  string
		Count int
	}
	rows, err := h.DB.Pool.Query(ctx, `
SELECT lower(i.name) as name, COUNT(*)::int as cnt
FROM meal_plans mp
JOIN recipe_ingredients ri ON ri.recipe_id = mp.recipe_id
JOIN ingredients i ON i.id = ri.ingredient_id
WHERE mp.user_id=$1 AND mp.week_start=$2
GROUP BY lower(i.name)
ORDER BY lower(i.name)
`, uid, weekStart)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	items := []Row{}
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.Name, &r.Count); err != nil {
			c.String(500, err.Error())
			return
		}
		items = append(items, r)
	}

	h.R.Render(c, "shopping.html", gin.H{
		"Title":     "Shopping list",
		"Authed":    true,
		"WeekStart": ws,
		"Items":     items,
	})
}

func key(day int, slot string) string {
	return fmt.Sprintf("%d:%s", day, slot)
}

func parseInt(s string) (int, error) {
	var x int
	_, err := fmt.Sscanf(s, "%d", &x)
	return x, err
}
func parseInt64(s string) (int64, error) {
	var x int64
	_, err := fmt.Sscanf(s, "%d", &x)
	return x, err
}
