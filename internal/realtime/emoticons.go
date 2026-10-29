package realtime

import (
	"context"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
)

func (s *Coordinator) catalogEmoticon(request *commands.Request, name string) *events.Event {
	if _, ok := catalog.LookupEmoticon(name); !ok {
		return events.Failure(request.Correlation, "Unknown downloadable emoticon")
	}
	return events.New(request.Correlation, events.EmoticonResource{Token: name})
}

func (s *Coordinator) initialEmoticonCatalog(ctx context.Context, owner string) (*events.Event, error) {
	tokens, err := s.catalogService.OwnedEmoticons(ctx, owner)
	if err != nil {
		return nil, err
	}
	return events.New(0, events.Emoticons{Tokens: tokens}), nil
}

func (s *Coordinator) validateEmoticonMessage(ctx context.Context, owner, body string) error {
	return s.catalogService.ValidateEmoticons(ctx, owner, body)
}

func decorateEmoticonMessage(event *events.Event, body string) {
	message, ok := event.Data.(events.Message)
	if !ok {
		return
	}
	message.Emoticons = append(message.Emoticons, catalog.ReferencedEmoticons(body)...)
	event.Data = message
}

func (s *Coordinator) refreshEmoticonCatalog(ctx context.Context, owner string) {
	for _, x := range s.sessions {
		if x.authenticated && x.username == owner {
			x.emoticonsStale = true
			s.flushEmoticonCatalog(ctx, x)
		}
	}
}

func (s *Coordinator) flushEmoticonCatalog(ctx context.Context, x *session) {
	if !x.authenticated || !x.ready || !x.emoticonsStale || len(x.push) == cap(x.push) {
		return
	}
	event, err := s.initialEmoticonCatalog(ctx, x.username)
	if err != nil {
		s.logError(ctx, "load emoticon catalog: %v", err)
		return
	}
	x.push <- event
	x.emoticonsStale = false
}
