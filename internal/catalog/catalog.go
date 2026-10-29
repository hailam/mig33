// Package catalog contains the bounded local product catalog. Historical names
// have source references; IDs, prices, aliases and placeholder artwork are local.
package catalog

import (
	_ "embed"
	"encoding/json"
)

type Product struct {
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

//go:embed products.json
var productData []byte

var products = func() []Product {
	var result []Product
	if err := json.Unmarshal(productData, &result); err != nil {
		panic(err)
	}
	return result
}()

func Products() []Product {
	result := append([]Product(nil), products...)
	for i := range result {
		result[i].SourceURLs = append([]string(nil), result[i].SourceURLs...)
	}
	return result
}

func Lookup(id string) (Product, bool) {
	for _, product := range products {
		if product.ID == id {
			product.SourceURLs = append([]string(nil), product.SourceURLs...)
			return product, true
		}
	}
	return Product{}, false
}
