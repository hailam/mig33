package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/hailam/mig33/internal/faults"
)

// Ownership supplies the inventory checks needed when sending paid emoticons.
// Downloads remain public so recipients can render a sender's purchased pack.
type Ownership interface {
	HasStoreItem(context.Context, string, string) (bool, error)
}

type Service struct{ ownership Ownership }

func New(ownership Ownership) *Service { return &Service{ownership: ownership} }

func Emoticons() []CatalogEmoticon {
	result := make([]CatalogEmoticon, 0, len(emoticons))
	for _, item := range emoticons {
		result = append(result, CatalogEmoticon{Token: item.Token, Label: item.Label, Pack: item.Pack, Type: item.Type})
	}
	return result
}

func LookupEmoticon(token string) (EmoticonDefinition, bool) {
	for _, item := range emoticons {
		if item.Token != token {
			continue
		}
		item.Cells = append([]int(nil), item.Cells...)
		return item, true
	}
	return EmoticonDefinition{}, false
}

func ReferencedEmoticons(body string) []string {
	var tokens []string
	for _, item := range emoticons {
		if strings.Contains(body, item.Token) {
			tokens = append(tokens, item.Token)
		}
	}
	return tokens
}

func (s *Service) OwnedEmoticons(ctx context.Context, owner string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var tokens []string
	for _, item := range emoticons {
		if item.Pack == "receipt" {
			continue
		}
		if item.Pack != "" {
			owns, err := s.ownership.HasStoreItem(ctx, owner, item.Pack)
			if err != nil {
				return nil, err
			}
			if !owns {
				continue
			}
		}
		tokens = append(tokens, item.Token)
	}
	return tokens, nil
}

func (s *Service) ValidateEmoticons(ctx context.Context, owner, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, item := range emoticons {
		if !strings.Contains(body, item.Token) || item.Pack == "" {
			continue
		}
		if item.Pack == "receipt" {
			return fmt.Errorf("Gift animations are sent by a completed store purchase")
		}
		owns, err := s.ownership.HasStoreItem(ctx, owner, item.Pack)
		if err != nil {
			return faults.WrapError(faults.Unavailable, "Emoticon ownership could not be checked", err)
		}
		if !owns {
			product, _ := Lookup(item.Pack)
			return fmt.Errorf("Buy %s before sending %s", product.Name, item.Token)
		}
	}
	return nil
}

func Snapshot(ctx context.Context, publicURL string) (CatalogSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return CatalogSnapshot{}, err
	}
	result := CatalogSnapshot{
		Version:      1,
		Target:       "4.62.000",
		Mode:         "local_reconstruction",
		CreditUnit:   "hundredths_of_test_credit",
		DefaultTheme: map[string]string{"id": "default", "name": "Original default", "availability": "free", "evidence": "original_bundled_bytes"},
		Languages: []map[string]string{{
			"code":     "en-US",
			"name":     "English",
			"evidence": "original_bundled_bytes",
			"url":      publicURL + "/sites/resources/language/v4.6/en-US",
		}},
		Providers: map[string]string{
			"sms":         "mock_recorded",
			"email":       "mock_external_local_inbox",
			"calls":       "mock_recorded",
			"payments":    "test_credit_ledger",
			"external_im": "disconnected_tbd",
		},
		Availability: "Products are local renditions; historical game references have no play target. No real providers are called.",
		Emoticons:    Emoticons(),
		Games:        Games(),
	}
	for _, item := range Products() {
		result.Products = append(result.Products, CatalogProduct(item))
	}
	return result, nil
}
