// Package pages renders mig33-compatible embedded pages with typed templ components.
// Authentication and HTTP transport belong to the caller.
package pages

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// User is the profile information supplied by the server's authenticated state.
type User struct {
	Username    string
	DisplayName string
	Status      string
	Online      bool
}

// Result can be delivered by HTTP or the legacy 922/923 page transport.
type Result struct {
	Status      int
	ContentType string
	Body        []byte
}

//go:embed routes.json
var catalogJSON []byte

type route struct {
	ID         string `json:"id"`
	Template   string `json:"template"`
	Controller string `json:"controller"`
	Action     string `json:"action"`
	View       string `json:"view"`
	Purpose    string `json:"purpose"`
}

type routeKey struct {
	controller string
	action     string
	view       string
}

var knownRoutes = loadRoutes()

func loadRoutes() map[routeKey]route {
	var catalog struct {
		Routes []route `json:"routes"`
	}
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		panic("pages: invalid embedded route catalog: " + err.Error())
	}
	routes := make(map[routeKey]route, len(catalog.Routes))
	for _, entry := range catalog.Routes {
		routes[routeKey{entry.Controller, entry.Action, entry.View}] = entry
	}
	return routes
}

// Handle preserves the historical /sites/index.php?c=...&a=...&v=midlet routes.
// This stateless dispatcher implements profile/home and help/main. The server
// handles authenticated room, report and block-list state. Other catalogued
// features return 501, so an unavailable feature never appears successful.
func Handle(method string, u *url.URL, viewer User, users []User) Result {
	if u == nil || u.Path != "/sites/index.php" {
		return notFound()
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return notFound()
	}
	// Duplicate routing selectors are ambiguous; ordinary feature parameters
	// (username, roomName, cid, etc.) do not change the selected route.
	for _, name := range []string{"c", "a", "v"} {
		if len(query[name]) != 1 {
			return notFound()
		}
	}
	key := routeKey{query.Get("c"), query.Get("a"), query.Get("v")}
	entry, known := knownRoutes[key]
	if !known {
		return notFound()
	}
	if method != http.MethodGet {
		return Render(http.StatusMethodNotAllowed, "Method Not Allowed",
			Paragraph("This page supports GET requests only."))
	}
	switch key {
	case routeKey{"profile", "home", "midlet"}:
		return profile(query.Get("username"), viewer, users)
	case routeKey{"help", "main", "midlet"}:
		return Render(http.StatusOK, "Help", legacyHelpView())
	default:
		return Render(http.StatusNotImplemented, "Feature Unavailable", Stack(
			Paragraph(entry.Purpose),
			Paragraph("This historical mig33 feature is not implemented in this reconstruction.")))
	}
}

func profile(username string, viewer User, users []User) Result {
	var target User
	found := false
	if username == "" || strings.EqualFold(username, viewer.Username) {
		target, found = viewer, true
	} else {
		for _, candidate := range users {
			if strings.EqualFold(username, candidate.Username) {
				target, found = candidate, true
				break
			}
		}
	}
	if !found {
		return Render(http.StatusNotFound, "User Not Found",
			Paragraph("The requested user was not found."))
	}
	name := target.DisplayName
	if name == "" {
		name = target.Username
	}
	presence := "Offline"
	if target.Online {
		presence = "Online"
	}
	status := target.Status
	if status == "" {
		status = "No status message."
	}
	return Render(http.StatusOK, "User Profile", legacyProfileView(profileView{
		Name:        name,
		Username:    target.Username,
		Presence:    presence,
		Status:      status,
		ShowActions: !strings.EqualFold(target.Username, viewer.Username),
	}))
}

func notFound() Result {
	return Render(http.StatusNotFound, "Page Not Found",
		Paragraph("The requested mig33 page was not found."))
}

type profileView struct {
	Name, Username, Presence, Status string
	ShowActions                      bool
}
