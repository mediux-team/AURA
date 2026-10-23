package routes_auth

import (
	"aura/config"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/oauth2"
)

func TestOIDCLoginUsesPKCE(t *testing.T) {
	// A fake identity provider that remembers the code verifier it was sent
	var gotVerifier string
	mux := http.NewServeMux()
	idp := httptest.NewServer(mux)
	t.Cleanup(idp.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 idp.URL,
			"authorization_endpoint": idp.URL + "/authorize",
			"token_endpoint":         idp.URL + "/token",
			"jwks_uri":               idp.URL + "/jwks",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotVerifier = r.PostForm.Get("code_verifier")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "token_type": "Bearer"})
	})

	previous := config.Current
	t.Cleanup(func() {
		config.Current = previous
		_ = InitOIDC(context.Background())
	})
	config.Current.Auth.Enabled = true
	config.Current.Auth.OIDC = config.Config_Auth_OIDC{
		Enabled:      true,
		IssuerURL:    idp.URL,
		ClientID:     "aura",
		ClientSecret: "secret",
		RedirectURLs: []string{"http://aura.test/api/auth/oidc/callback"},
	}
	if err := InitOIDC(context.Background()); err.Message != "" {
		t.Fatalf("InitOIDC: %s", err.Message)
	}

	// The login sends the S256 challenge of the verifier it keeps in a cookie
	login := httptest.NewRecorder()
	OIDCLoginRedirect(login, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil))
	location, err := url.Parse(login.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location: %v", err)
	}
	cookies := map[string]*http.Cookie{}
	for _, cookie := range login.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
	verifier := cookies[oidcVerifierCookieName]
	if verifier == nil || verifier.Value == "" || !verifier.HttpOnly {
		t.Fatalf("verifier cookie = %+v", verifier)
	}
	query := location.Query()
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") != oauth2.S256ChallengeFromVerifier(verifier.Value) {
		t.Errorf("challenge = %q (%q)", query.Get("code_challenge"), query.Get("code_challenge_method"))
	}

	// The callback sends the verifier with the code
	callback := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=abc&state="+query.Get("state"), nil)
	for _, cookie := range cookies {
		callback.AddCookie(cookie)
	}
	OIDCCallback(httptest.NewRecorder(), callback)
	if gotVerifier != verifier.Value {
		t.Errorf("token request code_verifier = %q, want %q", gotVerifier, verifier.Value)
	}

	// Without the verifier cookie the code is never exchanged
	gotVerifier = ""
	callback = httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?code=abc&state="+query.Get("state"), nil)
	callback.AddCookie(cookies[oidcStateCookieName])
	response := httptest.NewRecorder()
	OIDCCallback(response, callback)
	if gotVerifier != "" || response.Header().Get("Location") != "/login?error=oidc_failed" {
		t.Errorf("without verifier: exchanged = %v, Location = %q", gotVerifier != "", response.Header().Get("Location"))
	}
}
