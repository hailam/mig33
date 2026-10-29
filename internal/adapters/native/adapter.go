// Package native translates the original framed protocol to typed application
// commands and keeps transport acknowledgements associated with delivery events.
package native

import (
	"context"
	"sync"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/adapters/nativecodec"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/session"
)

// Application is the typed input and delivery port consumed by this adapter.
// The session owner implements it without depending on native packets.
type Application interface {
	Execute(context.Context, string, *commands.Request) ([]*events.Event, string)
	Poll(context.Context, string, int) []*events.Event
	RequireNative(context.Context, string) (session.View, error)
	BeginWrite(string) func()
	BeginResponse(string) func()
	TryBeginResponse(string) (func(), bool)
	AllowEvent(context.Context, string, *events.Event) bool
	FinishWrite(context.Context, []*events.Event, bool)
	DropSession(context.Context, string)
}

type Adapter struct {
	core      Application
	pages     *legacyweb.Adapter
	publicURL string
	codec     *nativecodec.Codec
	mu        sync.Mutex
	packets   map[*protocol.Packet]*events.Event
}

func New(core Application, pages *legacyweb.Adapter, publicURL string, codec *nativecodec.Codec) *Adapter {
	return &Adapter{
		core:      core,
		pages:     pages,
		publicURL: publicURL,
		codec:     codec,
		packets:   make(map[*protocol.Packet]*events.Event),
	}
}

func (a *Adapter) remember(p *protocol.Packet, e *events.Event) *protocol.Packet {
	a.mu.Lock()
	a.packets[p] = e
	a.mu.Unlock()

	return p
}

func (a *Adapter) encode(ctx context.Context, input []*events.Event) []*protocol.Packet {
	output := make([]*protocol.Packet, 0, len(input))
	for _, event := range input {
		packet := a.codec.Encode(ctx, event, a.publicURL)
		if packet == nil {
			a.core.FinishWrite(ctx, []*events.Event{event}, false)
			continue
		}

		output = append(output, a.remember(packet, event))
	}

	return output
}

func (a *Adapter) Handle(ctx context.Context, id string, p *protocol.Packet) ([]*protocol.Packet, string) {
	reply := func(packet *protocol.Packet, sid string) ([]*protocol.Packet, string) {
		return []*protocol.Packet{a.remember(packet, nil)}, sid
	}
	failure := func(text string) *protocol.Packet { return protocol.New(0, p.Transaction).SetText(2, text) }

	if err := ctx.Err(); err != nil {
		return reply(failure(err.Error()), id)
	}

	switch p.Opcode {
	case 7:
		return reply(protocol.New(6, p.Transaction).SetText(1, "Local registration\nChoose a username and password. Enter your mobile number and development security code "+accounts.DevelopmentOTP+".\nAfter login, activate with the same code. No SMS is sent."), id)
	case 904:
		return reply(failure("Local recovery: open "+a.publicURL+"/sites/index.php?c=account&a=recover&v=midlet and use development code "+accounts.DevelopmentOTP+". No SMS is sent."), id)
	case 922:
		view, err := a.core.RequireNative(ctx, id)
		if err != nil {
			return reply(failure(err.Error()), view.ID)
		}

		raw, hasURL := p.Text(2)
		method, hasMethod := p.Byte(1)
		body, _ := p.Text(3)
		purpose, _ := p.Int(4)

		result := a.pages.NativePage(ctx, id, legacyweb.NativeRequest{
			URL:       raw,
			Method:    method,
			Body:      body,
			Purpose:   purpose,
			HasURL:    hasURL,
			HasMethod: hasMethod,
		})
		return reply(protocol.New(923, p.Transaction).SetLong(3, result.TTL).SetInt(1, int32(result.Status)).SetBytes(2, result.Body), id)
	}

	replies, sid := a.core.Execute(ctx, id, nativecodec.Decode(p, id))
	return a.encode(ctx, replies), sid
}

func (a *Adapter) Poll(ctx context.Context, id string, limit int) []*protocol.Packet {
	if limit <= 0 {
		return nil
	}

	return a.encode(ctx, a.core.Poll(ctx, id, limit))
}

func (a *Adapter) BeginWrite(id string) func()               { return a.core.BeginWrite(id) }
func (a *Adapter) BeginResponse(id string) func()            { return a.core.BeginResponse(id) }
func (a *Adapter) TryBeginResponse(id string) (func(), bool) { return a.core.TryBeginResponse(id) }

func (a *Adapter) AllowPacket(ctx context.Context, id string, p *protocol.Packet) bool {
	a.mu.Lock()
	entry, ok := a.packets[p]
	a.mu.Unlock()
	if !ok {
		return false
	}

	allowed := entry == nil || a.core.AllowEvent(ctx, id, entry)
	if !allowed {
		a.mu.Lock()
		delete(a.packets, p)
		a.mu.Unlock()
	}

	return allowed
}

func (a *Adapter) FinishWrite(ctx context.Context, packets []*protocol.Packet, written bool) {
	var output []*events.Event

	a.mu.Lock()
	for _, packet := range packets {
		if entry, ok := a.packets[packet]; ok {
			if entry != nil {
				output = append(output, entry)
			}

			delete(a.packets, packet)
		}
	}
	a.mu.Unlock()

	a.core.FinishWrite(ctx, output, written)
}

func (a *Adapter) DropSession(ctx context.Context, id string) {
	// A transport owns every emitted frame until FinishWrite (or denied
	// admission). Closing a shared session must not discard another transport's
	// acknowledgment for an already committed command. Core admission still
	// rejects chat and activity events after their receiving session closes.
	a.core.DropSession(ctx, id)
}
