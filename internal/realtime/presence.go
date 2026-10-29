package realtime

import (
	"context"
	"fmt"
	"time"
)

const (
	sessionSweepInterval = 30 * time.Second
	maintenanceBudget    = 5 * time.Second
)

// Run owns session and vote expiry work. It starts no detached workers and
// returns only when its final maintenance operation has completed.
func (s *Coordinator) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.closed || !s.initialized || s.running {
		s.mu.Unlock()
		return fmt.Errorf("application must be initialized, open and not already running")
	}
	s.running = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.running = false; s.mu.Unlock() }()
	nextSweep := time.Now().Add(sessionSweepInterval)
	timer := time.NewTimer(sessionSweepInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.stop:
			return nil
		case <-s.maintenance:
		case <-timer.C:
		}
		work, cancel := context.WithTimeout(ctx, maintenanceBudget)
		s.mu.Lock()
		if s.closed || work.Err() != nil {
			s.mu.Unlock()
			cancel()
			continue
		}
		now := time.Now()
		if !now.Before(nextSweep) {
			s.expireSessions(work)
			nextSweep = now.Add(sessionSweepInterval)
		}
		wake := nextSweep
		for room, vote := range s.kickVotes {
			if !now.Before(vote.ExpiresAt) {
				if err := s.pruneKickVote(work, room, now); err != nil {
					s.logError(work, "check room vote eligibility: %v", err)
				}
			}
			if current := s.kickVotes[room]; current != nil && current.ExpiresAt.Before(wake) {
				wake = current.ExpiresAt
			}
		}
		s.mu.Unlock()
		cancel()
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(max(time.Until(wake), time.Millisecond))
	}
}

func (s *Coordinator) wakeMaintenance() {
	select {
	case s.maintenance <- struct{}{}:
	default:
	}
}

// expireSessions is called under the application state lock.
func (s *Coordinator) expireSessions(ctx context.Context) {
	for id := range s.sessions {
		s.active(ctx, id)
	}
}
