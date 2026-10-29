// Package httptransport composes generated JSON routes and legacy HTTP adapters.
package httptransport

import (
	"net/http"
	"strings"

	"github.com/hailam/mig33/internal/adapters/httpapi"
)

// Controller combines generated operations with historical HTTP handlers.
// The concrete controller implements these using application service ports.
type Controller interface {
	httpapi.ServerInterface
	ServeCatalog(http.ResponseWriter, *http.Request)
	ServeContactsAPI(http.ResponseWriter, *http.Request)
	ServeChatAPI(http.ResponseWriter, *http.Request)
	ServeHealth(http.ResponseWriter, *http.Request)
	ServeWAP(http.ResponseWriter, *http.Request)
	ServeOperator(http.ResponseWriter, *http.Request)
	ServeMedia(http.ResponseWriter, *http.Request)
	ServeLanguageResource(http.ResponseWriter, *http.Request)
	ServeFeaturePages(http.ResponseWriter, *http.Request)
	ServeNativeGateway(http.ResponseWriter, *http.Request)
}

func New(app Controller) http.Handler {
	mux := http.NewServeMux()
	// Exact legacy URLs and prefixes are registered in one place. Narrower
	// generated method/path routes take precedence over these fallback handlers.
	for _, route := range []struct {
		pattern string
		handler http.HandlerFunc
	}{
		{"/api/catalog", app.ServeCatalog},
		{"/api/contacts", app.ServeContactsAPI},
		{"/api/chat/", app.ServeChatAPI},
		{"/api/photos", app.ServeMedia},
		{"/assets/photos/", app.ServeMedia},
		{"/member2/t.php", app.ServeWAP},
		{"/operator", app.ServeOperator},
		{"/operator/", app.ServeOperator},
		{"/sites/resources/language/v4.6/", app.ServeLanguageResource},
		{"/sites/", app.ServeFeaturePages},
		{"/healthz", app.ServeHealth},
		{"/", app.ServeNativeGateway},
	} {
		mux.HandleFunc(route.pattern, route.handler)
	}

	httpapi.HandlerWithOptions(app, httpapi.StdHTTPServerOptions{
		BaseRouter: mux,
		// Preserve the original authentication/error ordering when generated
		// parameter binding fails. The feature handler validates the original
		// request and never receives a partially bound generated request object.
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) { app.ServeChatAPI(w, r) },
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// Go's GET patterns also accept HEAD; the original endpoints require GET.
		// Send HEAD through existing validation rather than silently changing it.
		if r.Method == http.MethodHead {
			switch {
			case r.URL.Path == "/healthz":
				app.ServeHealth(w, r)
				return
			case r.URL.Path == "/api/catalog":
				app.ServeCatalog(w, r)
				return
			case r.URL.Path == "/api/contacts":
				app.ServeContactsAPI(w, r)
				return
			case strings.HasPrefix(r.URL.Path, "/api/chat/"):
				app.ServeChatAPI(w, r)
				return
			}
		}

		mux.ServeHTTP(w, r)
	})
}
