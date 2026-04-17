package main

import (
	"log"
	"net/http"
	"os"

	"github.com/brian/config-generation/backend/db"
	"github.com/brian/config-generation/backend/handlers"
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

	router := handlers.NewRouter(database, []byte(jwtSecret))

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
