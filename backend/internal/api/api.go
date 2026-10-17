package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/reekta92/wad_project/backend/internal/auth"
	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/internal/store"
)

func New(cfg config.Config, databases *store.Databases) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true, BodyLimit: 1024 * 1024,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second,
		ErrorHandler: respondError,
	})
	app.Use(recover.New(), requestid.New())
	issuer, _ := url.Parse(cfg.Issuer)
	identityOrigin := ""
	if issuer != nil && issuer.Host != "" {
		identityOrigin = issuer.Scheme + "://" + issuer.Host
	}
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self'; connect-src 'self' "+identityOrigin+"; frame-src 'self' "+identityOrigin+"; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		return c.Next()
	})
	app.Get("/api/v1/health/live", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	app.Get("/api/v1/health/ready", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 4*time.Second)
		defer cancel()
		checks := fiber.Map{"database": "unavailable", "identity": "unavailable"}
		if databases.Ready(ctx) == nil {
			checks["database"] = "ok"
		}
		if identityReady(ctx, client, cfg.Issuer) {
			checks["identity"] = "ok"
		}
		status, label := 200, "ok"
		if checks["database"] != "ok" || checks["identity"] != "ok" {
			status, label = 503, "unavailable"
		}
		return c.Status(status).JSON(fiber.Map{"status": label, "checks": checks})
	})
	app.Get("/api/v1/schedule", func(c *fiber.Ctx) error {
		filter, err := scheduleFilter(string(c.Request().URI().QueryString()))
		if err != nil {
			return fiber.NewError(400, err.Error())
		}
		if err := requireJSON(c); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()
		body, err := store.Schedule(ctx, databases.Reader, filter)
		if err != nil {
			return fiber.NewError(503, "Schedule data is temporarily unavailable.")
		}
		c.Set("Vary", "Accept")
		c.Type("json")
		return c.Send(body)
	})
	app.Get("/api/v1/auth/config", func(c *fiber.Ctx) error {
		index := strings.LastIndex(cfg.Issuer, "/realms/")
		if index < 0 {
			return fiber.NewError(503, "Sign-in is temporarily unavailable.")
		}
		return c.JSON(fiber.Map{"issuer": cfg.Issuer, "url": cfg.Issuer[:index], "realm": cfg.Issuer[index+8:], "client_id": "exam-web"})
	})
	verify := authenticate(databases, auth.NewVerifier(cfg.Issuer))
	app.Get("/api/v1/me", verify, profile)
	businessRoutes(app, databases, verify)
	app.Use("/api", func(c *fiber.Ctx) error { return fiber.NewError(404, "API resource not found.") })
	app.Static("/", cfg.StaticDir)
	app.Get("*", func(c *fiber.Ctx) error {
		if filepath.Ext(c.Path()) != "" || c.Accepts("html") == "" {
			return fiber.ErrNotFound
		}
		return c.SendFile(filepath.Join(cfg.StaticDir, "index.html"))
	})
	return app
}

func identityReady(ctx context.Context, client *http.Client, issuer string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(issuer, "/")+"/.well-known/openid-configuration", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var document struct {
		Issuer string `json:"issuer"`
	}
	return resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&document) == nil && document.Issuer == issuer
}

func scheduleFilter(raw string) (store.ScheduleFilter, error) {
	f := store.ScheduleFilter{Page: 1, PageSize: 20}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return f, fmt.Errorf("Invalid query parameters.")
	}
	allowed := map[string]bool{"page": true, "page_size": true, "date_from": true, "date_to": true, "course_id": true, "department_id": true, "term": true, "q": true}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 || entries[0] == "" {
			return f, fmt.Errorf("Invalid or repeated query parameter: %s.", key)
		}
	}
	for key, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize, "term": &f.Term} {
		if value := values.Get(key); value != "" {
			n, err := strconv.ParseInt(value, 10, 32)
			if err != nil || n < 1 {
				return f, fmt.Errorf("%s must be a positive integer.", key)
			}
			*target = int(n)
		}
	}
	if f.PageSize > 100 {
		return f, fmt.Errorf("page_size must not exceed 100.")
	}
	if f.Term > 8 {
		return f, fmt.Errorf("term must be between 1 and 8.")
	}
	for key, target := range map[string]*int64{"course_id": &f.CourseID, "department_id": &f.DepartmentID} {
		if value := values.Get(key); value != "" {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || n < 1 {
				return f, fmt.Errorf("%s must be a positive integer.", key)
			}
			*target = n
		}
	}
	f.DateFrom, f.DateTo = values.Get("date_from"), values.Get("date_to")
	for _, date := range []string{f.DateFrom, f.DateTo} {
		if date != "" {
			if _, err := time.Parse(time.DateOnly, date); err != nil {
				return f, fmt.Errorf("Dates must use valid YYYY-MM-DD values.")
			}
		}
	}
	if f.DateFrom != "" && f.DateTo != "" && f.DateFrom > f.DateTo {
		return f, fmt.Errorf("date_from must not follow date_to.")
	}
	f.Query = strings.TrimSpace(values.Get("q"))
	if !validSearchText(f.Query) {
		return f, fmt.Errorf("q must contain valid UTF-8 text of at most 100 bytes, without NUL characters.")
	}
	return f, nil
}
