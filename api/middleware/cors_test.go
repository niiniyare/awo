package middleware_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"awo.so/awo/api/middleware"
)

func TestCORS_AllowedSubdomain(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CORS(middleware.CORSConfig{BaseDomain: "awo.so"}))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	tests := []struct {
		origin  string
		allowed bool
	}{
		{"https://acme.awo.so", true},
		{"https://awo.so", true},
		{"https://evil.com", false},
		{"https://notawo.so", false},
		{"https://sub.acme.awo.so", true},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", tt.origin)
		resp, _ := app.Test(req)

		got := resp.Header.Get("Access-Control-Allow-Origin")
		if tt.allowed && got != tt.origin {
			t.Errorf("origin %q: expected allowed, got %q", tt.origin, got)
		}
		if !tt.allowed && got != "" {
			t.Errorf("origin %q: expected blocked, got %q", tt.origin, got)
		}
	}
}

func TestCORS_ExplicitAllowlist(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CORS(middleware.CORSConfig{
		BaseDomain:     "awo.so",
		AllowedOrigins: []string{"http://localhost:3000"},
	}))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	resp, _ := app.Test(req)

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("expected localhost allowed, got %q", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CORS(middleware.CORSConfig{BaseDomain: "awo.so"}))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://tenant.awo.so")
	resp, _ := app.Test(req)

	if resp.StatusCode != 204 {
		t.Errorf("preflight: expected 204, got %d", resp.StatusCode)
	}
}
