package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tewan0/FairPick/config"
	"github.com/Tewan0/FairPick/routes"
	"github.com/gofiber/fiber/v2"
)

func TestHealthDBEndpointDoesNotExposePassword(t *testing.T) {
	secretPassword := "SUPER_SECRET_PASSWORD_12345"
	cfg := &config.Config{
		Port:       "8080",
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: secretPassword,
		DBName:     "fairpick",
		DBSSLMode:  "disable",
	}

	app := fiber.New()
	routes.Setup(app, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/health/db", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("unexpected error testing /api/health/db: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	bodyStr := string(bodyBytes)

	if strings.Contains(bodyStr, secretPassword) {
		t.Fatalf("security violation: response body exposed DB_PASSWORD: %s", bodyStr)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v (body: %s)", err, bodyStr)
	}

	if _, exists := payload["password"]; exists {
		t.Fatalf("security violation: response JSON contains password field")
	}
	if _, exists := payload["dsn"]; exists {
		t.Fatalf("security violation: response JSON contains dsn field")
	}

	t.Logf("GET /api/health/db status=%d response=%s", resp.StatusCode, bodyStr)
}

func TestHealthDBRealConnection(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	app := fiber.New()
	routes.Setup(app, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/health/db", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("unexpected error testing /api/health/db: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	bodyStr := string(bodyBytes)

	if cfg.DBPassword != "" && strings.Contains(bodyStr, cfg.DBPassword) {
		t.Fatalf("security violation: response body exposed DB_PASSWORD")
	}

	t.Logf("Real DB Health Check: status=%d response=%s", resp.StatusCode, bodyStr)
}


