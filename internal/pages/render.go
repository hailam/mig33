package pages

import (
	"bytes"
	"context"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/a-h/templ"
)

// HiddenField is a named fixed form value. Free text belongs in the last
// control because the preserved native form serializer only escapes spaces.
type HiddenField struct{ Name, Value string }

//go:generate go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate -path .

// legacyURL admits the applet's native command scheme, which a web-only URL
// sanitizer would reject. Template escaping remains enabled for all values.
func legacyURL(value string) templ.SafeURL {
	if strings.ContainsAny(value, "\"'<>`\r\n\t") {
		return templ.SafeURL("about:invalid#invalid-native-url")
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil {
		return templ.SafeURL("about:invalid#invalid-native-url")
	}
	switch u.Scheme {
	case "", "http", "https", "mig33", "local":
		return templ.SafeURL(value)
	}
	return templ.SafeURL("about:invalid#invalid-native-url")
}

var (
	legacyURLAttribute = regexp.MustCompile(`\b(href|action|src)="([^"]*)"`)
	legacyVoidElement  = regexp.MustCompile(`(<(?:input|img|br)\b[^>]*?)/>`)
)

// RenderFragment adapts standards-compliant templ output at one boundary.
// The original parser does not decode entities in URL attributes. Only their
// ampersand escapes are restored, never text nodes or quote delimiters.
func RenderFragment(component templ.Component) ([]byte, error) {
	var output bytes.Buffer
	if err := component.Render(context.Background(), &output); err != nil {
		return nil, err
	}
	rendered := legacyURLAttribute.ReplaceAllFunc(output.Bytes(), func(attribute []byte) []byte { return bytes.ReplaceAll(attribute, []byte("&amp;"), []byte("&")) })
	rendered = legacyVoidElement.ReplaceAll(rendered, []byte("$1>"))
	return rendered, nil
}

func Render(status int, title string, body templ.Component) Result {
	content, err := RenderFragment(Document(title, body))
	if err != nil {
		log.Printf("render native page %q: %v", title, err)
		return Result{Status: 500, ContentType: "text/plain; charset=utf-8", Body: []byte("The page could not be rendered.")}
	}
	return Result{Status: status, ContentType: "text/html; charset=utf-8", Body: content}
}
