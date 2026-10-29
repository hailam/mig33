// Package formatting parses an explicitly bounded authoring subset of the
// historical page renderer. It does not interpret user HTML as trusted output.
package formatting

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxSource = 4096
	MaxNodes  = 128
	MaxDepth  = 8
)

type Document struct {
	Nodes           []Node `json:"nodes"`
	Background      string `json:"background,omitempty"`
	BackgroundImage string `json:"background_image,omitempty"`
}

type Node struct {
	Tag        string `json:"tag,omitempty"`
	Text       string `json:"text,omitempty"`
	Color      string `json:"color,omitempty"`
	Background string `json:"background,omitempty"`
	Href       string `json:"href,omitempty"`
	Src        string `json:"src,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Children   []Node `json:"children,omitempty"`
}

type parser struct {
	source     string
	pos, count int
	document   Document
	body       bool
}

func Parse(source string) (Document, error) {
	if len(source) > MaxSource || !utf8.ValidString(source) || invalidText(source) {
		return Document{}, fmt.Errorf("Formatting must contain at most %d UTF-8 bytes without control characters", MaxSource)
	}
	p := parser{source: source}
	nodes, err := p.nodes("", 0)
	if err != nil {
		return Document{}, err
	}
	p.document.Nodes = nodes
	return p.document, nil
}

func invalidText(s string) bool {
	for _, r := range s {
		if (unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r') || r == '\u2028' || r == '\u2029' || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return true
		}
	}
	return false
}

func (p *parser) addNode() error {
	p.count++
	if p.count > MaxNodes {
		return fmt.Errorf("Formatting has too many elements")
	}
	return nil
}

func (p *parser) nodes(end string, depth int) ([]Node, error) {
	if depth > MaxDepth {
		return nil, fmt.Errorf("Formatting is nested too deeply")
	}
	var nodes []Node
	for p.pos < len(p.source) {
		if p.source[p.pos] != '<' {
			n := strings.IndexByte(p.source[p.pos:], '<')
			if n < 0 {
				n = len(p.source) - p.pos
			}
			value := html.UnescapeString(p.source[p.pos : p.pos+n])
			p.pos += n
			if invalidText(value) {
				return nil, fmt.Errorf("Text contains control characters")
			}
			if value != "" {
				if err := p.addNode(); err != nil {
					return nil, err
				}
				nodes = append(nodes, Node{Text: value})
			}
			continue
		}
		tag, attrs, closing, void, err := p.tag()
		if err != nil {
			return nil, err
		}
		if closing {
			if tag != end {
				return nil, fmt.Errorf("Unexpected closing tag %s", tag)
			}
			return nodes, nil
		}
		if tag == "body" {
			if depth != 0 || p.body {
				return nil, fmt.Errorf("Use one outer body element")
			}
			p.body = true
			for key, value := range attrs {
				switch key {
				case "bgcolor":
					p.document.Background, err = Color(value)
				case "background":
					if !PhotoReference(value) {
						err = fmt.Errorf("Background must reference one of your photos as photo:ID")
					} else {
						p.document.BackgroundImage = value
					}
				default:
					err = fmt.Errorf("Unsupported body attribute %s", key)
				}
				if err != nil {
					return nil, err
				}
			}
			if void {
				return nil, fmt.Errorf("Close body with </body>")
			}
			children, e := p.nodes("body", depth+1)
			if e != nil {
				return nil, e
			}
			nodes = append(nodes, children...)
			continue
		}
		node, err := newNode(tag, attrs)
		if err != nil {
			return nil, err
		}
		if err = p.addNode(); err != nil {
			return nil, err
		}
		if tag != "br" && tag != "img" {
			if void {
				return nil, fmt.Errorf("%s needs a closing tag", tag)
			}
			node.Children, err = p.nodes(tag, depth+1)
			if err != nil {
				return nil, err
			}
		}
		nodes = append(nodes, node)
	}
	if end != "" {
		return nil, fmt.Errorf("Missing closing tag %s", end)
	}
	return nodes, nil
}

func (p *parser) tag() (name string, attrs map[string]string, closing, void bool, err error) {
	start := p.pos + 1
	quote := byte(0)
	stop := start
	for ; stop < len(p.source); stop++ {
		c := p.source[stop]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '>' {
			break
		}
		if c == '<' {
			err = fmt.Errorf("Invalid tag")
			return
		}
	}
	if stop == len(p.source) {
		err = fmt.Errorf("Unclosed tag")
		return
	}
	p.pos = stop + 1
	value := strings.TrimSpace(p.source[start:stop])
	if strings.HasPrefix(value, "/") {
		closing = true
		value = strings.TrimSpace(value[1:])
	}
	if strings.HasSuffix(value, "/") {
		void = true
		value = strings.TrimSpace(value[:len(value)-1])
	}
	i := 0
	for i < len(value) && value[i] >= 'a' && value[i] <= 'z' {
		i++
	}
	name = value[:i]
	if name == "" {
		err = fmt.Errorf("Use lowercase supported HTML tags")
		return
	}
	attrs = map[string]string{}
	rest := value[i:]
	if closing {
		if strings.TrimSpace(rest) != "" || void {
			err = fmt.Errorf("Invalid closing tag")
		}
		return
	}
	for len(rest) > 0 {
		if rest[0] != ' ' && rest[0] != '\t' && rest[0] != '\r' && rest[0] != '\n' {
			err = fmt.Errorf("Invalid attributes")
			return
		}
		rest = strings.TrimLeft(rest, " \t\r\n")
		if rest == "" {
			break
		}
		j := 0
		for j < len(rest) && ((rest[j] >= 'a' && rest[j] <= 'z') || rest[j] == '-') {
			j++
		}
		key := rest[:j]
		rest = strings.TrimLeft(rest[j:], " \t\r\n")
		if key == "" || len(rest) == 0 || rest[0] != '=' {
			err = fmt.Errorf("Attributes require quoted values")
			return
		}
		rest = strings.TrimLeft(rest[1:], " \t\r\n")
		if len(rest) < 2 || (rest[0] != '"' && rest[0] != '\'') {
			err = fmt.Errorf("Attributes require quoted values")
			return
		}
		q := rest[0]
		j = strings.IndexByte(rest[1:], q)
		if j < 0 {
			err = fmt.Errorf("Unclosed attribute")
			return
		}
		v := html.UnescapeString(rest[1 : 1+j])
		rest = rest[2+j:]
		if _, exists := attrs[key]; exists {
			err = fmt.Errorf("Duplicate attribute %s", key)
			return
		}
		if invalidText(v) || strings.ContainsAny(v, "<>\"'`\r\n\t") {
			err = fmt.Errorf("Invalid attribute value")
			return
		}
		attrs[key] = v
	}
	return
}

