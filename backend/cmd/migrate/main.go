package main

import (
	"log"

	"github.com/Tewan0/FairPick/config"
	"github.com/Tewan0/FairPick/database"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL database '%s' at %s:%s (check your .env)", cfg.DBName, cfg.DBHost, cfg.DBPort)
	}

	log.Printf("Running migrations on PostgreSQL database '%s' at %s:%s...", cfg.DBName, cfg.DBHost, cfg.DBPort)
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	log.Printf("Database migrations completed successfully!")
}

