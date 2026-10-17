package provision

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/reekta92/wad_project/backend/internal/store"
)

type Account struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	TOTPSecret string `json:"totp_secret"`
	Tag        string `json:"provisioning_tag"`
	Subject    string `json:"subject,omitempty"`
}

type demoState struct {
	Issuer   string    `json:"issuer"`
	Complete bool      `json:"complete"`
	Accounts []Account `json:"accounts"`
}

var demoNames = []string{"student.demo", "invigilator.demo", "admin.demo"}

func secret(n int) ([]byte, error) {
	value := make([]byte, n)
	_, err := rand.Read(value)
	return value, err
}

func loadState(path, issuer string) (demoState, error) {
	state := demoState{Issuer: issuer}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return state, fmt.Errorf("demo account file must be a private regular file")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return state, err
		}
		if err := json.Unmarshal(body, &state); err != nil {
			return state, fmt.Errorf("invalid demo account file")
		}
		if state.Issuer != issuer || len(state.Accounts) != len(demoNames) {
			return state, fmt.Errorf("demo account file belongs to another configuration")
		}
		for i, a := range state.Accounts {
			if a.Username != demoNames[i] || len(a.Password) < 32 || len(a.Tag) < 32 || len(a.TOTPSecret) != 32 {
				return state, fmt.Errorf("invalid demo account file")
			}
		}
		return state, nil
	} else if !os.IsNotExist(err) {
		return state, err
	}
	for _, name := range demoNames {
		password, err := secret(32)
		if err != nil {
			return state, err
		}
		tag, err := secret(24)
		if err != nil {
			return state, err
		}
		otp, err := secret(20)
		if err != nil {
			return state, err
		}
		state.Accounts = append(state.Accounts, Account{Username: name, Password: base64.RawURLEncoding.EncodeToString(password), Tag: base64.RawURLEncoding.EncodeToString(tag), TOTPSecret: base32.StdEncoding.EncodeToString(otp)})
	}
	return state, saveState(path, state)
}

func saveState(path string, state demoState) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("demo account directory must be private (mode 0700)")
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".demo-accounts-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(body, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

type providerUser struct {
	ID         string              `json:"id"`
	Username   string              `json:"username"`
	Attributes map[string][]string `json:"attributes"`
}

func (p *provider) account(ctx context.Context, account Account, index int, complete bool) (string, error) {
	var users []providerUser
	query := "/users?exact=true&username=" + url.QueryEscape(account.Username)
	if err := p.request(ctx, "GET", query, nil, &users); err != nil {
		return "", err
	}
	if len(users) == 0 {
		if complete || account.Subject != "" {
			return "", fmt.Errorf("demo identity %s was removed; refusing to recreate it", account.Username)
		}
		credentialData, err := json.Marshal(map[string]any{"subType": "totp", "digits": 6, "counter": 0, "period": 30, "algorithm": "HmacSHA1", "secretEncoding": "BASE32"})
		if err != nil {
			return "", err
		}
		secretData, err := json.Marshal(map[string]string{"value": account.TOTPSecret})
		if err != nil {
			return "", err
		}
		body := map[string]any{
			"username": account.Username, "enabled": true, "emailVerified": true,
			"email": account.Username + "." + account.Tag + "@example.test", "firstName": "Demo", "lastName": account.Username,
			"attributes": map[string][]string{"exams-demo-id": {account.Tag}},
			"credentials": []map[string]any{
				{"type": "password", "value": account.Password, "temporary": false},
				{"type": "otp", "userLabel": "Local demo authenticator", "credentialData": string(credentialData), "secretData": string(secretData)},
			},
		}
		if err := p.request(ctx, "POST", "/users", body, nil); err != nil {
			return "", err
		}
		if err := p.request(ctx, "GET", query, nil, &users); err != nil {
			return "", err
		}
	}
	if len(users) != 1 || users[0].Username != account.Username || !reflect.DeepEqual(users[0].Attributes["exams-demo-id"], []string{account.Tag}) || (account.Subject != "" && account.Subject != users[0].ID) {
		return "", fmt.Errorf("demo username %s belongs to a different identity; no credentials or roles were changed", account.Username)
	}
	id := users[0].ID
	if !complete {
		roles := []string{"member"}
		if index == 1 {
			roles = append(roles, "invigilator")
		}
		if index == 2 {
			roles = []string{"administrator"}
		}
		var values []map[string]any
		for _, name := range roles {
			var role map[string]any
			if err := p.request(ctx, "GET", "/roles/"+name, nil, &role); err != nil {
				return "", err
			}
			values = append(values, role)
		}
		if err := p.request(ctx, "POST", "/users/"+url.PathEscape(id)+"/role-mappings/realm", values, nil); err != nil {
			return "", err
		}
	}
	return id, nil
}

// Run provisions only the local demo realm and links its subjects using migration credentials.
func Run(ctx context.Context, issuer, ownerURL, operatorPassword, root string) error {
	u, err := url.Parse(issuer)
	if err != nil || u.Hostname() != "localhost" || u.Path != "/realms/exams" || os.Getenv("APP_ENV") == "production" {
		return fmt.Errorf("demo provisioning is restricted to the localhost exams realm outside production")
	}
	conn, err := pgx.Connect(ctx, ownerURL)
	if err != nil {
		return fmt.Errorf("application database is unavailable")
	}
	defer conn.Close(ctx)
	// Serialize provisioning across retries before reading or replacing the private credential file.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(92831403)`); err != nil {
		return fmt.Errorf("cannot lock demo provisioning")
	}
	var demo bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM seed_runs WHERE name = 'demo-v1')`).Scan(&demo); err != nil || !demo {
		return fmt.Errorf("run migrate and seed against the local demo database before provisioning")
	}
	p, err := connect(ctx, issuer, operatorPassword)
	if err != nil {
		return err
	}
	template, err := os.ReadFile(filepath.Join(root, "infrastructure", "exams-realm.json"))
	if err != nil {
		return err
	}
	if err := p.configure(ctx, template); err != nil {
		return err
	}
	path := filepath.Join(root, ".local", "demo-accounts.json")
	state, err := loadState(path, issuer)
	if err != nil {
		return err
	}
	for i := range state.Accounts {
		id, err := p.account(ctx, state.Accounts[i], i, state.Complete)
		if err != nil {
			return err
		}
		state.Accounts[i].Subject = id
		if err := saveState(path, state); err != nil {
			return err
		}
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cannot begin identity mapping transaction")
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, store.ScheduleLock); err != nil {
		return fmt.Errorf("cannot lock identity mapping")
	}
	for i, a := range state.Accounts {
		var personnel *int64
		if i == 1 {
			id := int64(1)
			personnel = &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app_users (issuer, subject, personnel_id) VALUES ($1, $2, $3) ON CONFLICT (issuer, subject) DO NOTHING`, issuer, a.Subject, personnel); err != nil {
			return fmt.Errorf("demo identity mapping conflicts with existing data; existing records were preserved")
		}
		var actual *int64
		if err := tx.QueryRow(ctx, `SELECT personnel_id FROM app_users WHERE issuer = $1 AND subject = $2`, issuer, a.Subject).Scan(&actual); err != nil || !reflect.DeepEqual(actual, personnel) {
			return fmt.Errorf("existing identity personnel mapping differs; refusing to overwrite it")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cannot commit identity mapping")
	}
	state.Complete = true
	return saveState(path, state)
}
