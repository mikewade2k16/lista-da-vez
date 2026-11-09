package finance

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ExtractKnowledgePDF reuses the bounded Poppler pipeline. Scanned sources require
// an explicit text transcription: no LLM/OCR provider is invoked by knowledge.
func ExtractKnowledgePDF(ctx context.Context, content []byte) (string, error) {
	return extractPDFText(ctx, content, func(context.Context, []byte) (string, error) { return "", errors.New("manual transcription required") })
}

// Limites da coleta de documentos oficiais longos (guia web NFS-e ~130 páginas,
// Perguntão do Simples). Sem OCR: página sem texto vira página vazia e o coletor
// recusa o documento se a maioria estiver vazia, em vez de publicar parcial.
const (
	maxPublicPDFPages     = 400
	maxPublicPDFTextBytes = 4 << 20
)

// ExtractPublicPDF devolve o texto de cada página (pdftotext separa páginas com \f).
func ExtractPublicPDF(ctx context.Context, content []byte) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	info, err := documentCommand(ctx, content, 32000, "pdfinfo", "-")
	if err != nil {
		return nil, pdfCommandError(err)
	}
	if pdfCopyDeniedRE.Match(info) {
		return nil, invalidFiscal("PDF não permite extração de texto.")
	}
	match := pdfPagesRE.FindSubmatch(info)
	if len(match) != 2 {
		return nil, invalidFiscal("PDF inválido ou sem páginas legíveis.")
	}
	pages, _ := strconv.Atoi(string(match[1]))
	if pages < 1 || pages > maxPublicPDFPages {
		return nil, invalidFiscal("PDF acima do limite de páginas da coleta.")
	}
	raw, err := documentCommand(ctx, content, maxPublicPDFTextBytes, "pdftotext", "-enc", "UTF-8", "-", "-")
	if err != nil {
		return nil, pdfCommandError(err)
	}
	out := strings.Split(strings.TrimRight(string(raw), "\f\n"), "\f")
	if len(out) > pages {
		out = out[:pages]
	}
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out, nil
}
