package catalog

import "strings"

type EmoticonDefinition struct {
	Token, Label, Pack string
	Type               byte
	Cells              []int
}

// Art is taken from the original default_v2 tag129 atlas (20x4 cells,12px).
// Aliases, effects, pack pricing and this grouping are reconstruction choices.
var emoticons = buildLocalEmoticons()

func buildLocalEmoticons() []EmoticonDefinition {
	items := []EmoticonDefinition{
		{"[smile]", "Smile", "", 1, []int{5}},
		{"[heart]", "Heart", "", 1, []int{49}},
		{"[gift]", "Gift", "", 1, []int{27}},
		{"[buzz]", "Buzz (vibrate)", "emotes-effects", 2, []int{49}},
		{"[sparkle]", "Sparkle (animated)", "emotes-effects", 3, []int{5, 25, 5}},
		{"[chime]", "Chime (sound)", "emotes-effects", 4, []int{5}},
		{"[gift:rose]", "Rose gift", "receipt", 3, []int{49, 49, 5}},
		{"[gift:cake]", "Birthday cake gift", "receipt", 3, []int{27, 5, 27}},
		{"[gift:star]", "Thank you star gift", "receipt", 3, []int{5, 25, 5}},
	}
	for _, product := range Products() {
		switch product.Kind {
		case "gift":
			if product.ID == "rose" || product.ID == "cake" || product.ID == "star" {
				continue
			}
			items = append(items, EmoticonDefinition{"[gift:" + product.ID + "]", product.Name + " (placeholder)", "receipt", 3, []int{27, 5, 27}})
		case "emoticon_pack":
			if product.ID != "emotes-effects" {
				items = append(items, EmoticonDefinition{PackPlaceholderToken(product.ID), product.Name + " (placeholder)", product.ID, 1, []int{5}})
			}
		}
	}
	return items
}

func PackPlaceholderToken(id string) string {
	return "[pack:" + strings.TrimPrefix(id, "emotes-") + "]"
}
