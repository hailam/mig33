package nativecodec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func TestDecodePreservesHistoricalNativeRetryIdentity(t *testing.T) {
	packet := protocol.New(500, 47).SetByte(1, 1).SetByte(3, 1).SetText(4, "bob").SetShort(6, 1).SetText(8, "hello").SetText(99, "unknown future field")
	var raw bytes.Buffer
	if err := protocol.Encode(&raw, packet); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(append([]byte("session:"), raw.Bytes()...))
	request := Decode(packet, "session")
	if request.Nonce != hex.EncodeToString(digest[:24]) || request.Correlation != 47 {
		t.Fatalf("retry contract: %+v", request)
	}
	if request.Input.(commands.SendMessage).Body.Value != "hello" {
		t.Fatal(request.Input)
	}
	if request.Nonce == Decode(packet, "another-session").Nonce {
		t.Fatal("retry identity crossed sessions")
	}
	packet.SetText(99, "changed")
	if request.Nonce == Decode(packet, "session").Nonce {
		t.Fatal("unknown fields removed from retry identity")
	}
}

func TestOwnPrivateMirrorChangesOnlyEncodedView(t *testing.T) {
	color := int32(0x123456)
	id := int32(12)
	message := chat.Message{Kind: 1, Sender: "alice", Conversation: "bob", Body: "hello [smile]", Color: &color}
	event := events.New(0, events.Message{Message: message, Viewer: "alice", OwnMirror: true, ContactID: &id, Avatar: "photos/9", Emoticons: []string{"[smile]"}})
	packet := New(nil).Encode(t.Context(), event, "http://localhost:8080")
	if body, _ := packet.Text(8); body != "alice: hello [smile]" {
		t.Fatal(body)
	}
	if sender, _ := packet.Text(2); sender != "bob" {
		t.Fatal(sender)
	}
	if content, _ := packet.Short(6); content != 6 {
		t.Fatal(content)
	}
	if _, ok := packet.Fields[10]; ok {
		t.Fatal("own mirror retained avatar")
	}
	if _, ok := packet.Fields[13]; ok {
		t.Fatal("own mirror retained color")
	}
	if source := event.Data.(events.Message).Message; source.Body != "hello [smile]" || *source.Color != color || source.Sender != "alice" {
		t.Fatalf("codec changed stored message: %+v", source)
	}
	ordinary := New(nil).Encode(t.Context(), events.New(0, events.Message{Message: message, Viewer: "bob", Avatar: "photos/9"}), "")
	if body, _ := ordinary.Text(8); body != message.Body {
		t.Fatal(body)
	}
	if rgb, _ := ordinary.Int(13); rgb != color {
		t.Fatal(rgb)
	}
}

func TestPhotoAliasAndActionRoutingMatchNativeContract(t *testing.T) {
	packet := New(nil).Encode(t.Context(), events.New(0, events.Message{Message: chat.Message{Kind: 1, Sender: "alice", Conversation: "bob", Body: "photo [smile]", PhotoID: 4}, Emoticons: []string{"[smile]"}}), "")
	if content, _ := packet.Short(6); content != 5 {
		t.Fatal(content)
	}
	if aliases, _ := packet.Text(11); aliases != "[photo:4] [smile]" {
		t.Fatal(aliases)
	}
	packet = New(nil).Encode(t.Context(), events.New(0, events.Message{Message: chat.Message{Kind: 1, Sender: "alice", Conversation: "bob", Body: "alice waves", Action: true}, Viewer: "alice"}), "")
	if sender, _ := packet.Text(2); sender != "bob" {
		t.Fatal(sender)
	}
	if body, _ := packet.Text(8); body != "alice waves" {
		t.Fatal(body)
	}
}

func TestAuthenticatedFramePreservesAdvertisedOriginAndFlags(t *testing.T) {
	packet := New(nil).Encode(t.Context(), events.New(8, events.Authenticated{Mobile: "+9607777777", Activated: true, RosterRevision: 5, Status: "Ready"}), "http://localhost:8080")
	if packet.Opcode != 203 || packet.Transaction != 8 {
		t.Fatal(packet)
	}
	if assets, _ := packet.Text(23); assets != "http://localhost:8080/assets/" {
		t.Fatal(assets)
	}
	if pages, _ := packet.Text(26); pages != "http://localhost:8080/sites/index.php" {
		t.Fatal(pages)
	}
	for _, tag := range []int16{33, 35, 37} {
		if flag, _ := packet.Int(tag); flag != 1 {
			t.Fatal(tag, flag)
		}
	}
	if enabled, _ := packet.Byte(2); enabled != 1 {
		t.Fatal(enabled)
	}
}