func newNode(tag string, attrs map[string]string) (Node, error) {
	n := Node{Tag: tag}
	allowed := map[string]bool{}
	switch tag {
	case "p", "span", "div":
		allowed["style"] = true
	case "b", "strong", "i", "em", "u", "big", "small", "center", "br":
	case "font":
		n.Tag = "span"
		allowed["color"] = true
	case "a":
		allowed["href"] = true
	case "img":
		allowed["src"] = true
		allowed["width"] = true
		allowed["height"] = true
	default:
		return n, fmt.Errorf("Unsupported tag %s", tag)
	}
	for key, value := range attrs {
		if !allowed[key] {
			return n, fmt.Errorf("Unsupported %s attribute %s", tag, key)
		}
		switch key {
		case "style":
			for _, part := range strings.Split(value, ";") {
				if strings.TrimSpace(part) == "" {
					continue
				}
				name, v, ok := strings.Cut(part, ":")
				name = strings.TrimSpace(name)
				if !ok || (name != "color" && name != "background-color") {
					return n, fmt.Errorf("Only color and background-color styles are supported")
				}
				c, err := Color(strings.TrimSpace(v))
				if err != nil {
					return n, err
				}
				if name == "color" {
					if n.Color != "" {
						return n, fmt.Errorf("Duplicate color style")
					}
					n.Color = c
				} else {
					if n.Background != "" {
						return n, fmt.Errorf("Duplicate background style")
					}
					n.Background = c
				}
			}
		case "color":
			c, err := Color(value)
			if err != nil {
				return n, err
			}
			n.Color = c
		case "href":
			if !NativeLink(value) {
				return n, fmt.Errorf("Links must use mig33:privateChat(username) or mig33:joinChatroom(room)")
			}
			n.Href = value
		case "src":
			if !PhotoReference(value) {
				return n, fmt.Errorf("Images must reference one of your photos as photo:ID")
			}
			n.Src = value
		case "width", "height":
			v, err := strconv.Atoi(value)
			if err != nil || v < 1 || v > 240 {
				return n, fmt.Errorf("Image dimensions must be 1 to 240")
			}
			if key == "width" {
				n.Width = v
			} else {
				n.Height = v
			}
		}
	}
	if tag == "a" && n.Href == "" {
		return n, fmt.Errorf("Link href required")
	}
	if tag == "img" {
		if n.Src == "" {
			return n, fmt.Errorf("Image src required")
		}
		if n.Width == 0 {
			n.Width = 48
		}
		if n.Height == 0 {
			n.Height = 48
		}
	}
	return n, nil
}

