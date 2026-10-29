package pages

import (
	"log"
	"strings"

	"github.com/a-h/templ"

	"github.com/hailam/mig33/internal/formatting"
)

type ProfileFormattingPhoto struct{ Reference, Caption string }

// Native paragraph colour parsing rejects a trailing semicolon. Attribute maps
// preserve exact CSS, constructed only from canonical RGB and fixed properties.
func formattedColorStyle(property, color string) string {
	if property != "color" && property != "background-color" {
		return ""
	}
	canonical, err := formatting.Color(color)
	if err != nil {
		return ""
	}
	return property + ":" + canonical
}

func formattedColorAttributes(property, color string) templ.Attributes {
	if style := formattedColorStyle(property, color); style != "" {
		return templ.Attributes{"style": style}
	}
	return nil
}

func formattedStyle(node formatting.Node) string {
	parts := []string{}
	if node.Color != "" {
		parts = append(parts, formattedColorStyle("color", node.Color))
	}
	if node.Background != "" {
		parts = append(parts, formattedColorStyle("background-color", node.Background))
	}
	return strings.Join(parts, ";")
}

func formattedAttributes(node formatting.Node) templ.Attributes {
	if style := formattedStyle(node); style != "" {
		return templ.Attributes{"style": style}
	}
	return nil
}

// RenderFormatted receives parsed, authorized metadata, never raw user HTML.
func RenderFormatted(status int, title string, document formatting.Document, body templ.Component) Result {
	content, err := RenderFragment(FormattedDocument(title, document, body))
	if err != nil {
		log.Printf("render formatted page %q: %v", title, err)
		return Result{Status: 500, ContentType: "text/plain; charset=utf-8", Body: []byte("The page could not be rendered.")}
	}
	return Result{Status: status, ContentType: "text/html; charset=utf-8", Body: content}
}
