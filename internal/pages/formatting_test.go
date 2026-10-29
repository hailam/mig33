package pages

import (
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/formatting"
)

func TestFormattedTemplEscapesTextAndPreservesNativeStyles(t *testing.T) {
	doc, err := formatting.Parse(`<body bgcolor="#fff0cc"><p style="color:red"><b>&lt;script&gt; &amp; text</b></p><a href="mig33:privateChat(bob)">Chat</a></body>`)
	if err != nil {
		t.Fatal(err)
	}
	page := RenderFormatted(200, "Profile", doc, Stack(FormattedContent(doc), Paragraph("Username: alice")))
	html := string(page.Body)
	for _, want := range []string{
		`bgcolor="#fff0cc"`,
		`style="color:#ff0000"`,
		`<b>&lt;script&gt; &amp; text</b>`,
		`href="mig33:privateChat(bob)"`,
		`Username: alice`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %s in %s", want, html)
		}
	}
	if strings.Contains(html, "<script>") {
		t.Fatal("decoded text became markup")
	}
}

func TestFormattingEditorKeepsSourceOutsideNativeTextarea(t *testing.T) {
	source := `<p>literal </textarea> &quot; &amp; values</p>`
	r := Render(200, "Editor", ProfileFormattingEditor("http://127.0.0.1/sites/index.php?c=profile&a=edit&v=midlet&field=customization", "abc", source, "", nil))
	body := string(r.Body)
	if strings.Count(body, "</textarea>") != 1 || !strings.Contains(body, `size="4096"></textarea>`) {
		t.Fatalf("unsafe prefill %s", body)
	}
	if !strings.Contains(body, `&lt;/textarea&gt;`) {
		t.Fatal("current source not escaped")
	}
}

func TestFormattedEmMatchesOriginalBoldStyle(t *testing.T) {
	doc, err := formatting.Parse(`<p><em>Emphasis</em></p>`)
	if err != nil {
		t.Fatal(err)
	}
	r := Render(200, "Emphasis", FormattedContent(doc))
	if !strings.Contains(string(r.Body), `<b>Emphasis</b>`) || strings.Contains(string(r.Body), `<em>`) {
		t.Fatalf("em must retain native bold behavior in both renderers: %s", r.Body)
	}
}
