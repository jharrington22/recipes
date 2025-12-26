package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/migrate"
)

type SeedRecipe struct {
	Title        string
	Slug         string
	Category     string
	Description  string
	Instructions string
	Ingredients  []string
}

func main() {
	ctx := context.Background()
	d, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer d.Pool.Close()

	// Ensure schema exists
	if err := migrate.ApplyDirectory(ctx, d.Pool, "migrations"); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Println("seeding...")

	// demo user
	email := "demo@example.com"
	pw := "password123"

	hash, _ := auth.HashPassword(pw)
	_, _ = d.Pool.Exec(ctx, `
INSERT INTO users(email, password_hash) VALUES ($1,$2)
ON CONFLICT (email) DO NOTHING
`, email, hash)

	recipes := []SeedRecipe{
		{
			Title:        "Greek Yogurt Berry Bowl",
			Slug:         "greek-yogurt-berry-bowl",
			Category:     "breakfast",
			Description:  "Fast, high-protein breakfast bowl with berries and granola.",
			Instructions: "Spoon yogurt into a bowl. Top with berries and granola. Drizzle honey if you like.",
			Ingredients:  []string{"greek yogurt", "blueberries", "strawberries", "granola", "honey"},
		},
		{
			Title:        "Avocado Toast with Egg",
			Slug:         "avocado-toast-with-egg",
			Category:     "breakfast",
			Description:  "Creamy avocado on toast with a jammy egg.",
			Instructions: "Toast bread. Mash avocado with salt and lemon. Top with egg and pepper.",
			Ingredients:  []string{"bread", "avocado", "egg", "lemon", "salt", "black pepper"},
		},
		{
			Title:        "Chicken Caesar Salad",
			Slug:         "chicken-caesar-salad",
			Category:     "lunch",
			Description:  "Classic Caesar with roasted chicken and crunchy croutons.",
			Instructions: "Toss romaine with dressing. Add chicken, croutons, parmesan. Finish with pepper.",
			Ingredients:  []string{"romaine", "chicken", "caesar dressing", "croutons", "parmesan", "black pepper"},
		},
		{
			Title:        "Tomato Basil Pasta",
			Slug:         "tomato-basil-pasta",
			Category:     "dinner",
			Description:  "Simple weeknight pasta with tomato, garlic, and basil.",
			Instructions: "Cook pasta. Sauté garlic, add tomatoes, simmer. Toss with pasta, basil, parmesan.",
			Ingredients:  []string{"pasta", "tomato", "garlic", "olive oil", "basil", "parmesan", "salt"},
		},
	}

	tx, err := d.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, r := range recipes {
		var rid int64
		err := tx.QueryRow(ctx, `
INSERT INTO recipes(title, slug, category, description, instructions, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$6)
ON CONFLICT (slug) DO UPDATE SET
  title=EXCLUDED.title,
  category=EXCLUDED.category,
  description=EXCLUDED.description,
  instructions=EXCLUDED.instructions,
  updated_at=EXCLUDED.updated_at
RETURNING id
`, r.Title, r.Slug, r.Category, r.Description, r.Instructions, time.Now()).Scan(&rid)
		if err != nil {
			log.Fatalf("recipe %s: %v", r.Slug, err)
		}

		for _, ing := range r.Ingredients {
			ing = strings.ToLower(strings.TrimSpace(ing))
			if ing == "" {
				continue
			}
			var iid int64
			err := tx.QueryRow(ctx, `
INSERT INTO ingredients(name) VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name
RETURNING id
`, ing).Scan(&iid)
			if err != nil {
				log.Fatal(err)
			}

			_, err = tx.Exec(ctx, `
INSERT INTO recipe_ingredients(recipe_id, ingredient_id, quantity, unit)
VALUES ($1,$2,NULL,NULL)
ON CONFLICT (recipe_id, ingredient_id) DO NOTHING
`, rid, iid)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("seed complete")
	log.Printf("demo user: %s / %s", email, pw)

	_ = os.Stdout
}
