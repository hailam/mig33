package realtime

import (
	"context"
	"time"

	"github.com/hailam/mig33/internal/faults"
	sessionview "github.com/hailam/mig33/internal/session"
)

// requireSession and sessionView are called with the application's state lock.
func (s *Coordinator) requireSession(ctx context.Context, id string) (*session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	x := s.active(ctx, id)
	if x == nil || !x.authenticated {
		return nil, faults.NewError(faults.Unauthenticated, "Login required")
	}
	x.touched = time.Now()
	return x, nil
}

func (s *Coordinator) sessionView(x *session) sessionview.View {
	return sessionview.View{ID: x.id, Username: x.username, CSRF: x.csrf, WebOnly: x.webOnly, Operator: s.operators[x.username], TTL: sessionTTL}
}

func (s *Coordinator) Session(ctx context.Context, id string) (sessionview.View, error) {
	if err := ctx.Err(); err != nil {
		return sessionview.View{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, id)
	if err != nil {
		return sessionview.View{}, err
	}
	return s.sessionView(x), nil
}

func validToken(id string) bool { return sessionview.ValidToken(id) }
