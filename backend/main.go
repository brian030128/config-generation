package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/brian/config-generation/backend/db"
	"github.com/brian/config-generation/backend/handlers"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	database, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := seedAdmin(database); err != nil {
		log.Fatalf("failed to seed admin user: %v", err)
	}

	router := handlers.NewRouter(database, []byte(jwtSecret))

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}

func seedAdmin(database *sql.DB) error {
	username := os.Getenv("ADMIN_USERNAME")
	password := os.Getenv("ADMIN_PASSWORD")
	if username == "" || password == "" {
		log.Println("ADMIN_USERNAME/ADMIN_PASSWORD not set, skipping admin seed")
		return nil
	}

	var exists bool
	if err := database.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", username).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = database.Exec(
		"INSERT INTO users (username, display_name, password_hash, superuser) VALUES ($1, $2, $3, true)",
		username, "Administrator", string(hash),
	)
	if err != nil {
		return err
	}

	log.Printf("admin user %q created", username)
	return nil
}
