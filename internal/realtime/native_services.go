package realtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
)

// IDs/channels are the recovered v3 mapping advertised by v4 descriptors.
// These directories store local contacts; external providers stay disconnected.
type nativeIMService struct {
	ID, Channel byte
	Name, Key   string
}

var nativeIMServices = []nativeIMService{{2, 4, "MSN", "msn"}, {3, 5, "AIM", "aim"}, {4, 6, "Yahoo", "yahoo"}, {6, 8, "Google Talk", "gtalk"}}

func nativeIM(id byte) (nativeIMService, bool) {
	for _, service := range nativeIMServices {
		if service.ID == id {
			return service, true
		}
	}
	return nativeIMService{}, false
}

func nativeIMDescriptor(service nativeIMService) *events.Event {
	return events.New(0, events.Provider{ID: service.ID, Channel: service.Channel, Name: service.Name})
}

func (s *Coordinator) initialNativeServices(ctx context.Context, x *session) []*events.Event {
	out := []*events.Event{}
	for _, service := range nativeIMServices {
		out = append(out, nativeIMDescriptor(service))
	}

	event, err := s.initialEmoticonCatalog(ctx, x.username)
	if err != nil {
		s.logError(ctx, "load initial emoticon catalog: %v", err)
		x.emoticonsStale = true
		return out
	}

	x.emoticonsStale = false
	return append(out, event)
}

func nativeRequestNonce(_ *session, request *commands.Request) string { return request.Nonce }

func colorPalette() *events.Event { return events.New(0, events.ColorPalette{}) }

func (s *Coordinator) nativeProviderMessage(ctx context.Context, x *session, request *commands.Request) (*events.Event, bool) {
	input := request.Input.(commands.SendMessage)
	channel, ok := input.Channel.Get()
	if ok && channel == 1 {
		return nil, false
	}
	if channel != 2 && channel != 9 {
		return events.Failure(request.Correlation, "External IM messaging is disconnected; use mig33 chat or save a provider request"), true
	}

	kind, okKind := input.Kind.Get()
	content, okContent := input.Content.Get()
	target, okTarget := input.Target.Get()
	body, okBody := input.Body.Get()
	if !okKind ||
		kind != 1 ||
		!okContent ||
		content != 1 ||
		!okTarget ||
		!okBody ||
		len(body) > 4000 ||
		strings.TrimSpace(body) == "" ||
		!utf8.ValidString(body) ||
		strings.ContainsRune(body, 0) {
		return events.Failure(request.Correlation, "Use a private text message of1 to4000 UTF-8 bytes"), true
	}

	command := activity.ProviderCommand{Kind: "mail", Actor: x.username, Target: target, Body: body, Nonce: request.Nonce}
	if channel == 2 {
		phone, err := accounts.NormalizeMobile(target)
		if err != nil {
			return events.Failure(request.Correlation, "SMS requires a valid mobile number"), true
		}
		command.Kind, command.Target = "sms", phone
	} else {
		exists, err := s.privacy.Exists(ctx, strings.ToLower(target))
		if err != nil {
			return events.Failure(request.Correlation, "Provider message could not be saved"), true
		}
		if exists {
			command.Target = strings.ToLower(target)
			allowed, err := s.privacy.CanContact(ctx, x.username, command.Target)
			blocked, blockErr := s.privacy.BlocksIncoming(ctx, command.Target, x.username)
			if err != nil || blockErr != nil || !allowed || blocked {
				return events.Failure(request.Correlation, "Recipient privacy settings do not allow this mail"), true
			}
		} else if err := s.activityService.ValidateProvider(ctx, "mail", target); err != nil {
			return events.Failure(request.Correlation, "Mail requires a local username or valid email address"), true
		}
	}

	var saved activity.ServiceEntry
	var err error
	if channel == 2 {
		saved, err = s.activityService.RecordTelephone(ctx, command)
	} else {
		saved, err = s.activityService.RecordProvider(ctx, command)
	}
	if err != nil {
		return events.Failure(request.Correlation, "Provider message could not be saved"), true
	}

	notice := fmt.Sprintf("Request #%d: %s.", saved.ID, saved.State)
	if saved.State == "pending_provider" {
		notice += " No external message was sent."
	}

	// Native channel2 ACK hardcodes 'SMS sent'; a rejection carries the truthful
	// recorded-request result without asserting a disconnected provider sent it.
	if channel == 2 {
		return events.Failure(request.Correlation, notice), true
	}
	return events.New(request.Correlation, events.Acknowledged{Message: &notice}), true
}

