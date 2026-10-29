package acceptance

import (
	"context"
	"errors"
	"testing"

	"github.com/hailam/mig33/internal/faults"
	sessionview "github.com/hailam/mig33/internal/session"
)

func TestOperatorCommandsAuthorizeAtExecution(t *testing.T) {
	s := newTestServer(t)
	if err := s.Core.ConfigureOperators([]string{"alice"}); err != nil {
		t.Fatal(err)
	}
	id := login(t, s, "alice")
	auth := sessionview.OperatorSessionRequest{SessionID: id, CSRF: s.session(id).csrf}
	if _, err := s.Core.OperatorSession(context.Background(), auth); err != nil {
		t.Fatal(err)
	}
	assertForbidden := func(err error) {
		t.Helper()
		var failure *faults.ServiceError
		if !errors.As(err, &failure) || failure.Kind != faults.Forbidden {
			t.Fatalf("expected authorization failure, got %v", err)
		}
	}
	// A caller cannot bypass CSRF by marking a mutating command as a read.
	badCSRF := sessionview.OperatorSessionRequest{SessionID: id, CSRF: "wrong", Mutation: false}
	assertForbidden(s.Core.ChangeOperatorCategory(context.Background(), sessionview.OperatorCategoryRequest{
		OperatorSessionRequest: badCSRF, Action: "save", ID: 42, Name: "Unauthorized",
	}))
	assertForbidden(s.Core.ReviewOperatorItem(context.Background(), sessionview.OperatorReviewRequest{OperatorSessionRequest: badCSRF}))
	assertForbidden(s.Core.LogoutOperator(context.Background(), badCSRF))

	// A role revoked after the preliminary read must be checked by each command.
	if err := s.Core.ConfigureOperators([]string{"bob"}); err != nil {
		t.Fatal(err)
	}
	assertForbidden(s.Core.ChangeOperatorCategory(context.Background(), sessionview.OperatorCategoryRequest{
		OperatorSessionRequest: auth, Action: "save", ID: 42, Name: "Unauthorized",
	}))
	assertForbidden(s.Core.ReviewOperatorItem(context.Background(), sessionview.OperatorReviewRequest{OperatorSessionRequest: auth}))
	assertForbidden(s.Core.LogoutOperator(context.Background(), auth))
	if _, err := s.Core.Session(context.Background(), id); err != nil {
		t.Fatalf("rejected logout removed session: %v", err)
	}
}

func TestBrowserLogoutKeepsNativeSession(t *testing.T) {
	s := newTestServer(t)
	id := login(t, s, "alice")
	err := s.Core.LogoutBrowser(context.Background(), sessionview.BrowserLogout{SessionID: id, CSRF: s.session(id).csrf})
	var failure *faults.ServiceError
	if !errors.As(err, &failure) || failure.Kind != faults.Forbidden {
		t.Fatalf("native session accepted browser logout: %v", err)
	}
	if _, err := s.Core.Session(context.Background(), id); err != nil {
		t.Fatalf("native session removed: %v", err)
	}
}
