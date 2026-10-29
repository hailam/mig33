package controllers

import (
	"crypto/subtle"
	"errors"
	"mime"
	"net/http"
	"net/url"

	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/session"
)

const wapChatPath = "/member2/t.php"

func browserCookie(name, value, path string, r *http.Request, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	}
}

func browserForm(w http.ResponseWriter, r *http.Request, limit int64, origin string) (url.Values, bool) {
	if r.Method != http.MethodPost {
		return nil, true
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/x-www-form-urlencoded" {
		http.Error(w, "Form content type required", http.StatusUnsupportedMediaType)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return nil, false
	}
	for _, values := range r.PostForm {
		if len(values) != 1 {
			http.Error(w, "Duplicate form field", http.StatusBadRequest)
			return nil, false
		}
	}
	if supplied := r.Header.Get("Origin"); supplied != "" && supplied != origin {
		http.Error(w, "Origin is not allowed", http.StatusForbidden)
		return nil, false
	}
	return r.PostForm, true
}

func browserFormFields(form url.Values, fields ...string) bool {
	if len(form) != len(fields) {
		return false
	}
	for _, field := range fields {
		if len(form[field]) != 1 {
			return false
		}
	}
	return true
}

func loginCSRF(r *http.Request, cookieName, csrf string) bool {
	cookie, err := r.Cookie(cookieName)
	return err == nil && validToken(csrf) && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(csrf)) == 1
}

func unauthenticated(err error) bool {
	var failure *faults.ServiceError
	return errors.As(err, &failure) && failure.Kind == faults.Unauthenticated
}

func (c *Controller) ServeWAP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Use GET or POST", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	f, ok := browserForm(w, r, 32*1024, c.publicURL)
	if !ok {
		return
	}
	x, err := c.sessions.Session(r.Context(), wapSessionID(r))
	if err != nil {
		if !unauthenticated(err) {
			writeError(w, err)
			return
		}
		if r.Method == http.MethodGet {
			csrf := token()
			http.SetCookie(w, browserCookie("mig33_wap_login", csrf, "/", r, 600))
			writePage(w, pages.Render(200, "Sign in", pages.WAPLogin(c.publicURL+wapChatPath, csrf)))
			return
		}
		if !browserFormFields(f, "csrf", "op", "username", "password") || f.Get("op") != "login" || !loginCSRF(r, "mig33_wap_login", f.Get("csrf")) {
			http.Error(w, "Reopen the sign-in page", http.StatusForbidden)
			return
		}
		x, err = c.sessions.LoginBrowser(r.Context(), session.BrowserLogin{Username: f.Get("username"), Password: f.Get("password")})
		if err != nil {
			writeError(w, err)
			return
		}
		http.SetCookie(w, browserCookie("mig33_wap", x.ID, "/", r, int(x.TTL.Seconds())))
		http.SetCookie(w, browserCookie("mig33_wap_login", "", "/", r, -1))
		http.Redirect(w, r, wapChatPath, http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost && f.Get("op") == "logout" {
		if !browserFormFields(f, "csrf", "op") {
			http.Error(w, "Invalid sign-out request", http.StatusForbidden)
			return
		}
		if err := c.sessions.LogoutBrowser(r.Context(), session.BrowserLogout{SessionID: x.ID, CSRF: f.Get("csrf")}); err != nil {
			writeError(w, err)
			return
		}
		http.SetCookie(w, browserCookie("mig33_wap", "", "/", r, -1))
		http.Redirect(w, r, wapChatPath, http.StatusSeeOther)
		return
	}
	result, err := c.pages.BrowserPage(r.Context(), x.ID, legacyweb.Request{Method: r.Method, URL: r.URL, Form: f})
	if err != nil {
		writeError(w, err)
		return
	}

	writePage(w, result)
}
