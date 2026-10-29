package controllers

import (
	"mime"
	"net/http"
	"net/url"

	"github.com/hailam/mig33/internal/adapters/legacyweb"
)

func (c *Controller) ServeFeaturePages(w http.ResponseWriter, r *http.Request) {
	var form url.Values
	if r.Method == http.MethodPost {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/x-www-form-urlencoded" {
			http.Error(w, "Form content type required", 415)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid or oversized form", 400)
			return
		}
		form = r.PostForm
	}
	result, err := c.pages.FeaturePage(r.Context(), wapSessionID(r), legacyweb.Request{Method: r.Method, URL: r.URL, Form: form})
	if err != nil {
		writeError(w, err)
		return
	}

	writePage(w, result)
}
