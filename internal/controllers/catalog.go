package controllers

import (
	"net/http"

	"github.com/hailam/mig33/internal/adapters/httpapi"
)

func (c *Controller) GetCatalog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	if r.URL.RawQuery != "" {
		http.Error(w, "Catalog does not accept query parameters", http.StatusBadRequest)
		return
	}

	snapshot, err := c.catalog(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	result := httpapi.Catalog{
		Version:      snapshot.Version,
		Target:       snapshot.Target,
		Mode:         snapshot.Mode,
		CreditUnit:   snapshot.CreditUnit,
		DefaultTheme: snapshot.DefaultTheme,
		Languages:    snapshot.Languages,
		Providers:    snapshot.Providers,
		Availability: snapshot.Availability,
		Products:     make([]httpapi.Product, 0, len(snapshot.Products)),
		Emoticons:    make([]httpapi.Emoticon, 0, len(snapshot.Emoticons)),
		Games:        make([]httpapi.Game, 0, len(snapshot.Games)),
	}
	for _, item := range snapshot.Products {
		result.Products = append(result.Products, httpapi.Product{
			Id:               item.ID,
			Name:             item.Name,
			Kind:             item.Kind,
			Price:            item.Price,
			Description:      item.Description,
			Category:         item.Category,
			CategoryEvidence: item.CategoryEvidence,
			NameEvidence:     item.NameEvidence,
			ArtworkEvidence:  item.ArtworkEvidence,
			PriceEvidence:    item.PriceEvidence,
			SourceUrls:       item.SourceURLs,
		})
	}
	for _, item := range snapshot.Emoticons {
		result.Emoticons = append(result.Emoticons, httpapi.Emoticon{
			Token: item.Token,
			Label: item.Label,
			Pack:  item.Pack,
			Type:  int64(item.Type),
		})
	}
	for _, item := range snapshot.Games {
		result.Games = append(result.Games, httpapi.Game{
			Id:                 item.ID,
			Name:               item.Name,
			Availability:       item.Availability,
			Evidence:           item.Evidence,
			HistoricalCharging: optionalString(item.HistoricalCharging),
			Description:        item.Description,
			SourceUrl:          optionalString(item.SourceURL),
			PlayTarget:         optionalString(item.PlayTarget),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (c *Controller) ServeCatalog(w http.ResponseWriter, r *http.Request) { c.GetCatalog(w, r) }

func (c *Controller) ReadHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, httpapi.Health{Mode: "local-demo", Status: "ok"})
}

func (c *Controller) ServeHealth(w http.ResponseWriter, r *http.Request) { c.ReadHealth(w, r) }
