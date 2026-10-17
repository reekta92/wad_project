package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Address, ReaderURL, AdminURL, InvigilatorURL, Issuer, StaticDir, TLSCert, TLSKey string
}

func DatabaseURL(role, passwordName string) (string, error) {
	if password := os.Getenv(passwordName); password != "" {
		port := value("POSTGRES_PORT", "5433")
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("POSTGRES_PORT must be between 1 and 65535")
		}
		u := url.URL{Scheme: "postgres", Host: net.JoinHostPort("127.0.0.1", port), Path: "/exams", User: url.UserPassword(role, password)}
		q := url.Values{"sslmode": {"disable"}, "connect_timeout": {"5"}}
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	return "", fmt.Errorf("%s is required", passwordName)
}

func Load() (Config, error) {
	c := Config{Address: value("API_ADDRESS", "127.0.0.1:8080"), Issuer: value("OIDC_ISSUER", "http://localhost:8081/realms/exams"), StaticDir: value("STATIC_DIR", "../frontend/dist"), TLSCert: os.Getenv("TLS_CERT"), TLSKey: os.Getenv("TLS_KEY")}
	var err error
	c.ReaderURL = os.Getenv("DATABASE_READER_URL")
	if c.ReaderURL == "" {
		c.ReaderURL, err = DatabaseURL("exam_reader", "EXAM_READER_PASSWORD")
		if err != nil {
			return c, err
		}
	}
	c.AdminURL, err = connection("DATABASE_ADMIN_URL", "exam_admin", "EXAM_ADMIN_PASSWORD")
	if err != nil {
		return c, err
	}
	c.InvigilatorURL, err = connection("DATABASE_INVIGILATOR_URL", "exam_invigilator", "EXAM_INVIGILATOR_PASSWORD")
	if err != nil {
		return c, err
	}
	u, err := url.Parse(c.Issuer)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return c, fmt.Errorf("OIDC_ISSUER must be an absolute HTTP(S) issuer URL without credentials, query, or fragment")
	}
	if i := strings.LastIndex(u.Path, "/realms/"); i < 0 || u.Path[i+8:] == "" || strings.Contains(u.Path[i+8:], "/") {
		return c, fmt.Errorf("OIDC_ISSUER must identify a Keycloak realm")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if u.Scheme == "http" && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return c, fmt.Errorf("OIDC_ISSUER requires HTTPS outside loopback development")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return c, fmt.Errorf("TLS_CERT and TLS_KEY must be configured together")
	}
	if os.Getenv("APP_ENV") == "production" && (c.TLSCert == "" || u.Scheme != "https") {
		return c, fmt.Errorf("production requires API TLS and an HTTPS OIDC issuer")
	}
	return c, nil
}

func MigrationURL() (string, error) {
	return connection("DATABASE_OWNER_URL", "exam_owner", "EXAM_OWNER_PASSWORD")
}

func connection(name, role, passwordName string) (string, error) {
	if value := os.Getenv(name); value != "" {
		return value, nil
	}
	return DatabaseURL(role, passwordName)
}

func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
