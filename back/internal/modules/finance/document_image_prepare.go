package finance

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
)

// Enlarge small text in memory with bounded bilinear interpolation.
func prepareDocumentOCRImage(raw []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width >= 600 {
		return raw, err
	}
	factor := min(4, (800+cfg.Width-1)/cfg.Width)
	for factor > 1 && (cfg.Height*factor > 10000 || int64(cfg.Width)*int64(cfg.Height)*int64(factor*factor) > 24000000) {
		factor--
	}
	if factor <= 1 {
		return raw, nil
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	gray := image.NewGray(image.Rect(0, 0, cfg.Width, cfg.Height))
	draw.Draw(gray, gray.Bounds(), src, src.Bounds().Min, draw.Src)
	out := image.NewGray(image.Rect(0, 0, cfg.Width*factor, cfg.Height*factor))
	for y := 0; y < out.Bounds().Dy(); y++ {
		sy, dy := y/factor, y%factor
		ny := min(sy+1, cfg.Height-1)
		for x := 0; x < out.Bounds().Dx(); x++ {
			sx, dx := x/factor, x%factor
			nx := min(sx+1, cfg.Width-1)
			top := int(gray.Pix[sy*gray.Stride+sx])*(factor-dx) + int(gray.Pix[sy*gray.Stride+nx])*dx
			bottom := int(gray.Pix[ny*gray.Stride+sx])*(factor-dx) + int(gray.Pix[ny*gray.Stride+nx])*dx
			out.Pix[y*out.Stride+x] = uint8((top*(factor-dy) + bottom*dy) / (factor * factor))
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, out); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
