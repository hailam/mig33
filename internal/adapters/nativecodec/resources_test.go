package nativecodec

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/testassets"
)

func TestMissingResourcesOnlyDisableDependentFeatures(t *testing.T) {
	codec := New(nil)
	for _, event := range []*events.Event{
		events.New(47, events.Theme{ID: "default"}),
		events.New(47, events.EmoticonResource{Token: "[smile]"}),
		events.New(47, events.ProviderIcons{ID: 2}),
	} {
		packet := codec.Encode(t.Context(), event, "")
		if packet.Opcode != 0 || packet.Transaction != 47 {
			t.Fatalf("missing resource response for %T: %+v", event.Data, packet)
		}
	}
	_, err := codec.LanguageResource(t.Context(), "", "en-US")
	var serviceError *faults.ServiceError
	if !errors.As(err, &serviceError) || serviceError.Kind != faults.Unavailable || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing language: %v", err)
	}
	for _, tc := range []struct {
		event  *events.Event
		opcode int16
	}{
		{events.New(3, events.Authenticated{}), 203},
		{events.New(4, events.Message{Message: chat.Message{Kind: 1, Sender: "alice", Conversation: "bob", Body: "hello"}}), 500},
	} {
		if packet := codec.Encode(t.Context(), tc.event, ""); packet.Opcode != tc.opcode {
			t.Fatalf("core response depends on local assets: %+v", packet)
		}
	}
}

func TestCancelledResourceRequestsDoNotPoisonSpriteCache(t *testing.T) {
	codec := New(testassets.FS())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, event := range []*events.Event{
		events.New(12, events.Theme{ID: "default"}),
		events.New(12, events.EmoticonResource{Token: "[smile]"}),
		events.New(12, events.ProviderIcons{ID: 2}),
	} {
		if packet := codec.Encode(ctx, event, ""); packet.Opcode != 0 {
			t.Fatalf("cancelled resource encoded for %T", event.Data)
		}
	}
	if _, err := codec.LanguageResource(ctx, "", "en-US"); !errors.Is(err, context.Canceled) {
		t.Fatalf("language cancellation: %v", err)
	}
	if _, err := codec.spriteCell(t.Context(), 0); err != nil {
		t.Fatalf("cancelled request poisoned cache: %v", err)
	}
	if _, err := codec.spriteCell(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached sprite ignored cancellation: %v", err)
	}
}

type cancellingFS struct {
	fs.FS
	cancel context.CancelFunc
}

func (f cancellingFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	f.cancel()
	return file, err
}

func TestCancellationDuringReadIsObserved(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	codec := New(cancellingFS{FS: testassets.FS(), cancel: cancel})
	if _, err := codec.spriteCell(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("read cancellation: %v", err)
	}
	if codec.sprites != nil {
		t.Fatal("cancelled read populated cache")
	}
	if _, err := codec.spriteCell(t.Context(), 0); err != nil {
		t.Fatalf("later read failed: %v", err)
	}
}

func TestSpriteCachesAreConcurrentAndIsolatedPerCodec(t *testing.T) {
	first := New(testassets.FS())
	secondFS := testassets.FS()
	img := image.NewNRGBA(image.Rect(0, 0, 240, 48))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.NRGBA{R: 255, A: 255}), image.Point{}, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	secondFS["sprites.png"] = &fstest.MapFile{Data: out.Bytes()}
	second := New(secondFS)
	var group sync.WaitGroup
	for range 8 {
		for _, codec := range []*Codec{first, second} {
			group.Go(func() {
				if _, err := codec.spriteCell(t.Context(), 0); err != nil {
					t.Errorf("concurrent sprite read: %v", err)
				}
			})
		}
	}
	group.Wait()
	a, err := first.spriteCell(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.spriteCell(t.Context(), 0)
	if err != nil || bytes.Equal(a, b) {
		t.Fatalf("codec resource caches crossed instances: %v", err)
	}
}

func TestMalformedThemeCannotEncodeUnrelatedOpcode(t *testing.T) {
	var data bytes.Buffer
	if err := protocol.Encode(&data, protocol.New(203, 0)); err != nil {
		t.Fatal(err)
	}
	codec := New(fstest.MapFS{"default_v2": {Data: data.Bytes()}})
	packet := codec.Encode(t.Context(), events.New(5, events.Theme{ID: "default"}), "")
	if packet.Opcode != 0 || packet.Transaction != 5 {
		t.Fatalf("invalid theme escaped validation: %+v", packet)
	}
}
