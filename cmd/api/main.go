package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/handlers"
	"github.com/jharrington22/recipes/internal/migrate"
)

func main() {
	ctx := context.Background()

	d, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer d.Pool.Close()

	if os.Getenv("AUTO_MIGRATE") == "true" {
		if err := migrate.ApplyDirectory(ctx, d.Pool, "migrations"); err != nil {
			log.Fatalf("auto-migrate failed: %v", err)
		}
		log.Printf("auto-migrate: ok")
	}

	r := gin.Default()
	r.Static("/static", "web/static")

	renderer, err := handlers.NewTemplateRenderer("web/templates/*.html")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	// Attach auth middleware to set user context if cookie exists (non-blocking)
	r.Use(func(c *gin.Context) {
		// Attempt auth; do not abort if missing
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
	recipesHTML := &handlers.RecipesHTML{DB: d, R: renderer}
	favHTML := &handlers.FavoritesHTML{DB: d, R: renderer}
	mealHTML := &handlers.MealPlanHTML{DB: d, R: renderer}

	r.GET("/", recipesHTML.Home)
	r.GET("/recipes", recipesHTML.List)
	r.GET("/recipes/:slug", recipesHTML.Detail)

	r.GET("/login", authHTML.LoginPage)
	r.POST("/login", authHTML.LoginPost)
	r.GET("/register", authHTML.RegisterPage)
	r.POST("/register", authHTML.RegisterPost)
	r.POST("/logout", authHTML.Logout)

	r.POST("/recipes/:id/favorite", recipesHTML.ToggleFavorite)

	r.GET("/favorites", favHTML.List)
	r.GET("/mealplan", mealHTML.Page)
	r.POST("/mealplan", mealHTML.Set)
	r.GET("/shopping", mealHTML.ShoppingList)

	// JSON API (authenticated group)
	api := &handlers.API{DB: d}
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
	s := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	log.Fatal(s.ListenAndServe())
}
