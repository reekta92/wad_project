package provision

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPrivateCredentialsAreStableAndConfigurationBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "demo-accounts.json")
	issuer := "http://localhost:8081/realms/exams"
	first, err := loadState(path, issuer)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadState(path, issuer)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("repeat provisioning changed credentials")
	}
	for _, path := range []string{filepath.Dir(path), path} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Fatal("credentials are not private")
		}
	}
	for i, a := range first.Accounts {
		if a.Username != demoNames[i] || a.Password == a.TOTPSecret || a.Password == a.Tag {
			t.Fatal("incorrect demo credential generation")
		}
		for _, b := range first.Accounts[i+1:] {
			if a.Password == b.Password || a.TOTPSecret == b.TOTPSecret || a.Tag == b.Tag {
				t.Fatal("demo credentials were reused")
			}
		}
	}
	if _, err := loadState(path, "http://localhost:8082/realms/exams"); err == nil {
		t.Fatal("adopted credentials from another issuer")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadState(path, issuer); err == nil {
		t.Fatal("accepted world-readable credentials")
	}
}

func TestProvisioningPreservesAndDoesNotAdoptExistingIdentities(t *testing.T) {
	account := Account{Username: "student.demo", Tag: "owned-nonce", Subject: "known-subject"}
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"owned existing identity", `[{"id":"known-subject","username":"student.demo","attributes":{"exams-demo-id":["owned-nonce"]}}]`, true},
		{"unrelated username", `[{"id":"other-subject","username":"student.demo","attributes":{"exams-demo-id":["other-nonce"]}}]`, false},
		{"deleted identity", `[]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &provider{base: "https://identity.example", realm: "exams", client: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" {
					t.Fatal("repeat provisioning attempted to change credentials or roles")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{}}, nil
			})}}
			id, err := p.account(context.Background(), account, 0, true)
			if (err == nil) != tc.ok || (tc.ok && id != account.Subject) {
				t.Fatalf("unexpected ownership result: %s %v", id, err)
			}
		})
	}
}

func TestRealmTemplateHasDistinctStrongAuthenticationMethods(t *testing.T) {
	body, err := os.ReadFile("../../../infrastructure/exams-realm.json")
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Passkeys bool   `json:"webAuthnPolicyPasswordlessPasskeysEnabled"`
		UV       string `json:"webAuthnPolicyPasswordlessUserVerificationRequirement"`
		Flows    []flow `json:"authenticationFlows"`
		Clients  []struct {
			ID            string            `json:"clientId"`
			PasswordGrant bool              `json:"directAccessGrantsEnabled"`
			Implicit      bool              `json:"implicitFlowEnabled"`
			Attributes    map[string]string `json:"attributes"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(body, &template); err != nil {
		t.Fatal(err)
	}
	if template.Passkeys || template.UV != "required" || len(template.Flows) != 4 {
		t.Fatal("passkey verification or explicit method separation is missing")
	}
	password, otp, passkey := false, false, false
	for _, f := range template.Flows {
		for _, e := range f.Executions {
			switch e.Provider {
			case "auth-password-form":
				password = e.Requirement == "REQUIRED" && e.Config == "exams-amr-pwd"
			case "auth-otp-form":
				otp = e.Requirement == "REQUIRED" && e.Config == "exams-amr-otp"
			case "webauthn-authenticator-passwordless":
				passkey = e.Requirement == "ALTERNATIVE" && e.Config == "exams-amr-passkey"
			}
		}
	}
	if !password || !otp || !passkey {
		t.Fatal("a login branch lacks the required method proof")
	}
	if len(template.Clients) != 2 {
		t.Fatal("browser and Postman clients are required")
	}
	for _, c := range template.Clients {
		if c.PasswordGrant || c.Implicit || c.Attributes["pkce.code.challenge.method"] != "S256" {
			t.Fatal("unsafe application grant configuration")
		}
	}
}

func TestInitialOwnershipToleratesProviderEmailNormalization(t *testing.T) {
	account := Account{Username: "student.demo", Tag: "MixedCASEnonce"}
	p := &provider{base: "https://identity.example", realm: "exams", client: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		if strings.Contains(r.URL.Path, "/users") && r.Method == "GET" {
			body = `[{"id":"owned-subject","username":"student.demo","email":"student.demo.mixedcasenonce@example.test"}]`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}}
	id, err := p.account(context.Background(), account, 0, false)
	if err != nil || id != "owned-subject" {
		t.Fatalf("provider-normalized ownership marker rejected: %v", err)
	}
}
