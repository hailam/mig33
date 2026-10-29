package nativecodec

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"

	"github.com/hailam/mig33/internal/catalog"
)

func (c *Codec) spriteCell(ctx context.Context, index int) ([]byte, error) {
	if index < 0 || index >= 80 {
		return nil, fmt.Errorf("invalid sprite")
	}

	c.spritesMu.Lock()
	defer c.spritesMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.sprites == nil {
		data, err := c.readResource(ctx, "sprites.png")
		if err != nil {
			return nil, err
		}

		atlas, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if atlas.Bounds().Dx() != 240 || atlas.Bounds().Dy() != 48 {
			return nil, fmt.Errorf("sprite atlas must be 240 by 48 pixels")
		}

		images := make([][]byte, 80)
		for i := range images {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			img := image.NewNRGBA(image.Rect(0, 0, 12, 12))
			draw.Draw(img, img.Bounds(), atlas, image.Pt(i%20*12, i/20*12), draw.Src)
			var b bytes.Buffer
			if err = png.Encode(&b, img); err != nil {
				return nil, err
			}

			images[i] = b.Bytes()
		}

		c.sprites = images
	}

	return c.sprites[index], nil
}

// Local short PCM chime; this is replacement content, not recovered audio.
func localChime() []byte {
	const samples = 1600
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+samples*2))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(8000))
	binary.Write(&b, binary.LittleEndian, uint32(16000))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(samples*2))
	for i := 0; i < samples; i++ {
		v := int16(math.Sin(float64(i)*2*math.Pi*660/8000) * 5000 * (1 - float64(i)/samples))
		binary.Write(&b, binary.LittleEndian, v)
	}

	return b.Bytes()
}

func (c *Codec) emoticonContent(ctx context.Context, e catalog.EmoticonDefinition) ([]byte, error) {
	var b bytes.Buffer
	if e.Type == 3 {
		binary.Write(&b, binary.BigEndian, uint32(len(e.Cells)))
	}

	for _, cell := range e.Cells {
		data, err := c.spriteCell(ctx, cell)
		if err != nil {
			return nil, err
		}
		if e.Type == 3 || e.Type == 4 {
			binary.Write(&b, binary.BigEndian, uint32(len(data)))
		}

		b.Write(data)
	}
	if e.Type == 4 {
		audio := localChime()
		binary.Write(&b, binary.BigEndian, uint32(len(audio)))
		b.Write(audio)
	}

	return b.Bytes(), nil
}
