package config

import (
	"net/url"
	"testing"
)

func TestDatabaseCredentialsAreEncoded(t *testing.T) {
	t.Setenv("EXAM_READER_PASSWORD", "a:@/?# safe")
	t.Setenv("POSTGRES_PORT", "5433")
	s, err := DatabaseURL("exam_reader", "EXAM_READER_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	if password != "a:@/?# safe" || u.Hostname() != "127.0.0.1" {
		t.Fatal("credentials changed URL structure")
	}
}

func TestIssuerAndProductionValidation(t *testing.T) {
	t.Setenv("DATABASE_READER_URL", "postgres://reader@localhost/exams")
	t.Setenv("DATABASE_ADMIN_URL", "postgres://admin@localhost/exams")
	t.Setenv("DATABASE_INVIGILATOR_URL", "postgres://invigilator@localhost/exams")
	t.Setenv("APP_ENV", "development")
	t.Setenv("TLS_CERT", "")
	t.Setenv("TLS_KEY", "")
	for _, issuer := range []string{"http://identity.example/realms/exams", "http://localhost.evil/realms/exams", "https://user:password@identity.example/realms/exams", "file:///tmp/identity", "https://identity.example/?token=secret"} {
		t.Setenv("OIDC_ISSUER", issuer)
		if _, err := Load(); err == nil {
			t.Errorf("accepted unsafe issuer %q", issuer)
		}
	}
	t.Setenv("OIDC_ISSUER", "http://localhost:8081/realms/exams")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted plaintext services")
	}
}
