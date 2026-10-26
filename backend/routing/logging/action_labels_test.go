package routes_logging

import "testing"

func TestActionKeyForRoute(t *testing.T) {
	for _, tc := range []struct{ method, path, want string }{
		{"GET", "/api/db", "GET:/api/db"},
		{"GET", "/api/db/", "GET:/api/db"},
		{"get", "/api/statistics", "GET:/api/statistics"},
		// Routes with {params} match by pattern
		{"PATCH", "/api/config/mediux", "PATCH:/api/config/{section}"},
		{"GET", "/api/plugin/silo/download/abc123", "GET:/api/plugin/silo/download/{token}"},
		// Not a known route
		{"PATCH", "/api/config/mediux/extra", "PATCH:/api/config/mediux/extra"},
		{"POST", "/api/config/mediux", "POST:/api/config/mediux"},
	} {
		if got := actionKeyForRoute(tc.method, tc.path); got != tc.want {
			t.Errorf("actionKeyForRoute(%s, %s) = %s, want %s", tc.method, tc.path, got, tc.want)
		}
	}
}
