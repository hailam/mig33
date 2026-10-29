// Package controllers implements HTTP handlers over application service ports.
// Request decoding and response writing stay here; persistence is never exposed.
package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/hailam/mig33/internal/adapters/httpapi"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/ports"
	"github.com/hailam/mig33/internal/session"
)

type Controller struct {
	sessions  Sessions
	chat      Chat
	contacts  Contacts
	pages     Pages
	media     Media
	language  Language
	operator  Operator
	native    ports.NativeService
	catalog   Catalog
	publicURL string
}

func New(d Dependencies) *Controller {
	return &Controller{
		sessions:  d.Sessions,
		chat:      d.Chat,
		contacts:  d.Contacts,
		pages:     d.Pages,
		media:     d.Media,
		language:  d.Language,
		operator:  d.Operator,
		native:    d.Native,
		catalog:   d.Catalog,
		publicURL: d.PublicURL,
	}
}

var _ httpapi.ServerInterface = (*Controller)(nil)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writePage(w http.ResponseWriter, result pages.Result) {
	w.Header().Set("Content-Type", result.ContentType)
	w.WriteHeader(result.Status)
	_, _ = w.Write(result.Body)
}

func writeError(w http.ResponseWriter, err error) {
	status, message := http.StatusServiceUnavailable, "Service unavailable"
	var failure *faults.ServiceError
	if errors.As(err, &failure) {
		message = failure.Message
		switch failure.Kind {
		case faults.Invalid:
			status = 400
		case faults.Unauthenticated:
			status = 401
		case faults.Forbidden:
			status = 403
		case faults.NotFound:
			status = 404
		case faults.Conflict:
			status = 409
		case faults.Limited:
			status = 429
		case faults.Internal:
			status = 500
		}
	}
	http.Error(w, message, status)
}

func token() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func validToken(id string) bool { return session.ValidToken(id) }
func wapSessionID(r *http.Request) string {
	if id := r.Header.Get("sid"); id != "" {
		return id
	}
	if c, err := r.Cookie("mig33_wap"); err == nil && validToken(c.Value) {
		return c.Value
	}
	return ""
}