func (s *Coordinator) nativeServicePackets(ctx context.Context, x *session, request *commands.Request) []*events.Event {
	bad := func(message string) []*events.Event {
		return []*events.Event{events.Failure(request.Correlation, message)}
	}

	ack := events.Ack(request.Correlation)
	switch input := request.Input.(type) {
	case commands.SetAvatar:
		key, ok := input.Key.Get()
		if !ok {
			return bad("Display picture ID required")
		}
		if key == "" || key == "default-avatar.png" {
			change, err := s.mediaService.ClearAvatar(ctx, x.username)
			if err != nil {
				return bad("Display picture could not be cleared")
			}
			s.broadcastAvatar(ctx, change.Owner, change.Key)
			return []*events.Event{ack, events.New(0, events.Profile{Avatar: &change.Key})}
		}
		if !strings.HasPrefix(key, "photos/") {
			return bad("Select a display picture from your scrapbook")
		}

		id, err := strconv.ParseInt(strings.TrimPrefix(key, "photos/"), 10, 64)
		if err != nil || id <= 0 {
			return bad("Invalid display picture")
		}
		if err := s.setPhotoAvatar(ctx, x, id); err != nil {
			return bad("Display picture must belong to your scrapbook")
		}
		return []*events.Event{ack, events.New(0, events.Profile{Avatar: &key})}
	case commands.RecordCallback:
		source, ok := input.Source.Get()
		destination, secondOK := input.Destination.Get()
		if !ok || !secondOK || !utf8.ValidString(destination) || strings.ContainsRune(destination, 0) {
			return bad("Complete the request details")
		}

		phone, err := accounts.NormalizeMobile(source)
		if err != nil {
			return bad(err.Error())
		}

		mode, modeOK := input.Mode.Get()
		confirm, confirmOK := input.Confirm.Get()
		if !modeOK || !confirmOK || mode != 0 || confirm != 1 {
			return bad("Unsupported callback mode")
		}

		target, err := accounts.NormalizeMobile(destination)
		if err != nil {
			return bad("Callback destination requires a telephone number")
		}

		saved, err := s.activityService.RecordTelephone(ctx, activity.ProviderCommand{Kind: "call", Actor: x.username, Target: target, Body: phone, Nonce: request.Nonce})
		if err != nil {
			return bad("Request could not be saved")
		}

		notice := fmt.Sprintf("Request #%d saved; pending_provider. No call or SMS was sent.", saved.ID)
		return []*events.Event{
			events.New(request.Correlation, events.Acknowledged{Message: &notice}),
			events.New(0, events.ProviderRequestSaved{Message: notice, Title: "Callback request"}),
		}
	case commands.InviteMobile:
		first, ok := input.Phone.Get()
		name, secondOK := input.Name.Get()
		if !ok || !secondOK || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
			return bad("Complete the request details")
		}

		phone, err := accounts.NormalizeMobile(first)
		if err != nil {
			return bad(err.Error())
		}
		if len(name) > 128 || strings.TrimSpace(name) == "" {
			return bad("Invitee name is required (at most128 bytes)")
		}

		saved, err := s.activityService.RecordTelephone(ctx, activity.ProviderCommand{Kind: "invite", Actor: x.username, Target: phone, Body: name, Nonce: request.Nonce})
		if err != nil {
			return bad("Request could not be saved")
		}

		notice := fmt.Sprintf("Request #%d saved; pending_provider. No call or SMS was sent.", saved.ID)
		return []*events.Event{events.New(request.Correlation, events.Acknowledged{Message: &notice})}
	case commands.ConnectDirectory:
		id, ok := input.ID.Get()
		_, known := nativeIM(id)
		if !ok || !known {
			return bad("Unknown external IM service")
		}

		mode, modeOK := input.Mode.Get()
		option, optionOK := input.Option.Get()
		if !modeOK || !optionOK || mode != 1 || option != 0 {
			return bad("Unsupported local directory connection mode")
		}
		notice := "Local saved contacts are available. The external IM provider is disconnected; no external sign-in occurred."
		return []*events.Event{events.New(request.Correlation, events.Acknowledged{Message: &notice})}
	case commands.DisconnectDirectory:
		id, ok := input.ID.Get()
		service, known := nativeIM(id)
		if !ok || !known {
			return bad("Unknown external IM service")
		}
		return []*events.Event{ack, nativeIMDescriptor(service)}
	case commands.RemoveDirectorySettings:
		id, ok := input.ID.Get()
		service, known := nativeIM(id)
		if !ok || !known {
			return bad("Unknown external IM service")
		}
		mode, ok := input.Mode.Get()
		if !ok || mode != 1 {
			return bad("Unsupported IM settings operation")
		}
		if err := s.contactsService.RemoveIMLabels(ctx, x.username, service.Key); err != nil {
			return bad("IM settings could not be removed")
		}
		return []*events.Event{ack, nativeIMDescriptor(service)}
	case commands.ChooseLanguage:
		code, ok := input.Code.Get()
		if !ok || code != "en-US" {
			return bad("Only the recovered English language pack is available")
		}
		if err := s.accountService.SetLanguage(ctx, x.username, code); err != nil {
			return bad("Language selection could not be saved")
		}
		return []*events.Event{ack}
	case commands.ProviderIcons:
		if len(input.IDs) == 0 || len(input.IDs) > len(nativeIMServices) {
			return bad("Select one or more supported IM services")
		}
		seen := map[byte]bool{}
		for _, id := range input.IDs {
			if _, ok := nativeIM(id); !ok || seen[id] {
				return bad("Unknown or duplicate IM service")
			}
			seen[id] = true
		}
		out := make([]*events.Event, 0, len(input.IDs))
		for _, id := range input.IDs {
			out = append(out, events.New(request.Correlation, events.ProviderIcons{ID: id}))
		}
		return out
	}
	return bad("Unknown native service operation")
}
