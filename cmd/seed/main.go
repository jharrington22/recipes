package main

import (
	"context"
	"log"

	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
	"github.com/jharrington22/recipes/internal/migrate"
)

// seed applies migrations and creates a demo login. Recipe content itself
// lives in Markdown files under recipes/ (see internal/recipes) and needs
// no seeding — just add or edit a .md file there.
func main() {
	ctx := context.Background()
	d, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer d.Pool.Close()

	if err := migrate.ApplyDirectory(ctx, d.Pool, "migrations"); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Println("seeding...")

	email := "demo@example.com"
	pw := "password123"

	hash, err := auth.HashPassword(pw)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	if _, err := d.Pool.Exec(ctx, `
INSERT INTO users(email, password_hash) VALUES ($1,$2)
ON CONFLICT (email) DO NOTHING
`, email, hash); err != nil {
		log.Fatalf("insert demo user: %v", err)
	}

	log.Println("seed complete")
	log.Printf("demo user: %s / %s", email, pw)
}
