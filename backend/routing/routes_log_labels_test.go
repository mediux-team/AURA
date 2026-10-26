package routing

import (
	"aura/config"
	routes_logging "aura/routing/logging"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// Every route needs a label for the Logs page's actions, in routing/logging/get.go
func TestEveryRouteHasALogsPageLabel(t *testing.T) {
	setTestConfig(t, config.Config{}, true)
	for name, addRoutes := range map[string]func(*chi.Mux){"full": AddRoutes, "setup": addOnboardingRoutes} {
		r := chi.NewRouter()
		addRoutes(r)
		_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			// Only /api routes are logged
			if strings.HasPrefix(route, "/api") && !routes_logging.HasActionLabel(method, route) {
				t.Errorf("%s route %s %s has no label in possible_actions_paths", name, method, route)
			}
			return nil
		})
	}
}
