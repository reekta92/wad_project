package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/internal/store"
)

func TestScheduleQueryValidation(t *testing.T) {
	for _, raw := range []string{"page=0", "page_size=101", "page=2147483648", "page=1&page=2", "course_id=-1", "department_id=hello", "term=9", "date_from=2026-02-30", "date_from=2026-11-03&date_to=2026-11-01", "sort=secret", "q=", "q=%00", "q=%ff"} {
		if _, err := scheduleFilter(raw); err == nil {
			t.Errorf("accepted invalid query %q", raw)
		}
	}
	f, err := scheduleFilter("page=2&page_size=1&date_from=2026-11-02&department_id=1&term=5&q=Web")
	if err != nil {
		t.Fatal(err)
	}
	if f.Page != 2 || f.PageSize != 1 || f.DepartmentID != 1 || f.Term != 5 || f.Query != "Web" {
		t.Fatal("validated query lost filters")
	}
}

func TestLivenessAndUnknownAPINeverServeSPA(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("SPA marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(config.Config{StaticDir: root}, nil)
	for _, tc := range []struct {
		path   string
		status int
	}{{"/api/v1/health/live", 200}, {"/api/v1/missing", 404}, {"/api/v1/schedule?page=0", 400}} {
		resp, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.status || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") || strings.Contains(string(body), "SPA marker") {
			t.Fatalf("%s: %d %s", tc.path, resp.StatusCode, body)
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing content-type protection")
		}
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIdentityReadinessRequiresMatchingIssuer(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
		want   bool
	}{
		{`{"issuer":"http://localhost:8081/realms/exams"}`, 200, true},
		{`{"issuer":"https://other.example/realms/exams"}`, 200, false},
		{`<html>Not discovery</html>`, 200, false},
		{`{"issuer":"http://localhost:8081/realms/exams"}`, 503, false},
	} {
		client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}
		if got := identityReady(context.Background(), client, "http://localhost:8081/realms/exams"); got != tc.want {
			t.Errorf("discovery %q: got %v", tc.body, got)
		}
	}
}

func TestReadinessReportsUnavailableDependenciesWithoutCredentials(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://reader:private-password@127.0.0.1:1/exams?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	app := New(config.Config{Issuer: ":invalid", StaticDir: t.TempDir()}, &store.Databases{Reader: pool})
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/health/ready", nil), 5000)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 503 || strings.Contains(string(body), "private-password") {
		t.Fatalf("unsafe readiness response: %d %s", resp.StatusCode, body)
	}
}

func TestDevelopmentReadiness(t *testing.T) {
	if os.Getenv("RUN_DATABASE_TESTS") != "1" {
		t.Skip("requires the isolated PostgreSQL and Keycloak services")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ownerURL, err := config.MigrationURL()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background(), ownerURL); err != nil {
		t.Fatal(err)
	}
	databases, err := store.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer databases.Close()
	app := New(cfg, databases)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/health/ready", nil), 10000)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("development services are not ready: %d %s", resp.StatusCode, body)
	}
}

func TestProfileDoesNotTrustSubmittedRoles(t *testing.T) {
	app := New(config.Config{Issuer: "https://identity.example/realms/exams", StaticDir: t.TempDir()}, nil)
	for _, authorization := range []string{"", "Basic arbitrary", "Bearer invalid-token"} {
		req := httptest.NewRequest("GET", "/api/v1/me?role=administrator&personnel_id=1", nil)
		req.Header.Set("Authorization", authorization)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 || resp.Header.Get("WWW-Authenticate") == "" || resp.Header.Get("Location") != "" {
			t.Fatalf("profile did not reject unverified identity: %d", resp.StatusCode)
		}
	}
}

func TestAdministratorProfileRequiresVerifiedSecondFactor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = transport(func(req *http.Request) (*http.Response, error) {
		body := `{"issuer":"https://identity.example/realms/exams","jwks_uri":"https://identity.example/keys"}`
		if req.URL.Path == "/keys" {
			encoded, err := json.Marshal(map[string]any{"keys": []map[string]string{{"kid": "test", "kty": "RSA", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
			if err != nil {
				t.Fatal(err)
			}
			body = string(encoded)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	claims := jwt.MapClaims{"iss": "https://identity.example/realms/exams", "aud": []string{"exam-api"}, "sub": "administrator", "exp": time.Now().Add(time.Minute).Unix(), "typ": "Bearer", "azp": "exam-web", "realm_access": map[string]any{"roles": []string{"administrator"}}, "amr": []string{"pwd"}}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test"
	encoded, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	app := New(config.Config{Issuer: "https://identity.example/realms/exams", StaticDir: t.TempDir()}, nil)
	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+encoded)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 403 || !strings.Contains(string(body), "password plus TOTP") {
		t.Fatalf("weak administrator authentication was not rejected: %d %s", resp.StatusCode, body)
	}
}
