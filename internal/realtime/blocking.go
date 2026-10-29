package realtime

import (
	"context"
	"strings"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
)

func (s *Coordinator) blockUser(ctx context.Context, x *session, p *commands.Request) *events.Event {
	input := p.Input.(commands.Block)
	target, ok := input.Username.Get()
	target = strings.ToLower(target)
	mode, valid := input.Mode.Get()
	if !ok || !valid || mode != 2 || s.accounts[target] == nil || target == x.username {
		return failure(p, "Choose another registered user to block")
	}

	if err := s.contactsService.SetBlocked(ctx, x.username, target, true); err != nil {
		s.logError(ctx, "set personal block: %v", err)
		return failure(p, "Could not update your block list; try again")
	}
	s.cancelBlockedDeliveries(x.username, target)
	return events.Ack(p.Correlation)
}

func (s *Coordinator) cancelBlockedDeliveries(owner, target string) {
	for handle, d := range s.deliveries {
		if d.username == owner && d.actor == target {
			d.canceled = true
			s.deliveries[handle] = d
		}
	}
}

func (s *Coordinator) blocksIncoming(ctx context.Context, owner, sender string) bool {
	if owner == sender || s.accounts[sender] == nil {
		return false
	}
	blocked, err := s.privacy.BlocksIncoming(ctx, owner, sender)
	if err != nil {
		s.logError(ctx, "check personal block: %v", err)
		return true
	}
	return blocked
}
