package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jharrington22/recipes/internal/auth"
	"github.com/jharrington22/recipes/internal/db"
)

type AuthHTML struct {
	DB *db.DB
	R  *TemplateRenderer
}

func (h *AuthHTML) RegisterPage(c *gin.Context) {
	h.R.Render(c, "register.html", gin.H{"Title": "Register"})
}

func (h *AuthHTML) LoginPage(c *gin.Context) {
	h.R.Render(c, "login.html", gin.H{"Title": "Login"})
}

func (h *AuthHTML) Logout(c *gin.Context) {
	// expire cookie
	c.SetCookie(auth.CookieName, "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHTML) RegisterPost(c *gin.Context) {
	ctx := context.Background()
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	pw := c.PostForm("password")

	if email == "" || len(pw) < 8 {
		h.R.Render(c, "register.html", gin.H{"Title": "Register", "Error": "Email and password (>=8 chars) required"})
		return
	}

	hash, err := auth.HashPassword(pw)
	if err != nil {
		h.R.Render(c, "register.html", gin.H{"Title": "Register", "Error": "Failed to create account"})
		return
	}

	var userID int64
	err = h.DB.Pool.QueryRow(ctx, `
INSERT INTO users(email, password_hash) VALUES ($1,$2)
RETURNING id
`, email, hash).Scan(&userID)
	if err != nil {
		h.R.Render(c, "register.html", gin.H{"Title": "Register", "Error": "Email already exists"})
		return
	}

	tok, _ := auth.Sign(userID, email)
	c.SetCookie(auth.CookieName, tok, int((7 * 24 * time.Hour).Seconds()), "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHTML) LoginPost(c *gin.Context) {
	ctx := context.Background()
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	pw := c.PostForm("password")

	var userID int64
	var hash string
	err := h.DB.Pool.QueryRow(ctx, `SELECT id, password_hash FROM users WHERE email=$1`, email).Scan(&userID, &hash)
	if err != nil || !auth.CheckPassword(hash, pw) {
		h.R.Render(c, "login.html", gin.H{"Title": "Login", "Error": "Invalid credentials"})
		return
	}

	tok, _ := auth.Sign(userID, email)
	c.SetCookie(auth.CookieName, tok, int((7 * 24 * time.Hour).Seconds()), "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}
