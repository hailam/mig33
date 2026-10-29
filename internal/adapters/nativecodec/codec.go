package nativecodec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/protocol"
)

// Codec encodes native responses using optional, locally supplied client
// resources. Each instance owns its sprite cache. The supplied filesystem must
// support concurrent reads and should remain unchanged while the codec is used.
type Codec struct {
	resources fs.FS
	spritesMu sync.Mutex
	sprites   [][]byte
}

// New accepts nil when optional client resources have not been installed. Core
// login and chat responses remain available without them.
func New(resources fs.FS) *Codec { return &Codec{resources: resources} }

func (c *Codec) readResource(ctx context.Context, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.resources == nil {
		return nil, faults.WrapError(faults.Unavailable, "Client resource unavailable", fs.ErrNotExist)
	}

	data, err := fs.ReadFile(c.resources, name)
	// fs.FS has no context-aware read operation. Observe cancellation before
	// and after the read, and never retain a cancelled result in the cache.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, faults.WrapError(faults.Unavailable, "Client resource unavailable", err)
	}

	return data, nil
}

// Decode retains field validity without running application validation. The
// coordinator resolves the session and authorizes before validating its command.
func Decode(packet *protocol.Packet, sessionID string) *commands.Request {
	if packet == nil {
		return &commands.Request{Input: commands.Unknown{Description: "Missing packet"}}
	}

	var request *commands.Request
	for _, decode := range []func(*protocol.Packet) (*commands.Request, bool){DecodeContact, DecodeRoom, DecodeGroup, DecodeServices} {
		if result, ok := decode(packet); ok {
			request = result
			break
		}
	}
	if request == nil {
		var input commands.Command
		switch packet.Opcode {
		case 2:
			input = commands.Ping{}
		case 10:
			input = commands.Telemetry{}
		case 101:
			input = commands.Captcha{}
		case 103:
			input = commands.Register{
				Username:  decodeFeatureField(packet, 1, packet.Text),
				Password:  decodeFeatureField(packet, 2, packet.Text),
				Mobile:    decodeFeatureField(packet, 3, packet.Text),
				Challenge: decodeFeatureField(packet, 4, packet.Text),
				Code:      decodeFeatureField(packet, 5, packet.Text),
			}
		case 200:
			input = commands.Login{Username: decodeFeatureField(packet, 5, packet.Text), Presence: decodeFeatureField(packet, 9, packet.Byte)}
		case 202:
			input = commands.Authenticate{Proof: decodeFeatureField(packet, 1, packet.Int)}
		case 204:
			input = commands.Activate{Code: decodeFeatureField(packet, 1, packet.Text)}
		case 205:
			input = commands.SetMobile{Mobile: decodeFeatureField(packet, 1, packet.Text)}
		case 300:
			input = commands.Logout{}
		case 417:
			input = commands.Block{Username: decodeFeatureField(packet, 1, packet.Text), Mode: decodeFeatureField(packet, 2, packet.Byte)}
		case 500:
			input = commands.SendMessage{
				Channel: decodeFeatureField(packet, 1, packet.Byte),
				Kind:    decodeFeatureField(packet, 3, packet.Byte),
				Content: decodeFeatureField(packet, 6, packet.Short),
				Target:  decodeFeatureField(packet, 4, packet.Text),
				Body:    decodeFeatureField(packet, 8, packet.Text),
				Color:   decodeFeatureField(packet, 13, packet.Int),
				Photo:   append([]byte(nil), packet.Fields[8]...),
			}
		case 600:
			input = commands.SetPresence{Presence: decodeFeatureField(packet, 1, packet.Byte)}
		case 601:
			input = commands.SetStatus{Status: decodeFeatureField(packet, 1, packet.Text)}
		default:
			input = commands.Unknown{Description: fmt.Sprintf("Operation %d is not implemented in the local prototype", packet.Opcode)}
		}

		request = &commands.Request{Input: input}
	}

	request.Correlation = packet.Transaction
	var encoded bytes.Buffer
	// The prior client contract hashes the canonical re-encoding of the decoded
	// frame, including its transaction and unknown fields, scoped to the session.
	_ = protocol.Encode(&encoded, packet)
	digest := sha256.Sum256(append([]byte(sessionID+":"), encoded.Bytes()...))
	request.Nonce = hex.EncodeToString(digest[:24])
	return request
}

