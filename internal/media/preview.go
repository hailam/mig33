package media

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"image/png"

	"github.com/hailam/mig33/internal/faults"
)

// Preview keeps the original nearest-neighbor sizing and PNG encoding used by
// native downloads and the legacy photo URL. It never enlarges a source image.
func Preview(data []byte, max int) ([]byte, error) {
	if max < 1 || max > 240 {
		return nil, faults.ErrInvalid
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := source.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return nil, faults.ErrInvalid
	}
	if w > max || h > max {
		if w >= h {
			h = h * max / w
			w = max
		} else {
			w = w * max / h
			h = max
		}
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, source.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
		}
	}
	var result bytes.Buffer
	err = png.Encode(&result, dst)
	return result.Bytes(), err
}
