package formatting

import (
	"fmt"
	"strings"
)

// ParseChat is the local authoring contract for the recovered WAP chat surface.
// The historical endpoint and HTML capability are attested; this bounded set
// of accepted input is a reconstruction, not a recovered server parser.
func ParseChat(source string) (Document, error) {
	doc, err := Parse(source)
	if err != nil {
		return Document{}, err
	}
	if doc.Background != "" || doc.BackgroundImage != "" {
		return Document{}, fmt.Errorf("Chat formatting cannot change the page background")
	}
	err = Walk(&doc, func(n *Node) error {
		if n.Tag == "img" || n.Src != "" {
			return fmt.Errorf("Use the photo-sharing action to send images")
		}
		return nil
	})
	if err != nil {
		return Document{}, err
	}
	if strings.TrimSpace(PlainText(doc)) == "" {
		return Document{}, fmt.Errorf("Enter some message text")
	}
	return doc, nil
}
