package routes_auth

import (
	"aura/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedirectURLForOrigin(t *testing.T) {
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.Auth.OIDC.RedirectURLs = []string{
		"https://aura.example.com/api/auth/oidc/callback",
		"http://192.168.1.10:3000/api/auth/oidc/callback",
	}

	tests := []struct {
		origin string
		want   int
	}{
		{"https://aura.example.com", 0},
		{"http://192.168.1.10:3000", 1},
		{"HTTP://192.168.1.10:3000/", 1},
		// Another port, scheme or host, or no origin, uses the first
		{"http://192.168.1.10:8080", 0},
		{"http://aura.example.com", 0},
		{"https://evil.example.com", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := redirectURLIndexForOrigin(tt.origin); got != tt.want {
			t.Errorf("origin %q = %d, want %d", tt.origin, got, tt.want)
		}
	}

	// The callback uses the Redirect URL the login started with, or the first one
	request := func(cookie string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: oidcRedirectCookieName, Value: cookie})
		}
		return r
	}
	for cookie, want := range map[string]string{
		"1":  "http://192.168.1.10:3000/api/auth/oidc/callback",
		"0":  "https://aura.example.com/api/auth/oidc/callback",
		"7":  "https://aura.example.com/api/auth/oidc/callback",
		"x":  "https://aura.example.com/api/auth/oidc/callback",
		"":   "https://aura.example.com/api/auth/oidc/callback",
		"-1": "https://aura.example.com/api/auth/oidc/callback",
	} {
		if got := redirectURLFromCookie(request(cookie)); got != want {
			t.Errorf("cookie %q = %s, want %s", cookie, got, want)
		}
	}
}
