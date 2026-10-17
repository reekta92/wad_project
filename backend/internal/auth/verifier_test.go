package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testVerifier(t *testing.T, key *rsa.PrivateKey) (*Verifier, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	v := NewVerifier("https://identity.example/realms/exams")
	v.client.Transport = testTransport(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		body := `{"issuer":"https://identity.example/realms/exams","jwks_uri":"https://identity.example/realms/exams/keys"}`
		if req.URL.Path == "/realms/exams/keys" {
			encoded, err := json.Marshal(map[string]any{"keys": []map[string]string{{"kid": "current", "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
			if err != nil {
				t.Fatal(err)
			}
			body = string(encoded)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	return v, &requests
}

func validClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{"iss": "https://identity.example/realms/exams", "aud": []string{"exam-api"}, "sub": "demo-subject", "exp": now.Add(5 * time.Minute).Unix(), "iat": now.Unix(), "nbf": now.Add(-time.Second).Unix(), "typ": "Bearer", "azp": "exam-web", "realm_access": map[string]any{"roles": []string{"administrator"}}, "amr": []string{"pwd", "otp"}}
}

func sign(t *testing.T, claims jwt.MapClaims, key *rsa.PrivateKey) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "current"
	result, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestJWTTrustBoundaries(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := testVerifier(t, key)
	claims, err := v.Verify(context.Background(), sign(t, validClaims(), key))
	if err != nil || !claims.HasRole("administrator") || !claims.StrongAuthentication() {
		t.Fatalf("valid administrator token rejected: %v", err)
	}
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"issuer", "iss", "https://attacker.example"},
		{"audience", "aud", []string{"account"}},
		{"subject", "sub", ""},
		{"ID token", "typ", "ID"},
		{"client", "azp", "unknown-client"},
		{"expired", "exp", time.Now().Add(-time.Minute).Unix()},
		{"not yet valid", "nbf", time.Now().Add(time.Minute).Unix()},
		{"future issuance", "iat", time.Now().Add(time.Minute).Unix()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := validClaims()
			body[tc.field] = tc.value
			if _, err := v.Verify(context.Background(), sign(t, body, key)); err == nil {
				t.Fatal("invalid access token accepted")
			}
		})
	}
	missingExpiry := validClaims()
	delete(missingExpiry, "exp")
	if _, err := v.Verify(context.Background(), sign(t, missingExpiry, key)); err == nil {
		t.Fatal("token without expiry accepted")
	}
	hmac := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
	hmac.Header["kid"] = "current"
	encoded, err := hmac.SignedString([]byte("test-only-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(context.Background(), encoded); err == nil {
		t.Fatal("algorithm confusion accepted")
	}
	parts := strings.Split(sign(t, validClaims(), key), ".")
	if parts[2][0] == 'a' {
		parts[2] = "b" + parts[2][1:]
	} else {
		parts[2] = "a" + parts[2][1:]
	}
	if _, err := v.Verify(context.Background(), strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered signature accepted")
	}
}

func TestConcurrentVerificationCachesTrustedKeys(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v, requests := testVerifier(t, key)
	token := sign(t, validClaims(), key)
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			if _, err := v.Verify(context.Background(), token); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if requests.Load() != 2 {
		t.Fatalf("expected one discovery/key fetch, got %d requests", requests.Load())
	}
}

func TestUntrustedKeyEndpointIsRejected(t *testing.T) {
	v := NewVerifier("https://identity.example/realms/exams")
	v.client.Transport = testTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "identity.example" {
			t.Fatal("followed untrusted JWKS endpoint")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"issuer":"https://identity.example/realms/exams","jwks_uri":"http://169.254.169.254/keys"}`))}, nil
	})
	if _, err := v.key(context.Background(), "current"); err == nil {
		t.Fatal("untrusted key endpoint accepted")
	}
}

func TestStrongAuthenticationMethods(t *testing.T) {
	for _, tc := range []struct {
		methods []string
		want    bool
	}{{[]string{"passkey"}, true}, {[]string{"pwd", "otp"}, true}, {[]string{"pwd"}, false}, {[]string{"otp"}, false}, {[]string{"unknown"}, false}, {nil, false}} {
		if got := (Claims{Methods: tc.methods}).StrongAuthentication(); got != tc.want {
			t.Errorf("methods %v: got %v", tc.methods, got)
		}
	}
}
