package pages

import (
	"strings"
	"testing"
)

func TestTemplLegacyURLBoundaryDoesNotUnescapeUserText(t *testing.T) {
	userText := `hello & <script> " href="bad&amp;value"`
	result := Render(200, "Title", Stack(Paragraph(userText), Link("http://127.0.0.1:8080/sites/index.php?c=profile&a=home&v=midlet", "Profile"), Link("mig33:joinChatroom(Test Room)", "Join")))
	body := string(result.Body)
	if !strings.Contains(body, `href="http://127.0.0.1:8080/sites/index.php?c=profile&a=home&v=midlet"`) {
		t.Fatal("native query delimiters were escaped")
	}
	if !strings.Contains(body, `href="mig33:joinChatroom(Test Room)"`) {
		t.Fatal("native action was filtered or URLencoded")
	}
	if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "bad&amp;amp;value") {
		t.Fatalf("user text was unescaped: %s", body)
	}
}

func TestTemplFormsKeepHiddenValuesBeforeFinalTextControl(t *testing.T) {
	result := Render(200, "Form", Form("http://127.0.0.1:8080/sites/index.php?c=x&a=y&v=midlet", []HiddenField{{Name: "csrf", Value: "safe"}, {Name: "nonce", Value: "token"}}, Input("body", "Message", "text", "", 10), "Save"))
	body := string(result.Body)
	csrf, nonce, value, submit := strings.Index(body, `name="csrf"`), strings.Index(body, `name="nonce"`), strings.Index(body, `name="body"`), strings.Index(body, `type="submit"`)
	if csrf < 0 || nonce < csrf || value < nonce || submit < value {
		t.Fatalf("native form order %s", body)
	}
	if strings.Contains(body, "/>") || strings.Contains(body, "&amp;") {
		t.Fatalf("native serialization %s", body)
	}
}

func TestTemplLinkRejectsAttributeInjectionAndUnsafeSchemes(t *testing.T) {
	for _, href := range []string{`http://x/" onclick="evil`, `javascript:alert(1)`, `http://user:password@example.test/`} {
		result := Render(200, "Link", Link(href, "Open"))
		body := string(result.Body)
		if !strings.Contains(body, "about:invalid#invalid-native-url") || strings.Contains(body, "onclick=") {
			t.Fatalf("unsafe href %q => %s", href, body)
		}
	}
}
