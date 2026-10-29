// Package testassets builds synthetic client resource fixtures for Go tests.
// It contains no recovered applet bytes and must not supply production assets.
package testassets

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing/fstest"

	"github.com/hailam/mig33/internal/protocol"
)

// FS returns an independent filesystem with synthetic native packets and PNGs.
// Only fields exercised by the backend are populated; this is not a usable
// replacement for the original client's complete theme or language resources.
func FS() fstest.MapFS {
	atlas := fixturePNG(240, 48)
	avatar := fixturePNG(24, 24)
	theme := protocol.New(1000, 0).
		SetText(0, "synthetic_theme").SetInt(1, 1).SetText(2, "Synthetic test theme").
		SetBytes(77, []byte("opaque fixture field")).SetBytes(129, atlas)
	for _, tag := range []int16{139, 142, 143, 144, 145} {
		theme.SetInt(tag, int32(tag)*100)
	}
	language := protocol.New(1004, 0).
		SetText(1, "en-US").SetText(2, "Synthetic English fixture").
		SetText(193, "mig33-test").SetText(361, "synthetic-v1")
	return fstest.MapFS{
		"default_v2":         {Data: packetBytes(theme)},
		"en-US":              {Data: packetBytes(language)},
		"default-avatar.png": {Data: avatar},
		"sprites.png":        {Data: atlas},
	}
}

// Bytes returns a fresh synthetic fixture. Unknown names indicate a test bug.
func Bytes(name string) []byte {
	file, ok := FS()[name]
	if !ok {
		panic(fmt.Sprintf("unknown test asset %q", name))
	}
	return file.Data
}

func packetBytes(packet *protocol.Packet) []byte {
	var out bytes.Buffer
	if err := protocol.Encode(&out, packet); err != nil {
		panic(err)
	}
	return out.Bytes()
}

func fixturePNG(width, height int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			cell := x/12 + y/12*20
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(cell * 3), G: uint8(x), B: uint8(y * 5), A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		panic(err)
	}
	return out.Bytes()
}
