.PHONY: tidy test build run up down logs migrate seed validate-recipes

tidy:
	go mod tidy

test:
	go test ./...

build:
	go build ./...

validate-recipes:
	go run ./cmd/validate-recipes

run:
	DATABASE_URL='postgres://recipes:recipes@localhost:5432/recipes?sslmode=disable' \
	JWT_SECRET='dev-secret-change-me' \
	AUTO_MIGRATE='true' \
	go run ./cmd/api

up:
	podman-compose up -d --build

down:
	podman-compose down

logs:
	podman-compose logs -f

migrate:
	DATABASE_URL='postgres://recipes:recipes@localhost:5432/recipes?sslmode=disable' \
	go run ./cmd/seed >/dev/null

seed:
	DATABASE_URL='postgres://recipes:recipes@localhost:5432/recipes?sslmode=disable' \
	go run ./cmd/seed
