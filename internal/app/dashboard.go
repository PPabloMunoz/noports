package app

import (
	"html/template"
	"net/http"
	"sort"

	"github.com/ppablomunoz/noports/internal/registry"
)

// dashboardData is the template input for web/index.html.tmpl.
type dashboardData struct {
	Routes []registry.Route
}

// newDashboardHandler returns a handler that renders tmpl with active routes on every request. A nil tmpl yields a nil handler so the proxy 404s localhost.
func newDashboardHandler(tmpl *template.Template, store *registry.Store) http.Handler {
	if tmpl == nil {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		all := store.List()
		routes := make([]registry.Route, 0, len(all))
		for _, route := range all {
			routes = append(routes, route)
		}
		sort.Slice(routes, func(i, j int) bool { return routes[i].Hostname < routes[j].Hostname })
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, dashboardData{Routes: routes}); err != nil {
			http.Error(w, "failed to render dashboard", http.StatusInternalServerError)
		}
	})
}
