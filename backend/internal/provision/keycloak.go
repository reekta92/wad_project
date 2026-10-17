package provision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type provider struct {
	base, realm, token string
	client             *http.Client
}

func (p *provider) request(ctx context.Context, method, path string, body, result any) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base+"/admin/realms/"+url.PathEscape(p.realm)+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("identity administration is unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("identity administration %s %s returned %d", method, strings.Split(path, "?")[0], resp.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(result)
	}
	return nil
}

func connect(ctx context.Context, issuer, password string) (*provider, error) {
	i := strings.LastIndex(issuer, "/realms/")
	if i < 0 || password == "" {
		return nil, fmt.Errorf("issuer and KEYCLOAK_ADMIN_PASSWORD are required")
	}
	p := &provider{base: issuer[:i], realm: issuer[i+8:], client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
	form := url.Values{"grant_type": {"password"}, "client_id": {"admin-cli"}, "username": {"local-admin"}, "password": {password}}
	req, err := http.NewRequestWithContext(ctx, "POST", p.base+"/realms/master/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("identity provider is unavailable")
	}
	defer resp.Body.Close()
	var result struct {
		Token string `json:"access_token"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result) != nil || result.Token == "" {
		return nil, fmt.Errorf("provider operator login failed; check the existing provider administrator credentials")
	}
	p.token = result.Token
	return p, nil
}

type flow struct {
	Alias      string      `json:"alias"`
	Executions []execution `json:"authenticationExecutions"`
}

type execution struct {
	Provider    string `json:"authenticator"`
	Flow        string `json:"flowAlias"`
	Requirement string `json:"requirement"`
	Config      string `json:"authenticatorConfig"`
	Priority    int    `json:"priority"`
}

type existingExecution struct {
	ID       string `json:"id"`
	Provider string `json:"providerId"`
	Name     string `json:"displayName"`
	Level    int    `json:"level"`
}

func (p *provider) configure(ctx context.Context, template []byte) error {
	var realm map[string]any
	var spec struct {
		Realm   string `json:"realm"`
		Flows   []flow `json:"authenticationFlows"`
		Configs []struct {
			Alias  string            `json:"alias"`
			Config map[string]string `json:"config"`
		} `json:"authenticatorConfig"`
		Clients []map[string]any `json:"clients"`
		Actions []map[string]any `json:"requiredActions"`
	}
	if json.Unmarshal(template, &realm) != nil || json.Unmarshal(template, &spec) != nil || spec.Realm != p.realm || len(spec.Flows) == 0 {
		return fmt.Errorf("invalid application realm template")
	}
	var known []struct {
		Alias string `json:"alias"`
	}
	if err := p.request(ctx, "GET", "/authentication/flows", nil, &known); err != nil {
		return err
	}
	found := false
	for _, f := range known {
		found = found || f.Alias == spec.Flows[0].Alias
	}
	if !found {
		if err := p.request(ctx, "POST", "/authentication/flows", map[string]any{"alias": spec.Flows[0].Alias, "providerId": "basic-flow", "topLevel": true, "builtIn": false}, nil); err != nil {
			return err
		}
	}
	for _, f := range spec.Flows {
		path := "/authentication/flows/" + url.PathEscape(f.Alias) + "/executions"
		var entries []existingExecution
		if err := p.request(ctx, "GET", path, nil, &entries); err != nil {
			return err
		}
		// Refuse an unknown branch rather than silently weaken an edited login policy.
		for _, entry := range entries {
			if entry.Level != 0 {
				continue
			}
			matches := 0
			for _, want := range f.Executions {
				if (want.Flow != "" && want.Flow == entry.Name) || (want.Provider != "" && want.Provider == entry.Provider) {
					matches++
				}
			}
			if matches != 1 {
				return fmt.Errorf("unexpected authentication execution in %s; existing policy was preserved", f.Alias)
			}
		}
		for _, want := range f.Executions {
			id := ""
			for _, entry := range entries {
				if entry.Level == 0 && ((want.Flow != "" && want.Flow == entry.Name) || (want.Provider != "" && want.Provider == entry.Provider)) {
					if id != "" {
						return fmt.Errorf("duplicate authentication execution in %s", f.Alias)
					}
					id = entry.ID
				}
			}
			if id == "" {
				endpoint := path + "/execution"
				body := map[string]any{"provider": want.Provider, "priority": want.Priority}
				if want.Flow != "" {
					endpoint = path + "/flow"
					body = map[string]any{"alias": want.Flow, "type": "basic-flow", "priority": want.Priority}
				}
				if err := p.request(ctx, "POST", endpoint, body, nil); err != nil {
					return err
				}
				var updated []existingExecution
				if err := p.request(ctx, "GET", path, nil, &updated); err != nil {
					return err
				}
				for _, entry := range updated {
					if entry.Level == 0 && ((want.Flow != "" && want.Flow == entry.Name) || (want.Provider != "" && want.Provider == entry.Provider)) {
						id = entry.ID
					}
				}
			}
			if id == "" {
				return fmt.Errorf("authentication execution was not created")
			}
			if err := p.request(ctx, "PUT", path, map[string]any{"id": id, "requirement": want.Requirement, "priority": want.Priority}, nil); err != nil {
				return err
			}
			if want.Config != "" {
				configured := false
				for _, cfg := range spec.Configs {
					if cfg.Alias == want.Config {
						if err := p.request(ctx, "POST", "/authentication/executions/"+url.PathEscape(id)+"/config", cfg, nil); err != nil {
							return err
						}
						configured = true
					}
				}
				if !configured {
					return fmt.Errorf("missing authentication execution configuration")
				}
			}
		}
	}
	for _, client := range spec.Clients {
		name, _ := client["clientId"].(string)
		var existing []struct {
			ID string `json:"id"`
		}
		if err := p.request(ctx, "GET", "/clients?clientId="+url.QueryEscape(name), nil, &existing); err != nil {
			return err
		}
		switch len(existing) {
		case 0:
			if err := p.request(ctx, "POST", "/clients", client, nil); err != nil {
				return err
			}
		case 1:
			if err := p.request(ctx, "PUT", "/clients/"+url.PathEscape(existing[0].ID), client, nil); err != nil {
				return err
			}
		default:
			return fmt.Errorf("ambiguous application client")
		}
	}
	for _, action := range spec.Actions {
		alias, _ := action["alias"].(string)
		if err := p.request(ctx, "PUT", "/authentication/required-actions/"+url.PathEscape(alias), action, nil); err != nil {
			return err
		}
	}
	for _, key := range []string{"clients", "roles", "authenticationFlows", "authenticatorConfig", "requiredActions"} {
		delete(realm, key)
	}
	return p.request(ctx, "PUT", "", realm, nil)
}
