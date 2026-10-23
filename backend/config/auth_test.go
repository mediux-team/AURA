package config

import (
	"aura/logging"
	"context"
	"slices"
	"testing"
)

func TestValidateAuthLoginMethods(t *testing.T) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	oidc := Config_Auth_OIDC{Enabled: true, IssuerURL: "https://idp.example.com", ClientID: "aura", ClientSecret: "secret",
		RedirectURLs: []string{" https://aura.example.com/api/auth/oidc/callback ", "", "http://192.168.1.10:3000/"}}

	tests := []struct {
		name string
		auth Config_Auth
		want bool
	}{
		{"no way to log in", Config_Auth{Enabled: true}, false},
		{"the password needs a hash", Config_Auth{Enabled: true, PasswordEnabled: true}, false},
		{"only Single Sign-On", Config_Auth{Enabled: true, OIDC: oidc}, true},
		{"Single Sign-On needs a Redirect URL", Config_Auth{Enabled: true, OIDC: Config_Auth_OIDC{Enabled: true, IssuerURL: "https://idp.example.com", ClientID: "aura", ClientSecret: "secret"}}, false},
		{"auth off", Config_Auth{}, true},
	}
	for _, tt := range tests {
		auth := tt.auth
		if got := ValidateAuth(ctx, &auth); got != tt.want {
			t.Errorf("%s: valid = %v, want %v", tt.name, got, tt.want)
		}
	}

	// Redirect URLs are trimmed, get the callback path, and empty ones are removed
	auth := Config_Auth{Enabled: true, OIDC: oidc}
	ValidateAuth(ctx, &auth)
	if !slices.Equal(auth.OIDC.RedirectURLs, []string{"https://aura.example.com/api/auth/oidc/callback", "http://192.168.1.10:3000/api/auth/oidc/callback"}) {
		t.Errorf("RedirectURLs = %q", auth.OIDC.RedirectURLs)
	}
}

func TestWithOIDCCallbackPath(t *testing.T) {
	tests := map[string]string{
		"https://aura.example.com":                         "https://aura.example.com/api/auth/oidc/callback",
		"http://192.168.1.10:3000/":                        "http://192.168.1.10:3000/api/auth/oidc/callback",
		"https://example.com/aura":                         "https://example.com/aura/api/auth/oidc/callback",
		"https://aura.example.com/api/auth":                "https://aura.example.com/api/auth/oidc/callback",
		"https://aura.example.com/api/auth/oidc/callback/": "https://aura.example.com/api/auth/oidc/callback",
		"  ": "",
	}
	for in, want := range tests {
		if got := WithOIDCCallbackPath(in); got != want {
			t.Errorf("WithOIDCCallbackPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseConfigYAMLOlderAuth(t *testing.T) {
	// An older config file always allowed the password, and had one Redirect URL
	cfg, err := ParseConfigYAML([]byte(`
Auth:
  Enabled: true
  Password: hash
  OIDC:
    Enabled: true
    RedirectURL: https://aura.example.com/api/auth/oidc/callback
`))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Auth.PasswordEnabled || !slices.Equal(cfg.Auth.OIDC.RedirectURLs, []string{"https://aura.example.com/api/auth/oidc/callback"}) {
		t.Errorf("older auth = %+v", cfg.Auth)
	}

	// A newer config file keeps its settings
	cfg, err = ParseConfigYAML([]byte(`
Auth:
  Enabled: true
  PasswordEnabled: false
  OIDC:
    Enabled: true
    RedirectURLs:
      - https://aura.example.com/api/auth/oidc/callback
      - http://192.168.1.10:3000/api/auth/oidc/callback
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.PasswordEnabled || len(cfg.Auth.OIDC.RedirectURLs) != 2 {
		t.Errorf("newer auth = %+v", cfg.Auth)
	}
}
