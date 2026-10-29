package pages

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestReportFormNativeContract(t *testing.T) {
	action := "http://127.0.0.1:8080/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=Lobby"
	r := ReportForm(action, "Lobby", "csrf", "nonce", "binding", []string{"bob", "carol"})
	body := string(r.Body)
	if r.Status != 200 || !strings.Contains(body, `<title>Report Abuse</title>`) || !strings.Contains(body, `action="`+action+`"`) {
		t.Fatalf("native form: %s", body)
	}
	parsed, err := url.Parse(action)
	if err != nil || parsed.Query().Get("c") != "report" {
		t.Fatal("action lost routing")
	}
	fields := regexp.MustCompile(`name="([^"]+)"`).FindAllStringSubmatch(body, -1)
	want := []string{"csrf", "nonce", "binding", "category", "target", "details"}
	if len(fields) != len(want) {
		t.Fatalf("fields: %v", fields)
	}
	for i, name := range want {
		if fields[i][1] != name {
			t.Fatalf("field order: %v", fields)
		}
	}
	if !strings.Contains(body, `<input type="submit" value="Submit Report">`) || strings.Contains(body, `&amp;a=`) {
		t.Fatal("unsupported native submission contract")
	}
	if !strings.Contains(body, `<textarea name="details" rows="4" cols="10">`) {
		t.Fatal("native textarea requires the full-width cols=10 sentinel")
	}
}

func TestReportEscapingAndTruthfulConfirmation(t *testing.T) {
	body := string(ReportForm("http://127.0.0.1:8080/sites/index.php", `<script>alert("x")</script>`, "csrf", "nonce", "binding", []string{`bob"><input name="bad">`}).Body)
	if strings.Contains(body, "<script>") || strings.Contains(body, `<input name="bad">`) {
		t.Fatal("report context injected HTML")
	}
	saved := string(ReportSaved(42).Body)
	if !strings.Contains(saved, "Report #42 saved for local review.") || !strings.Contains(saved, "No moderation action has been taken automatically.") {
		t.Fatal("confirmation overstates action")
	}
}
