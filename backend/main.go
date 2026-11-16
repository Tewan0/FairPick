package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Tewan0/FairPick/config"
	"github.com/Tewan0/FairPick/database"
	"github.com/Tewan0/FairPick/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	runMigrate := flag.Bool("migrate", false, "Run database migrations before starting the server")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Attempt initial connection to PostgreSQL (do not log credentials)
	db, err := database.Connect(cfg)
	if err != nil {
		log.Printf("Warning: initial database connection failed (check your .env configuration)")
	} else {
		log.Printf("Connected to PostgreSQL database '%s' at %s:%s", cfg.DBName, cfg.DBHost, cfg.DBPort)

		disableAutoMigrate := strings.EqualFold(os.Getenv("AUTO_MIGRATE"), "false")
		if *runMigrate || !disableAutoMigrate {
			if err := database.Migrate(db); err != nil {
				log.Fatalf("Failed to run database migrations: %v", err)
			}
			log.Printf("Database migrations completed successfully")
		}
	}

	// Configure explicit HTTP timeouts and body size limit per GO-HTTP-001 and GO-HTTP-002
	app := fiber.New(fiber.Config{
		AppName:      "FairPick API",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    10 * 1024 * 1024, // 10 MB max request body
	})

	app.Use(recover.New())
	app.Use(logger.New())

	routes.SetupWithDB(app, cfg, db)

	log.Printf("Starting FairPick backend server on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
