package session

import "time"

type BrowserLogin struct {
	Username string
	Password string
}

type BrowserLogout struct {
	SessionID string
	CSRF      string
}

type OperatorLogin struct {
	Username          string
	Password          string
	PreviousSessionID string
}

type OperatorSessionRequest struct {
	SessionID string
	CSRF      string
	Mutation  bool
}

type OperatorReviewRequest struct {
	OperatorSessionRequest
	Kind     string
	ItemID   int64
	Status   string
	Note     string
	Nonce    string
	Expected string
}

type OperatorCategoryRequest struct {
	OperatorSessionRequest
	Action string
	ID     int16
	Room   string
	Name   string
}

type OperatorDashboardRequest struct {
	SessionID     string
	Kind          string
	ItemID        string
	ReportBefore  string
	SupportBefore string
}

// View is an immutable authenticated session snapshot.
type View struct {
	ID, Username, CSRF string
	WebOnly, Operator  bool
	TTL                time.Duration
}

func ValidToken(id string) bool {
	if len(id) != 48 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
