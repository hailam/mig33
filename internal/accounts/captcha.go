package accounts

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// CaptchaImage is the legible local development-code image expected by the client.
func CaptchaImage() []byte {
	glyphs := []string{
		"00100011000010000100001000010001110",
		"01110100010000100010001000100011111",
		"11110000010000101110000010000111110",
		"00010001100101010010111110001000010",
		"11111100001000011110000010000111110",
		"01110100001000011110100011000101110",
	}
	im := image.NewRGBA(image.Rect(0, 0, 100, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 100; x++ {
			im.Set(x, y, color.White)
		}
	}
	for d, g := range glyphs {
		for i, c := range g {
			if c == '1' {
				for sy := 0; sy < 2; sy++ {
					for sx := 0; sx < 2; sx++ {
						im.Set(7+d*15+(i%5)*2+sx, 5+(i/5)*2+sy, color.Black)
					}
				}
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}
