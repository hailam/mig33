package nativecodec

import (
	"bytes"
	"testing"
	"time"

	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/testassets"
)

func TestServiceDecodeRetainsMissingAndMalformedFields(t *testing.T) {
	p := protocol.New(504, 17).SetBytes(2, []byte{1, 2, 3}).SetBytes(5, nil)
	request, ok := DecodeServices(p)
	if !ok || request.Correlation != 17 {
		t.Fatalf("decode: %+v %v", request, ok)
	}
	input := request.Input.(commands.UploadPhoto)
	if !input.Purpose.Present || input.Purpose.Valid {
		t.Fatalf("malformed field lost: %+v", input.Purpose)
	}
	p.Fields[2][0] = 9
	if input.Data[0] != 1 {
		t.Fatal("decoded command aliases mutable frame")
	}
	request, _ = DecodeServices(protocol.New(504, 18))
	if request.Input.(commands.UploadPhoto).Purpose.Present {
		t.Fatal("missing field became present")
	}
}

func TestThemeVariantsRetainUnmodifiedResourceFields(t *testing.T) {
	original, err := protocol.Decode(bytes.NewReader(testassets.Bytes("default_v2")))
	if err != nil {
		t.Fatal(err)
	}
	changed := map[int16]bool{0: true, 2: true, 139: true, 142: true, 143: true, 144: true, 145: true}
	for _, id := range []string{"default", "theme-blue", "theme-orange"} {
		packet, ok := New(testassets.FS()).EncodeServices(t.Context(), events.New(91, events.Theme{ID: id}), "http://localhost:8080")
		if !ok || packet.Opcode != 1000 || packet.Transaction != 91 || len(packet.Fields) != len(original.Fields) {
			t.Fatalf("incomplete theme %s", id)
		}
		for tag, field := range original.Fields {
			if id != "default" && changed[tag] {
				continue
			}
			if !bytes.Equal(packet.Fields[tag], field) {
				t.Fatalf("theme %s changed preserved field %d", id, tag)
			}
		}
	}
}

func TestProviderAndAssetWireContracts(t *testing.T) {
	packet, _ := New(testassets.FS()).EncodeServices(t.Context(), events.New(0, events.Provider{ID: 2, Channel: 4, Name: "MSN"}), "http://localhost:8080")
	if packet.Opcode != 208 {
		t.Fatal(packet.Opcode)
	}
	if text, _ := packet.Text(2); text != "MSN saved contacts (local)" {
		t.Fatal(text)
	}
	if state, _ := packet.Byte(4); state != 2 {
		t.Fatal(state)
	}
	icons, _ := New(testassets.FS()).EncodeServices(t.Context(), events.New(15, events.ProviderIcons{ID: 2}), "")
	if icons.Opcode != 927 || icons.Transaction != 15 {
		t.Fatal(icons)
	}
	for _, tag := range []int16{2, 4, 5, 6} {
		if !bytes.Equal(icons.Fields[tag], testassets.Bytes("default-avatar.png")) {
			t.Fatal(tag)
		}
	}
	resource, _ := New(testassets.FS()).EncodeServices(t.Context(), events.New(7, events.EmoticonResource{Token: "[smile]"}), "")
	if resource.Opcode != 914 || len(resource.Fields[4]) == 0 {
		t.Fatal("emoticon encoding missing")
	}
	failed, _ := New(testassets.FS()).EncodeServices(t.Context(), events.New(7, events.EmoticonResource{Token: "unknown"}), "")
	if failed.Opcode != 0 {
		t.Fatal("resource failure must use native error opcode")
	}
}

func TestActivityAgeClampAndPreservedURL(t *testing.T) {
	packet, _ := New(testassets.FS()).EncodeServices(t.Context(), events.New(0, events.Activity{Actor: "alice", Display: "posted an update", Text: "Read update", Timestamp: time.Now().Add(-30 * 24 * time.Hour).UnixMilli(), ImageID: 8}), "http://localhost:8080")
	if packet.Opcode != 422 {
		t.Fatal(packet.Opcode)
	}
	if age, _ := packet.Int(1); age != 2147483 {
		t.Fatal(age)
	}
	if avatar, _ := packet.Text(3); avatar != "http://localhost:8080/assets/photos/8" {
		t.Fatal(avatar)
	}
	if body, _ := packet.Text(4); !bytes.Contains([]byte(body), []byte("username=alice")) {
		t.Fatal(body)
	}
}

func TestLanguageCatalogUsesOriginalPath(t *testing.T) {
	resource, err := New(testassets.FS()).LanguageResource(t.Context(), "http://localhost:8080", "l")
	if err != nil {
		t.Fatal(err)
	}
	packet, err := protocol.Decode(bytes.NewReader(resource.Data))
	if err != nil || packet.Opcode != 1002 {
		t.Fatalf("language: %+v %v", packet, err)
	}
	if target, _ := packet.Text(3); target != "http://localhost:8080/sites/resources/language/v4.6/en-US" {
		t.Fatal(target)
	}
	language, err := New(testassets.FS()).LanguageResource(t.Context(), "http://localhost:8080", "en-US")
	if err != nil || !bytes.Equal(language.Data, testassets.Bytes("en-US")) {
		t.Fatal("injected language changed")
	}
}
