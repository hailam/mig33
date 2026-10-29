package catalog

// CatalogSnapshot contains public reconstruction metadata, never user balances.
type CatalogSnapshot struct {
	Version                  int64
	Target, Mode, CreditUnit string
	Products                 []CatalogProduct
	DefaultTheme             map[string]string
	Emoticons                []CatalogEmoticon
	Games                    []CatalogGame
	Languages                []map[string]string
	Providers                map[string]string
	Availability             string
}

type CatalogProduct struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`
	Price            int64    `json:"price"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	CategoryEvidence string   `json:"category_evidence"`
	NameEvidence     string   `json:"name_evidence"`
	ArtworkEvidence  string   `json:"artwork_evidence"`
	PriceEvidence    string   `json:"price_evidence"`
	SourceURLs       []string `json:"source_urls"`
}

type CatalogEmoticon struct {
	Token, Label, Pack string
	Type               byte
}

type CatalogGame struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Availability       string `json:"availability"`
	Evidence           string `json:"evidence"`
	HistoricalCharging string `json:"historical_charging,omitempty"`
	Description        string `json:"description"`
	SourceURL          string `json:"source_url,omitempty"`
	PlayTarget         string `json:"play_target,omitempty"`
}
