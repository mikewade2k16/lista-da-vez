package finance

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

func documentFormat(raw []byte, name string) (string, error) {
	if len(raw) == 0 || len(raw) > maxDocumentBytes {
		return "", invalidFiscal("Envie PDF, JPEG, PNG, TXT, CSV ou OFX de até 8 MB.")
	}
	if bytes.HasPrefix(raw, []byte("%PDF-")) {
		return "pdf", nil
	}
	if bytes.HasPrefix(raw, []byte{0xff, 0xd8, 0xff}) || bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
		if err := validateDocumentImage(raw); err != nil {
			return "", err
		}
		return "image", nil
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt", ".csv", ".ofx":
		if _, err := plainDocumentText(raw); err != nil {
			return "", err
		}
		if strings.EqualFold(filepath.Ext(name), ".ofx") && !bytes.Contains(bytes.ToUpper(raw), []byte("<OFX>")) {
			return "", invalidFiscal("Arquivo OFX inválido.")
		}
		return "text", nil
	default:
		return "", invalidFiscal("Formato não aceito. Use PDF, JPEG, PNG, TXT, CSV ou OFX.")
	}
}
func plainDocumentText(raw []byte) (string, error) {
	if bytes.ContainsRune(raw, 0) || bytes.HasPrefix(raw, []byte("MZ")) {
		return "", invalidFiscal("O arquivo deve conter texto, não conteúdo binário.")
	}
	if !utf8.Valid(raw) {
		var err error
		raw, err = charmap.Windows1252.NewDecoder().Bytes(raw)
		if err != nil {
			return "", invalidFiscal("Não foi possível ler a codificação do texto.")
		}
	}
	for _, r := range string(raw) {
		if r < 32 && r != '\r' && r != '\n' && r != '\t' && r != '\f' {
			return "", invalidFiscal("O arquivo contém caracteres binários inválidos.")
		}
	}
	text := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
	if len(text) < 10 || len(text) > 32000 {
		return "", invalidFiscal("O texto deve ter entre 10 e 32.000 bytes. Divida extratos longos em arquivos menores.")
	}
	return text, nil
}
func extractDocumentText(ctx context.Context, raw []byte, format string, ocr DocumentOCR) (string, error) {
	switch format {
	case "pdf":
		return extractPDFText(ctx, raw, ocr)
	case "text":
		return plainDocumentText(raw)
	default:
		return ocr(ctx, raw)
	}
}
