package api

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/internal/store"
)

func TestEveryBusinessOperationRequiresAuthentication(t *testing.T) {
	app := New(config.Config{Issuer: "https://identity.example/realms/exams", StaticDir: t.TempDir()}, nil)
	for resource := range resourceFilters {
		for _, operation := range []struct{ method, suffix string }{{"GET", ""}, {"GET", "/1"}, {"POST", ""}, {"PUT", "/1"}, {"DELETE", "/1"}} {
			response, err := app.Test(httptest.NewRequest(operation.method, "/api/v1/"+resource+operation.suffix, nil))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 401 || response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("unprotected operation: %s %s, status %d", operation.method, resource, response.StatusCode)
			}
		}
	}
}

func TestJSONInputRejectsMissingNullUnknownAndWrongTypes(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: respondError})
	app.Post("/", func(c *fiber.Ctx) error {
		var value store.ClassroomInput
		if err := decodeInput(c, &value, "name", "capacity", "type", "floor", "active"); err != nil {
			return err
		}
		if err := checkInput(value); err != nil {
			return err
		}
		return c.JSON(value)
	})
	for _, tc := range []struct {
		body, media string
		status      int
	}{
		{`{"name":"Room","capacity":20,"type":"lecture","floor":0,"active":false}`, "application/json", 200},
		{`{"name":"Room","capacity":20,"type":"lecture","floor":0}`, "application/json", 400},
		{`{"name":"Room","capacity":20,"type":"lecture","floor":null,"active":false}`, "application/json", 400},
		{`{"name":"Room","capacity":20,"type":"lecture","floor":0,"active":"false"}`, "application/json", 400},
		{`{"name":"Room","capacity":20,"type":"lecture","floor":0,"active":false,"role":"administrator"}`, "application/json", 400},
		{`{"name":" ","capacity":0,"type":"lecture","floor":0,"active":true}`, "application/json", 400},
		{`null`, "application/json", 400}, {`[]`, "application/json", 400}, {`{} {}`, "application/json", 400},
		{`{}`, "text/plain", 415}, {`{}`, "application/json; charset=iso-8859-1", 415},
	} {
		request := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", tc.media)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != tc.status {
			t.Errorf("%s: got %d, want %d", tc.body, response.StatusCode, tc.status)
		}
	}
}

func TestRepresentationExclusionsAndSanitizedDatabaseErrors(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: respondError})
	app.Get("/", func(c *fiber.Ctx) error {
		if err := requireJSON(c); err != nil {
			return err
		}
		return c.SendString("ok")
	})
	for _, tc := range []struct {
		accept string
		status int
	}{
		{"", 200}, {"*/*", 200}, {"application/json;q=.5,text/plain;q=1", 200},
		{"application/json;q=0,*/*;q=1", 406}, {"application/json;q=NaN", 406}, {"application/xml", 406},
	} {
		request := httptest.NewRequest("GET", "/", nil)
		request.Header.Set("Accept", tc.accept)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != tc.status {
			t.Errorf("Accept %q: %d", tc.accept, response.StatusCode)
		}
	}
	app.Get("/database", func(c *fiber.Ctx) error {
		return &pgconn.PgError{Code: "23503", Detail: "SECRET database row and connection password"}
	})
	response, err := app.Test(httptest.NewRequest("GET", "/database", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != 409 || strings.Contains(string(body), "SECRET") {
		t.Fatalf("unsafe conflict: %d %s", response.StatusCode, body)
	}
}

func TestResourceFilterValidation(t *testing.T) {
	for _, tc := range []struct{ resource, query string }{
		{"departments", "personnel_id=1"}, {"courses", "term=9"}, {"courses", "kind=other"}, {"classrooms", "active=1"},
		{"classrooms", "floor=x"}, {"sessions", "date_from=2026-02-30"}, {"sessions", "date_from=2026-11-04&date_to=2026-11-01"},
		{"personnel", "department_id=-1"}, {"exams", "session_id=0"}, {"invigilator-assignments", "page_size=101"},
		{"unavailability-requests", "status=unknown"}, {"unavailability-requests", "type=unknown"}, {"departments", "page=1&page=2"},
	} {
		if _, err := resourceFilter(tc.resource, tc.query); err == nil {
			t.Errorf("accepted %s?%s", tc.resource, tc.query)
		}
	}
	f, err := resourceFilter("classrooms", "active=false&floor=-1&min_capacity=10&page=2147483647&page_size=100")
	if err != nil || f.Active == nil || *f.Active || f.Floor == nil || *f.Floor != -1 || f.Page != 2147483647 {
		t.Fatalf("valid filter lost values: %+v %v", f, err)
	}
}
