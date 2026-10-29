package controllers

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/faults"
)

func (c *Controller) ServeMedia(w http.ResponseWriter, r *http.Request) {
	var data []byte
	if r.URL.Path == "/api/photos" && r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
		var err error
		data, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Photo exceeds 1 MiB", 413)
			return
		}
	}

	result, err := c.media.Media(r.Context(), legacyweb.MediaRequest{
		SessionID:      wapSessionID(r),
		Method:         r.Method,
		URL:            r.URL,
		CSRF:           r.Header.Get("X-CSRF-Token"),
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Data:           data,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	if result.Photo != nil {
		writeJSON(w, 201, result.Photo)
		return
	}
	writePage(w, result.Page)
}

func (c *Controller) ServeLanguageResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Use GET", 405)
		return
	}
	if c.language == nil {
		writeError(w, faults.NewError(faults.Unavailable, "Language resources unavailable"))
		return
	}

	result, err := c.language.LanguageResource(r.Context(), c.publicURL, strings.TrimPrefix(r.URL.Path, "/sites/resources/language/v4.6/"))
	if err != nil {
		var failure *faults.ServiceError
		if errors.As(err, &failure) && failure.Kind == faults.NotFound {
			http.NotFound(w, r)
			return
		}
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Data)
}
