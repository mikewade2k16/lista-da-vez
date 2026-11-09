package finance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxDocumentPDFPages = 10

var pdfPagesRE = regexp.MustCompile(`(?m)^Pages:\s+(\d+)\s*$`)
var pdfCopyDeniedRE = regexp.MustCompile(`(?mi)^Encrypted:\s+yes[^\n]*copy:no`)

// Poppler reads from stdin. Source PDF and rendered pages are never written to disk.
func extractPDFText(ctx context.Context, raw []byte, ocr DocumentOCR) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	info, err := documentCommand(ctx, raw, 32000, "pdfinfo", "-")
	if err != nil {
		return "", pdfCommandError(err)
	}
	if pdfCopyDeniedRE.Match(info) {
		return "", invalidFiscal("PDF não permite extração de texto. Envie uma cópia com permissão de leitura.")
	}
	match := pdfPagesRE.FindSubmatch(info)
	if len(match) != 2 {
		return "", invalidFiscal("PDF inválido ou sem páginas legíveis.")
	}
	pages, _ := strconv.Atoi(string(match[1]))
	if pages < 1 || pages > maxDocumentPDFPages {
		return "", invalidFiscal("O PDF deve ter até 10 páginas. Divida o extrato em arquivos menores.")
	}
	var result strings.Builder
	for page := 1; page <= pages; page++ {
		n := strconv.Itoa(page)
		rawText, e := documentCommand(ctx, raw, 32000, "pdftotext", "-f", n, "-l", n, "-layout", "-enc", "UTF-8", "-", "-")
		if e != nil {
			return "", pdfCommandError(e)
		}
		text := strings.TrimSpace(string(rawText))
		if strings.Contains(text, "R$") {
			bbox, bboxErr := documentCommand(ctx, raw, 256000, "pdftotext", "-f", n, "-l", n, "-bbox", "-enc", "UTF-8", "-", "-")
			if bboxErr != nil {
				return "", pdfCommandError(bboxErr)
			}
			text = normalizePDFCurrency(text, bbox)
		}
		// A page with a small header may still be a scanned statement: OCR the page.
		if len([]rune(text)) < 40 {
			rendered, e := documentCommand(ctx, raw, maxDocumentBytes, "pdftoppm", "-f", n, "-l", n, "-singlefile", "-scale-to", "2200", "-png", "-")
			if e != nil {
				return "", pdfCommandError(e)
			}
			text, e = ocr(ctx, rendered)
			if e != nil {
				return "", e
			}
		}
		if result.Len()+len(text) > 32000 {
			return "", invalidFiscal("PDF contém texto demais para uma leitura. Divida o extrato em arquivos menores.")
		}
		result.WriteString(text)
		result.WriteString("\n\n")
	}
	if len(strings.TrimSpace(result.String())) < 10 {
		return "", invalidFiscal("Não foi possível ler o PDF. Envie um documento nítido e sem cortes.")
	}
	return strings.TrimSpace(result.String()), nil
}
func documentCommand(ctx context.Context, raw []byte, limit int, program string, args ...string) ([]byte, error) {
	// program and arguments are supplied only by the fixed calls above, never by upload metadata.
	var cmd *exec.Cmd
	switch program {
	case "pdfinfo":
		cmd = exec.CommandContext(ctx, "pdfinfo", args...)
	case "pdftotext":
		cmd = exec.CommandContext(ctx, "pdftotext", args...)
	case "pdftoppm":
		cmd = exec.CommandContext(ctx, "pdftoppm", args...)
	default:
		return nil, ErrDocumentOCR
	}
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Stderr = io.Discard
	out := &boundedOCRBuffer{limit: limit}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func pdfCommandError(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return invalidFiscal("PDF inválido, protegido ou excedendo os limites de leitura. Envie uma cópia desbloqueada e legível.")
	}
	return ErrDocumentOCR
}