// Color returns explicit canonical RGB, avoiding historical named-blue quirks.
func Color(value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	names := map[string]string{
		"black":   "000000",
		"white":   "ffffff",
		"red":     "ff0000",
		"green":   "00ff00",
		"blue":    "0000ff",
		"yellow":  "ffff00",
		"cyan":    "00ffff",
		"magenta": "ff00ff",
		"gray":    "808080",
		"grey":    "808080",
		"orange":  "ffa500",
		"purple":  "800080",
		"brown":   "993300",
	}
	if c, ok := names[v]; ok {
		v = c
	} else {
		v = strings.TrimPrefix(strings.TrimPrefix(v, "#"), "0x")
	}
	if len(v) != 6 {
		return "", fmt.Errorf("Use a named colour or six-digit RGB colour")
	}
	if _, err := strconv.ParseUint(v, 16, 24); err != nil {
		return "", fmt.Errorf("Invalid RGB colour")
	}
	return "#" + v, nil
}

func PhotoReference(value string) bool {
	if !strings.HasPrefix(value, "photo:") {
		return false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(value, "photo:"), 10, 64)
	return err == nil && id > 0 && strconv.FormatInt(id, 10) == strings.TrimPrefix(value, "photo:")
}

func NativeLink(value string) bool {
	for _, method := range []string{"privateChat", "joinChatroom"} {
		prefix := "mig33:" + method + "("
		if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, ")") {
			continue
		}
		arg := strings.TrimSuffix(strings.TrimPrefix(value, prefix), ")")
		if arg == "" || len(arg) > 64 || strings.TrimSpace(arg) != arg {
			return false
		}
		for _, r := range arg {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-' || (method == "joinChatroom" && r == ' ')) {
				return false
			}
		}
		if method == "privateChat" && (len(arg) < 3 || len(arg) > 32) {
			return false
		}
		return true
	}
	return false
}

func Walk(doc *Document, fn func(*Node) error) error {
	var walk func([]Node) error
	walk = func(nodes []Node) error {
		for i := range nodes {
			if err := fn(&nodes[i]); err != nil {
				return err
			}
			if err := walk(nodes[i].Children); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(doc.Nodes)
}

func PlainText(doc Document) string {
	var out strings.Builder
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Tag == "p" || n.Tag == "div" || n.Tag == "center" || n.Tag == "br" {
				out.WriteByte('\n')
			}
			out.WriteString(n.Text)
			walk(n.Children)
			if n.Tag == "p" || n.Tag == "div" || n.Tag == "center" {
				out.WriteByte('\n')
			}
		}
	}
	walk(doc.Nodes)
	return strings.TrimSpace(out.String())
}

func UniformColor(doc Document) *int32 {
	seen, mixed := false, false
	value := ""
	var walk func([]Node, string)
	walk = func(nodes []Node, inherited string) {
		for _, n := range nodes {
			c := inherited
			if n.Color != "" {
				c = n.Color
			}
			if strings.TrimSpace(n.Text) != "" {
				if seen && value != c {
					mixed = true
				}
				if !seen {
					value = c
					seen = true
				}
			}
			walk(n.Children, c)
		}
	}
	walk(doc.Nodes, "")
	if !seen || mixed || value == "" {
		return nil
	}
	v, err := strconv.ParseInt(strings.TrimPrefix(value, "#"), 16, 32)
	if err != nil {
		return nil
	}
	rgb := int32(v)
	return &rgb
}
