package handlers

import (
	"context"
	"time"

	"github.com/Tewan0/FairPick/config"
	"github.com/Tewan0/FairPick/database"
	"github.com/gofiber/fiber/v2"
)

type HealthHandler struct {
	cfg *config.Config
}

func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{cfg: cfg}
}

func (h *HealthHandler) CheckDB(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()

	// If DB was not initialized at startup, attempt to connect safely without exposing secrets
	if database.DB == nil {
		if _, err := database.Connect(h.cfg); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":   "unhealthy",
				"database": "disconnected",
				"message":  "Unable to establish connection to PostgreSQL database",
			})
		}
	}

	if err := database.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":   "unhealthy",
			"database": "disconnected",
			"message":  "Database ping failed",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "healthy",
		"database": "connected",
		"db_name":  h.cfg.DBName,
		"host":     h.cfg.DBHost,
		"port":     h.cfg.DBPort,
	})
}

