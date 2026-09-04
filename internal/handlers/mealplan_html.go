package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/recipes"
)

type MealPlanHTML struct {
	DB      *db.DB
	R       *TemplateRenderer
	Recipes *recipes.Store
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

	type PlanRow struct {
		Day         int
		Slot        string
		RecipeSlug  string
		RecipeTitle string
	}
	rows, err := h.DB.Pool.Query(ctx, `
SELECT day_of_week, slot, recipe_slug
FROM meal_plans
WHERE user_id=$1 AND week_start=$2
`, uid, weekStart)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	grid := map[string]PlanRow{}
	for rows.Next() {
		var pr PlanRow
		if err := rows.Scan(&pr.Day, &pr.Slot, &pr.RecipeSlug); err != nil {
			c.String(500, err.Error())
			return
		}
		if r, ok := h.Recipes.BySlug(pr.RecipeSlug); ok {
			pr.RecipeTitle = r.Title
		} else {
			pr.RecipeTitle = pr.RecipeSlug + " (missing)"
		}
		grid[key(pr.Day, pr.Slot)] = pr
	}

	type Opt struct {
		Title string
		Slug  string
	}
	all := h.Recipes.All()
	opts := make([]Opt, 0, len(all))
	for _, r := range all {
		opts = append(opts, Opt{Title: r.Title, Slug: r.Slug})
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
	recipeSlug := strings.TrimSpace(c.PostForm("recipe_slug"))

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
	if _, ok := h.Recipes.BySlug(recipeSlug); !ok {
		c.String(400, "unknown recipe")
		return
	}

	_, err = h.DB.Pool.Exec(ctx, `
INSERT INTO meal_plans(user_id, week_start, day_of_week, slot, recipe_slug)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (user_id, week_start, day_of_week, slot)
DO UPDATE SET recipe_slug=EXCLUDED.recipe_slug
`, uid, weekStart, day, slot, recipeSlug)
	if err != nil {
		c.String(500, err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/mealplan?week_start="+ws)
}

// shoppingAmount accumulates the quantities seen for one ingredient+unit
// combination across every recipe planned for the week.
type shoppingAmount struct {
	sum      float64
	hasSum   bool
	freeform []string // non-numeric quantities, e.g. "to taste"
}

type ShoppingItem struct {
	Name    string
	Amounts []string // e.g. ["3 cloves", "to taste"]
}

func (h *MealPlanHTML) ShoppingList(c *gin.Context) {
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

	rows, err := h.DB.Pool.Query(ctx, `
SELECT DISTINCT recipe_slug FROM meal_plans WHERE user_id=$1 AND week_start=$2
`, uid, weekStart)
	if err != nil {
		c.String(500, err.Error())
		return
	}
	defer rows.Close()

	// name -> unit -> amount
	agg := map[string]map[string]*shoppingAmount{}
	addUnit := func(name string) map[string]*shoppingAmount {
		if agg[name] == nil {
			agg[name] = map[string]*shoppingAmount{}
		}
		return agg[name]
	}

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			c.String(500, err.Error())
			return
		}
		r, ok := h.Recipes.BySlug(slug)
		if !ok {
			continue
		}
		for _, ing := range r.Ingredients {
			name := strings.ToLower(strings.TrimSpace(ing.Name))
			if name == "" {
				continue
			}
			unit := strings.ToLower(strings.TrimSpace(ing.Unit))
			byUnit := addUnit(name)
			amt := byUnit[unit]
			if amt == nil {
				amt = &shoppingAmount{}
				byUnit[unit] = amt
			}
			if v, ok := recipes.ParseQuantity(ing.Quantity); ok {
				amt.sum += v
				amt.hasSum = true
			} else if q := strings.TrimSpace(ing.Quantity); q != "" {
				amt.freeform = append(amt.freeform, q)
			} else {
				amt.freeform = append(amt.freeform, "")
			}
		}
	}

	items := make([]ShoppingItem, 0, len(agg))
	for name, byUnit := range agg {
		var amounts []string
		for unit, amt := range byUnit {
			if amt.hasSum {
				amounts = append(amounts, formatAmount(amt.sum, unit))
			}
			for _, f := range amt.freeform {
				if f == "" {
					continue
				}
				if unit != "" {
					amounts = append(amounts, f+" "+unit)
				} else {
					amounts = append(amounts, f)
				}
			}
		}
		sort.Strings(amounts)
		items = append(items, ShoppingItem{Name: name, Amounts: amounts})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	h.R.Render(c, "shopping.html", gin.H{
		"Title":     "Shopping list",
		"Authed":    true,
		"WeekStart": ws,
		"Items":     items,
	})
}

func formatAmount(v float64, unit string) string {
	// Round to 2 decimal places, then trim any trailing zeros (and a
	// trailing '.') so "0.5" stays "0.5" but "0.3333333..." becomes "0.33".
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if unit == "" {
		return s
	}
	return s + " " + unit
}

func key(day int, slot string) string {
	return fmt.Sprintf("%d:%s", day, slot)
}

func parseInt(s string) (int, error) {
	var x int
	_, err := fmt.Sscanf(s, "%d", &x)
	return x, err
}
