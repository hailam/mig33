package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/session"
)

func (c *Controller) ServeOperator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Use GET or POST", http.StatusMethodNotAllowed)
		return
	}
	f, ok := browserForm(w, r, 8192, c.publicURL)
	if !ok {
		return
	}
	if !c.operator.OperatorsEnabled(r.Context()) {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/operator/login" {
		if r.Method == http.MethodGet {
			csrf := token()
			http.SetCookie(w, browserCookie("mig33_operator_login", csrf, "/operator", r, 600))
			writePage(w, pages.Render(200, "Operator sign in", pages.Stack(pages.Heading("Operator sign in"),
				pages.Form("/operator/login", []pages.HiddenField{{Name: "csrf", Value: csrf}},
					pages.Stack(pages.Input("username", "Username", "text", "", 10),
						pages.Input("password", "Password", "password", "", 10)), "Sign in"))))
			return
		}
		if !loginCSRF(r, "mig33_operator_login", f.Get("csrf")) {
			http.Error(w, "Reopen the sign-in page", http.StatusForbidden)
			return
		}
		request := session.OperatorLogin{Username: f.Get("username"), Password: f.Get("password")}
		if old, err := r.Cookie("mig33_operator"); err == nil {
			request.PreviousSessionID = old.Value
		}
		x, err := c.operator.LoginOperator(r.Context(), request)
		if err != nil {
			writeError(w, err)
			return
		}
		http.SetCookie(w, browserCookie("mig33_operator", x.ID, "/operator", r, int(x.TTL.Seconds())))
		http.SetCookie(w, browserCookie("mig33_operator_login", "", "/operator", r, -1))
		http.Redirect(w, r, "/operator", http.StatusSeeOther)
		return
	}
	id := r.Header.Get("sid")
	if id == "" {
		if cookie, err := r.Cookie("mig33_operator"); err == nil {
			id = cookie.Value
		}
	}
	auth := session.OperatorSessionRequest{SessionID: id, CSRF: f.Get("csrf"), Mutation: r.Method == http.MethodPost}
	if _, err := c.operator.OperatorSession(r.Context(), auth); err != nil {
		if unauthenticated(err) && r.Method == http.MethodGet {
			http.Redirect(w, r, "/operator/login", http.StatusSeeOther)
		} else {
			writeError(w, err)
		}
		return
	}
	switch r.URL.Path {
	case "/operator/logout":
		if r.Method != http.MethodPost {
			http.Error(w, "Use POST", http.StatusMethodNotAllowed)
			return
		}
		if err := c.operator.LogoutOperator(r.Context(), auth); err != nil {
			writeError(w, err)
			return
		}
		http.SetCookie(w, browserCookie("mig33_operator", "", "/operator", r, -1))
		http.Redirect(w, r, "/operator/login", http.StatusSeeOther)
	case "/operator/review":
		if r.Method != http.MethodPost {
			http.Error(w, "Use POST", http.StatusMethodNotAllowed)
			return
		}
		itemID, err := strconv.ParseInt(f.Get("id"), 10, 64)
		if err != nil {
			writeOperatorError(w, "Review", faults.ErrInvalid)
			return
		}
		request := session.OperatorReviewRequest{
			OperatorSessionRequest: auth,
			Kind:                   f.Get("kind"),
			ItemID:                 itemID,
			Status:                 f.Get("status"),
			Note:                   f.Get("note"),
			Nonce:                  f.Get("nonce"),
			Expected:               f.Get("expected"),
		}
		if err := c.operator.ReviewOperatorItem(r.Context(), request); err != nil {
			writeOperatorError(w, "Review", err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/operator?kind=%s&id=%d", request.Kind, request.ItemID), http.StatusSeeOther)
	case "/operator/categories":
		if r.Method != http.MethodPost {
			http.Error(w, "Use POST", http.StatusMethodNotAllowed)
			return
		}
		category, err := strconv.ParseInt(f.Get("id"), 10, 16)
		if err != nil {
			writeOperatorError(w, "Categories", faults.ErrInvalid)
			return
		}
		request := session.OperatorCategoryRequest{
			OperatorSessionRequest: auth,
			ID:                     int16(category),
			Action:                 f.Get("op"),
			Name:                   f.Get("name"),
			Room:                   f.Get("room"),
		}
		if err := c.operator.ChangeOperatorCategory(r.Context(), request); err != nil {
			writeOperatorError(w, "Categories", err)
			return
		}
		http.Redirect(w, r, "/operator", http.StatusSeeOther)
	case "/operator", "/operator/":
		if r.Method != http.MethodGet {
			http.Error(w, "Use GET", http.StatusMethodNotAllowed)
			return
		}
		query := r.URL.Query()
		result, err := c.pages.OperatorDashboard(r.Context(), session.OperatorDashboardRequest{
			SessionID:     id,
			Kind:          query.Get("kind"),
			ItemID:        query.Get("id"),
			ReportBefore:  query.Get("report_before"),
			SupportBefore: query.Get("support_before"),
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writePage(w, result)
	default:
		http.NotFound(w, r)
	}
}

func writeOperatorError(w http.ResponseWriter, title string, err error) {
	var applicationError *faults.ServiceError
	if errors.As(err, &applicationError) {
		writeError(w, err)
		return
	}
	status, message := 503, "This service is temporarily unavailable. Try again."
	switch {
	case errors.Is(err, faults.ErrForbidden):
		status, message = 403, "This action is not allowed. Reopen the page and check your account."
	case errors.Is(err, faults.ErrNotFound):
		status, message = 404, "The requested item was not found."
	case errors.Is(err, faults.ErrInvalid):
		status, message = 400, "Check the submitted fields."
	case errors.Is(err, faults.ErrConflict):
		status, message = 409, "This request was already used with different details. Reopen the page."
	case errors.Is(err, faults.ErrLimit):
		status, message = 429, "The storage limit has been reached."
	}
	writePage(w, pages.Render(status, title, pages.Stack(pages.Heading(title), pages.Paragraph(message))))
}
