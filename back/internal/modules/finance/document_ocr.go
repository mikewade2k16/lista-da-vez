package finance

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os/exec"
	"strings"
	"time"
)

var ErrDocumentOCR = errors.New("finance: document OCR unavailable")

const maxDocumentBytes = 8 << 20

type DocumentOCR func(context.Context, []byte) (string, error)

// RunDocumentOCR never invokes a shell or persists the source image.
func RunDocumentOCR(ctx context.Context, raw []byte) (string, error) {
	if err := validateDocumentImage(raw); err != nil {
		return "", err
	}
	prepared, err := prepareDocumentOCRImage(raw)
	if err != nil {
		return "", ErrDocumentOCR
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tesseract", "stdin", "stdout", "-l", "por+eng", "--psm", "4")
	cmd.Stdin = bytes.NewReader(prepared)
	cmd.Stderr = io.Discard
	output := &boundedOCRBuffer{}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		return "", ErrDocumentOCR
	}
	text := strings.TrimSpace(output.String())
	if len(text) < 10 {
		return "", invalidFiscal("Não foi possível ler texto suficiente. Envie uma imagem nítida e sem cortes.")
	}
	return text, nil
}
func validateDocumentImage(raw []byte) error {
	if len(raw) == 0 || len(raw) > maxDocumentBytes {
		return invalidFiscal("Envie uma imagem JPEG ou PNG com até 8 MB.")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 20 || cfg.Height < 20 || cfg.Width > 10000 || cfg.Height > 10000 || int64(cfg.Width)*int64(cfg.Height) > 24000000 {
		return invalidFiscal("Imagem inválida. Use JPEG/PNG de até 24 megapixels.")
	}
	return nil
}

type boundedOCRBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedOCRBuffer) Write(p []byte) (int, error) {
	limit := b.limit
	if limit == 0 {
		limit = 32000
	}
	if b.Len()+len(p) > limit {
		return 0, errors.New("OCR output limit")
	}
	return b.Buffer.Write(p)
}
