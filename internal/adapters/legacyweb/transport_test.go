package legacyweb

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/session"
)

func TestNativeFormsKeepUnescapedFinalText(t *testing.T) {
	const tail = "literal&+=% value😀"
	for _, test := range []struct{ name, path, body, key string }{
		{"registration", "/sites/index.php?c=registration&a=home&v=midlet", "csrf=abc&username=user&mobile=123&code=123456&agree=on&password=literal&+=%%20value😀", "password"},
		{"profile", "/sites/index.php?c=profile&a=edit&v=midlet", "csrf=abc&value=literal&+=%%20value😀", "value"},
		{"recovery", "/sites/index.php?c=account&a=recover&v=midlet&stage=complete", "csrf=abc&code=123456&password=literal&+=%%20value😀", "password"},
		{"room", "/sites/index.php?c=chatroom&a=create&v=midlet&stage=details", "csrf=abc&room=ff&binding=abc&description=literal&+=%%20value😀", "description"},
		{"service", "/sites/index.php?c=buzz&a=home&v=midlet", "csrf=abc&nonce=abc&op=publish&target=aa&body=literal&+=%%20value😀", "body"},
		{"wap", "/member2/t.php?kind=private&target=bob", "csrf=abc&nonce=abc&op=send&format=plain&body=literal&+=%%20value😀", "body"},
		{"report", "/sites/index.php?c=report&a=report_abuse&v=midlet", "csrf=abc&nonce=abc&binding=abc&category=other&details=literal&+=%%20value😀", "details"},
	} {
		t.Run(test.name, func(t *testing.T) {
			u, err := url.Parse(test.path)
			if err != nil {
				t.Fatal(err)
			}
			form, err := ParseNativeForm(u, test.body)
			if err != nil {
				t.Fatal(err)
			}
			if form.Get(test.key) != tail {
				t.Fatalf("tail=%q want=%q", form.Get(test.key), tail)
			}
			if len(form["+"]) != 0 {
				t.Fatal("tail became form controls")
			}
		})
	}
}

type publicWorkflow struct{ Workflow }

func (publicWorkflow) WebSession(context.Context, string) (session.View, []string, error) {
	return session.View{}, nil, faults.NewError(faults.Unauthenticated, "Login required")
}

func TestPublicUsernameSuggestionsMatchNativeBracketProtocol(t *testing.T) {
	repository, err := repositories.Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	credential, err := accounts.PasswordIdentity("alice", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"alice", "alice1", "alice3"} {
		credential.ID = int32(i + 1)
		credential.Username = name
		credential.Display = name
		if err := repository.SeedIdentity(t.Context(), credential); err != nil {
			t.Fatal(err)
		}
	}
	adapter := New(Dependencies{PublicURL: "http://127.0.0.1:8080", Accounts: accounts.NewService(repository), Workflow: publicWorkflow{}})
	path := "/sites/index.php?c=registration_api&a=check_username&v=midlet&username=alice"
	u, _ := url.Parse(path)
	http, err := adapter.FeaturePage(t.Context(), "", Request{Method: "GET", URL: u})
	if err != nil {
		t.Fatal(err)
	}
	native := adapter.NativePage(t.Context(), "", NativeRequest{URL: path, Method: 1, HasURL: true, HasMethod: true})
	if http.Status != 200 || string(http.Body) != "[alice2,alice4,alice5]" || string(native.Body) != string(http.Body) || native.Status != http.Status {
		t.Fatalf("suggestions HTTP=%+v native=%+v", http, native)
	}
	if strings.Contains(string(native.Body), `"`) {
		t.Fatal("JSON quotes break original suggestion picker")
	}
	for _, invalid := range []NativeRequest{
		{URL: "http://elsewhere.invalid" + path, Method: 1, HasURL: true, HasMethod: true},
		{URL: path, Method: 1, HasMethod: true},
		{URL: path, Method: 3, HasURL: true, HasMethod: true},
		{URL: path, Method: 2, HasURL: true, HasMethod: true, Body: strings.Repeat("x", 8193)},
	} {
		if result := adapter.NativePage(t.Context(), "", invalid); result.Status < 400 {
			t.Fatalf("accepted invalid native request: %+v", invalid)
		}
	}
}
