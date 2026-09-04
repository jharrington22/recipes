package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/handlers"
	"github.com/jharrington22/recipes/internal/migrate"
	"github.com/jharrington22/recipes/internal/recipes"
)

func pickPath(preferred, fallback string) string {
	if _, err := os.Stat(preferred); err == nil {
		return preferred
	}
	return fallback
}

func main() {
	ctx := context.Background()

	d, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer d.Pool.Close()

	// Detect container vs local paths
	templatesGlob := filepath.Join(pickPath("/web/templates", "web/templates"), "*.html")
	staticDir := pickPath("/web/static", "web/static")
	migrationsDir := pickPath("/migrations", "migrations")
	recipesDir := pickPath("/recipes", "recipes")

	recipeStore, parseErrs := recipes.Load(recipesDir)
	for _, pe := range parseErrs {
		log.Printf("recipes: skipping %s: %v", pe.FileName, pe.Err)
	}
	log.Printf("recipes: loaded %d recipe(s) from %s (%d skipped)", len(recipeStore.All()), recipesDir, len(parseErrs))

	if os.Getenv("AUTO_MIGRATE") == "true" {
		if err := migrate.ApplyDirectory(ctx, d.Pool, migrationsDir); err != nil {
			log.Fatalf("auto-migrate failed: %v", err)
		}
		log.Printf("auto-migrate: ok")
	}

	r := gin.Default()
	r.Static("/static", staticDir)

	renderer, err := handlers.NewTemplateRenderer(templatesGlob)
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	// Attach auth context if cookie exists (non-blocking)
	r.Use(func(c *gin.Context) {
		tok, _ := c.Cookie(auth.CookieName)
		if tok != "" {
			claims, err := auth.Verify(tok)
			if err == nil {
				c.Set(auth.CtxUserIDKey, claims.UserID)
				c.Set(auth.CtxEmailKey, claims.Email)
			}
		}
		c.Next()
	})

	// HTML handlers
	authHTML := &handlers.AuthHTML{DB: d, R: renderer}
	recipesHTML := &handlers.RecipesHTML{DB: d, R: renderer, Recipes: recipeStore}
	favHTML := &handlers.FavoritesHTML{DB: d, R: renderer, Recipes: recipeStore}
	mealHTML := &handlers.MealPlanHTML{DB: d, R: renderer, Recipes: recipeStore}

	r.GET("/", recipesHTML.Home)
	r.GET("/recipes", recipesHTML.List)
	r.GET("/recipes/:slug", recipesHTML.Detail)

	r.GET("/login", authHTML.LoginPage)
	r.POST("/login", authHTML.LoginPost)
	r.GET("/register", authHTML.RegisterPage)
	r.POST("/register", authHTML.RegisterPost)
	r.POST("/logout", authHTML.Logout)

	r.POST("/recipes/:slug/favorite", recipesHTML.ToggleFavorite)
	r.POST("/recipes/:slug/plan", recipesHTML.ToggleWeekPlan)

	r.GET("/favorites", favHTML.List)
	r.GET("/mealplan", mealHTML.Page)
	r.POST("/mealplan", mealHTML.Set)
	r.GET("/shopping", mealHTML.ShoppingList)

	// JSON API (authenticated group)
	api := &handlers.API{DB: d, RecipeStore: recipeStore}
	apiGroup := r.Group("/api")
	apiGroup.Use(auth.RequireAuth())
	{
		apiGroup.GET("/me", api.Me)
		apiGroup.GET("/mealplan", api.MealPlanWeek)
	}

	// Health
	r.GET("/healthz", func(c *gin.Context) {
		if err := d.Pool.Ping(ctx); err != nil {
			c.JSON(500, gin.H{"ok": false})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	s := &http.Server{Addr: ":" + port, Handler: r}
	log.Fatal(s.ListenAndServe())
}
