package realtime

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

func operatorRequest(s *Application, method, path, sid string, f url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var body string
	if f != nil {
		body = f.Encode()
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	r.Header.Set("sid", sid)
	if method == "POST" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
