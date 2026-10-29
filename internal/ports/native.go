// Package ports contains the native transport contract shared by socket and HTTP adapters.
package ports

import (
	"context"

	"github.com/hailam/mig33/internal/protocol"
)

// NativeService preserves ordered delivery across the socket and HTTP tunnel.
// Business capabilities receive typed commands through the native adapter.
type NativeService interface {
	Handle(context.Context, string, *protocol.Packet) ([]*protocol.Packet, string)
	DropSession(context.Context, string)
	BeginWrite(string) func()
	AllowPacket(context.Context, string, *protocol.Packet) bool
	FinishWrite(context.Context, []*protocol.Packet, bool)
	BeginResponse(string) func()
	TryBeginResponse(string) (func(), bool)
	Poll(context.Context, string, int) []*protocol.Packet
}