func (c *Codec) Encode(ctx context.Context, event *events.Event, publicURL string) *protocol.Packet {
	if event == nil {
		return nil
	}

	for _, encode := range []func(*events.Event) (*protocol.Packet, bool){EncodeContact, EncodeRoom, EncodeGroup} {
		if packet, ok := encode(event); ok {
			return packet
		}
	}
	if packet, ok := c.EncodeServices(ctx, event, publicURL); ok {
		return packet
	}

	switch payload := event.Data.(type) {
	case events.Acknowledged:
		packet := protocol.New(1, event.Correlation)
		if payload.Message != nil {
			packet.SetText(1, *payload.Message)
		}

		return packet
	case events.Rejected:
		return protocol.New(0, event.Correlation).SetText(2, payload.Message)
	case events.Alert:
		return protocol.New(6, event.Correlation).SetText(1, payload.Message)
	case events.Popup:
		return protocol.New(5, event.Correlation).SetByte(1, 0).SetText(2, payload.Message).SetByte(3, 1).SetText(4, payload.Title)
	case events.ColorPalette:
		return protocol.New(924, event.Correlation).SetText(2, "16711680;32768;255;8388736;16744192;3368601;10040064;8421504")
	case events.Pong:
		return protocol.New(3, event.Correlation)
	case events.Challenge:
		return protocol.New(201, event.Correlation).SetText(1, payload.Challenge).SetText(2, payload.SessionID)
	case events.Authenticated:
		activated := byte(0)
		if payload.Activated {
			activated = 1
		}

		return protocol.New(203, event.Correlation).SetText(1, payload.Mobile).SetByte(2, activated).
			SetInt(14, 0).SetInt(20, payload.RosterRevision).SetText(23, publicURL+"/assets/").
			SetText(26, publicURL+"/sites/index.php").SetByte(27, 0).SetText(28, payload.Status).SetByte(29, 0).
			SetShort(31, 0).SetText(32, "").SetInt(33, 1).SetInt(35, 1).SetInt(37, 1)
	case events.Captcha:
		return protocol.New(102, event.Correlation).SetText(1, "").SetText(2, payload.Challenge).SetBytes(3, payload.Image)
	case events.RegistrationComplete:
		return protocol.New(1, event.Correlation).SetText(1, payload.Message)
	case events.Status:
		return protocol.New(603, event.Correlation).SetText(1, payload.Text).SetText(3, payload.Text)
	case events.Profile:
		packet := protocol.New(603, event.Correlation)
		if payload.Status != nil {
			packet.SetText(1, *payload.Status).SetText(3, *payload.Status)
		}
		if payload.Avatar != nil {
			packet.SetText(2, *payload.Avatar)
		}

		return packet
	case events.Avatar:
		return protocol.New(421, event.Correlation).SetInt(1, payload.ContactID).SetText(2, payload.Key)
	case events.Message:
		return encodeMessage(event.Correlation, payload)
	case events.Notice:
		return protocol.New(500, event.Correlation).SetByte(1, 1).SetText(2, payload.Target).SetByte(3, payload.Kind).SetText(4, payload.Target).SetShort(6, 1).SetText(8, payload.Text)
	default:
		return protocol.New(0, event.Correlation).SetText(2, "Unsupported response")
	}
}

func encodeMessage(correlation int16, payload events.Message) *protocol.Packet {
	message := payload.Message
	packet := protocol.New(500, correlation).SetByte(1, 1).SetText(2, message.Sender).SetByte(3, message.Kind).SetText(4, message.Conversation).SetShort(6, 1).SetText(8, message.Body)
	aliases := append([]string(nil), payload.Emoticons...)
	if message.PhotoID != 0 {
		packet.SetShort(6, 5)
		aliases = append([]string{"[photo:" + strconv.FormatInt(message.PhotoID, 10) + "]"}, aliases...)
	}
	if len(aliases) != 0 {
		packet.SetText(11, strings.Join(aliases, " "))
	}
	if message.Action {
		packet.SetShort(6, 6)
		if message.Kind == 1 && payload.Viewer == message.Sender {
			packet.SetText(2, message.Conversation)
		}
	} else {
		if payload.Avatar != "" {
			packet.SetText(10, payload.Avatar)
		}
		if message.Color != nil {
			packet.SetInt(13, *message.Color)
		}
	}
	if payload.OwnMirror && message.Kind == 1 {
		packet.SetText(2, message.Conversation).SetShort(6, 6).SetText(8, message.Sender+": "+message.Body)
		delete(packet.Fields, 10)
		delete(packet.Fields, 13)
	}
	if payload.ContactID != nil {
		packet.SetInt(5, *payload.ContactID)
	}

	return packet
}
