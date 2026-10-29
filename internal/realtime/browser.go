package realtime

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/review"
	sessionview "github.com/hailam/mig33/internal/session"
)

func (s *Coordinator) LoginBrowser(ctx context.Context, request sessionview.BrowserLogin) (sessionview.View, error) {
	if err := ctx.Err(); err != nil {
		return sessionview.View{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return sessionview.View{}, err
	}
	name := strings.ToLower(strings.TrimSpace(request.Username))
	a := s.accounts[name]
	if a == nil || !a.identity.Activated || !accounts.VerifyPassword(a.identity, request.Password) {
		return sessionview.View{}, faults.NewError(faults.Unauthenticated, "Invalid credentials or account is not activated")
	}
	if err := s.browserSessionCapacity(ctx); err != nil {
		return sessionview.View{}, err
	}
	x := s.newBrowserSession(name, true)
	s.broadcastPresence(ctx, name)
	return s.sessionView(x), nil
}

// browserSessionCapacity and newBrowserSession require the application state lock.
func (s *Coordinator) browserSessionCapacity(ctx context.Context) error {
	for id := range s.sessions {
		s.active(ctx, id)
	}
	if len(s.sessions) >= 256 {
		return faults.NewError(faults.Limited, "Too many sessions")
	}
	return nil
}

func (s *Coordinator) newBrowserSession(username string, webOnly bool) *session {
	now := time.Now()
	x := &session{
		id: token(), username: username, csrf: token(), created: now, touched: now,
		authenticated: true, presence: 99, webOnly: webOnly, push: make(chan *events.Event, 128),
		inflight: make(map[int64]*events.Event), rooms: make(map[string]bool),
	}
	if webOnly {
		x.presence = 1
	}
	s.sessions[x.id] = x
	return x
}

func (s *Coordinator) LogoutBrowser(ctx context.Context, request sessionview.BrowserLogout) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, request.SessionID)
	if err != nil {
		return err
	}
	if !x.webOnly || !validCSRF(x, request.CSRF) {
		return faults.NewError(faults.Forbidden, "Invalid sign-out request")
	}
	s.removeSession(ctx, x.id)
	return nil
}

func (s *Coordinator) OperatorsEnabled(context.Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.operators) != 0
}

func (s *Coordinator) LoginOperator(ctx context.Context, request sessionview.OperatorLogin) (sessionview.View, error) {
	if err := ctx.Err(); err != nil {
		return sessionview.View{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return sessionview.View{}, err
	}
	if len(s.operators) == 0 {
		return sessionview.View{}, faults.NewError(faults.NotFound, "Operator access is not configured")
	}
	name := strings.ToLower(strings.TrimSpace(request.Username))
	a := s.accounts[name]
	if !s.operators[name] || a == nil || !accounts.VerifyPassword(a.identity, request.Password) {
		return sessionview.View{}, faults.NewError(faults.Unauthenticated, "Invalid operator credentials")
	}
	if err := s.browserSessionCapacity(ctx); err != nil {
		return sessionview.View{}, err
	}
	if old := s.sessions[request.PreviousSessionID]; old != nil && old.username == name {
		s.removeSession(ctx, request.PreviousSessionID)
	}
	return s.sessionView(s.newBrowserSession(name, false)), nil
}

// requireOperator authenticates again inside each command's lock, so a role or
// session change cannot race the command after a controller's preliminary check.
func (s *Coordinator) requireOperator(ctx context.Context, request sessionview.OperatorSessionRequest) (*session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	x := s.active(ctx, request.SessionID)
	if x == nil || !x.authenticated {
		return nil, faults.NewError(faults.Unauthenticated, "Sign in again")
	}
	if !s.operators[x.username] {
		return nil, faults.NewError(faults.Forbidden, "Operator access required")
	}
	x.touched = time.Now()
	if request.Mutation && (x.csrf == "" || subtle.ConstantTimeCompare([]byte(request.CSRF), []byte(x.csrf)) != 1) {
		return nil, faults.NewError(faults.Forbidden, "Reopen this form")
	}
	return x, nil
}

func (s *Coordinator) OperatorSession(ctx context.Context, request sessionview.OperatorSessionRequest) (sessionview.View, error) {
	if err := ctx.Err(); err != nil {
		return sessionview.View{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireOperator(ctx, request)
	if err != nil {
		return sessionview.View{}, err
	}
	return s.sessionView(x), nil
}

func (s *Coordinator) LogoutOperator(ctx context.Context, request sessionview.OperatorSessionRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	request.Mutation = true
	x, err := s.requireOperator(ctx, request)
	if err != nil {
		return err
	}
	s.removeSession(ctx, x.id)
	return nil
}

func (s *Coordinator) ReviewOperatorItem(ctx context.Context, request sessionview.OperatorReviewRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	request.Mutation = true
	x, err := s.requireOperator(ctx, request.OperatorSessionRequest)
	if err != nil {
		return err
	}
	return s.reviewService.Decide(ctx, review.OperatorReview{
		Kind: request.Kind, ItemID: request.ItemID,
		Actor: x.username, Status: request.Status, Note: request.Note, Nonce: request.Nonce,
	}, request.Expected)
}

func (s *Coordinator) ChangeOperatorCategory(ctx context.Context, request sessionview.OperatorCategoryRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	request.Mutation = true
	x, err := s.requireOperator(ctx, request.OperatorSessionRequest)
	if err != nil {
		return err
	}
	switch request.Action {
	case "assign":
		return s.roomsService.SetRoomCategory(ctx, x.username, request.Room, request.ID)
	case "save", "delete":
		return s.roomsService.ManageRoomCategory(ctx, x.username, request.ID, request.Name, request.Action == "delete")
	default:
		return faults.ErrInvalid
	}
}
