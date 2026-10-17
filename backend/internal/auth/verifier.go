package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	Type            string   `json:"typ"`
	AuthorizedParty string   `json:"azp"`
	Username        string   `json:"preferred_username"`
	Methods         []string `json:"amr"`
	RealmAccess     struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

func (c Claims) HasRole(role string) bool {
	for _, value := range c.RealmAccess.Roles {
		if value == role {
			return true
		}
	}
	return false
}

func (c Claims) StrongAuthentication() bool {
	password, otp := false, false
	for _, method := range c.Methods {
		switch method {
		case "passkey":
			return true
		case "pwd":
			password = true
		case "otp":
			otp = true
		}
	}
	return password && otp
}

type Verifier struct {
	issuer             string
	client             *http.Client
	mu                 sync.Mutex
	keys               map[string]*rsa.PublicKey
	expires, refreshed time.Time
}

func NewVerifier(issuer string) *Verifier {
	return &Verifier{issuer: issuer, client: &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	if v.issuer == "" || len(token) > 16384 {
		return nil, errors.New("invalid access token")
	}
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 128 {
			return nil, errors.New("missing signing key identifier")
		}
		return v.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.issuer), jwt.WithAudience("exam-api"), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(30*time.Second))
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid access token")
	}
	if claims.Subject == "" || claims.Type != "Bearer" || (claims.AuthorizedParty != "exam-web" && claims.AuthorizedParty != "exam-postman") {
		return nil, errors.New("token is not an authorized API access token")
	}
	return claims, nil
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	if key := v.keys[kid]; key != nil && now.Before(v.expires) {
		return key, nil
	}
	if now.Sub(v.refreshed) < 5*time.Second {
		return nil, errors.New("unknown signing key")
	}
	v.refreshed = now
	var discovery struct {
		Issuer string `json:"issuer"`
		JWKS   string `json:"jwks_uri"`
	}
	if err := v.getJSON(ctx, strings.TrimRight(v.issuer, "/")+"/.well-known/openid-configuration", &discovery); err != nil {
		return nil, err
	}
	issuerURL, issuerErr := url.Parse(v.issuer)
	keyURL, err := url.Parse(discovery.JWKS)
	if issuerErr != nil || err != nil || discovery.Issuer != v.issuer || keyURL.Host == "" || keyURL.Scheme != issuerURL.Scheme || keyURL.Host != issuerURL.Host || keyURL.User != nil || keyURL.Fragment != "" {
		return nil, errors.New("untrusted signing-key endpoint")
	}
	var set struct {
		Keys []struct {
			KID       string `json:"kid"`
			Type      string `json:"kty"`
			Use       string `json:"use"`
			Algorithm string `json:"alg"`
			N         string `json:"n"`
			E         string `json:"e"`
		} `json:"keys"`
	}
	if err := v.getJSON(ctx, keyURL.String(), &set); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, candidate := range set.Keys {
		if candidate.Type != "RSA" || (candidate.Use != "" && candidate.Use != "sig") || (candidate.Algorithm != "" && candidate.Algorithm != "RS256") || candidate.KID == "" {
			continue
		}
		nBytes, nErr := base64.RawURLEncoding.DecodeString(candidate.N)
		eBytes, eErr := base64.RawURLEncoding.DecodeString(candidate.E)
		n, e := new(big.Int).SetBytes(nBytes), new(big.Int).SetBytes(eBytes)
		if nErr != nil || eErr != nil || n.BitLen() < 2048 || n.BitLen() > 8192 || !e.IsInt64() || e.Int64() < 3 || e.Int64() > 2147483647 || e.Int64()%2 == 0 {
			continue
		}
		if keys[candidate.KID] != nil {
			return nil, errors.New("duplicate signing key identifier")
		}
		keys[candidate.KID] = &rsa.PublicKey{N: n, E: int(e.Int64())}
	}
	key := keys[kid]
	if key == nil {
		return nil, errors.New("unknown signing key")
	}
	v.keys, v.expires = keys, now.Add(5*time.Minute)
	return key, nil
}

func (v *Verifier) getJSON(ctx context.Context, endpoint string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("identity endpoint returned %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024))
	if err := decoder.Decode(result); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("invalid identity response")
	}
	return nil
}
