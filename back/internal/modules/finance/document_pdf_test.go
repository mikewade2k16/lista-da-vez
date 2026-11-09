package finance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// A small in-memory PDF avoids checking personal statements into the repository.
func testDocumentPDF(content string, pages int) []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	kids := []string{}
	for range pages {
		pageID := len(objects) + 1
		streamID := pageID + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", streamID), fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), pages)
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return out.Bytes()
}
func TestPDFTextAndScannedFallback(t *testing.T) {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("Poppler not installed")
	}
	ctx := context.Background()
	text, err := extractPDFText(ctx, testDocumentPDF("BT /F1 12 Tf 30 700 Td (Payment receipt QA - Valor R$ 125,50 - 08/10/2026) Tj ET", 1), func(context.Context, []byte) (string, error) { t.Fatal("text PDF unnecessarily OCRed"); return "", nil })
	if err != nil || !strings.Contains(text, "125,50") {
		t.Fatal("text PDF extraction failed", err)
	}
	f := parseDocumentText(text)
	if f.Amount == nil || *f.Amount != 125.5 || f.PaymentDate != "2026-10-08" {
		t.Fatal("text PDF fields failed")
	}
	calls := 0
	text, err = extractPDFText(ctx, testDocumentPDF("", 2), func(_ context.Context, png []byte) (string, error) {
		calls++
		if validateDocumentImage(png) != nil {
			t.Fatal("PDF rasterization invalid")
		}
		return "Scanned page QA document", nil
	})
	if err != nil || calls != 2 || strings.Count(text, "Scanned page") != 2 {
		t.Fatal("scan fallback skipped a page", err)
	}
	_, err = extractPDFText(ctx, testDocumentPDF("", 11), nil)
	if !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("oversized PDF accepted")
	}
	_, err = extractPDFText(ctx, []byte("%PDF-1.4 corrupt"), nil)
	if !errors.Is(err, ErrFiscalInvalid) {
		t.Fatal("invalid PDF accepted")
	}
}
func TestCommonDocumentFormats(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		kind string
	}{
		{"statement.pdf", []byte("%PDF-1.7 test"), "pdf"},
		{"statement.csv", []byte("Data;Valor;Descricao\n08/10/2026;100,00;Servico"), "text"},
		{"statement.txt", []byte("Valor R$ 100,00\n08/10/2026"), "text"},
		{"statement.ofx", []byte("OFXHEADER:100\n<OFX><BANKMSGSRSV1></BANKMSGSRSV1></OFX>"), "text"},
	}
	for _, c := range cases {
		kind, err := documentFormat(c.raw, c.name)
		if err != nil || kind != c.kind {
			t.Fatalf("%s unsupported", c.name)
		}
	}
	for _, c := range []struct {
		name string
		raw  []byte
	}{{"binary.csv", []byte{0, 1, 2, 3}}, {"fake.ofx", []byte("This is not OFX data")}, {"file.docx", []byte("zip binary")}, {"big.txt", bytes.Repeat([]byte("a"), 32001)}} {
		if _, err := documentFormat(c.raw, c.name); err == nil {
			t.Fatal("invalid document accepted", c.name)
		}
	}
	text, err := plainDocumentText([]byte("Descri\xe7\xe3o do pagamento: R$ 100,00"))
	if err != nil || !strings.Contains(text, "Descrição") {
		t.Fatal("Windows-1252 not decoded")
	}
}
func TestDocumentRealPDF(t *testing.T) {
	path := os.Getenv("FINANCE_OCR_TEST_PDF")
	if path == "" {
		t.Skip("private PDF not provided")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	kind, err := documentFormat(raw, "statement.pdf")
	if err != nil || kind != "pdf" {
		t.Fatal("PDF rejected")
	}
	text, err := extractPDFText(context.Background(), raw, RunDocumentOCR)
	if err != nil || len(text) < 40 {
		t.Fatal("real PDF extraction failed", err)
	}
	fields := parseDocumentText(text)
	if fields.Amount == nil || *fields.Amount != 12 || fields.PaymentDate != "2026-10-31" || fields.PayerName == "" || fields.PayeeName == "" {
		t.Fatal("real PDF amount/date/parties mismatch")
	}
	if fields.CustomerName != "" || fields.CompetenceDate != "" {
		t.Fatal("real PDF invented fiscal role")
	}
}

func TestPDFRaisedCentsAreNotMultipliedByHundred(t *testing.T) {
	bbox := []byte(`<html><word xMin="0" yMin="0" xMax="25" yMax="34">R$</word><word xMin="30" yMin="0" xMax="60" yMax="34">12</word><word xMin="60" yMin="2" xMax="80" yMax="21">00</word></html>`)
	text := normalizePDFCurrency("Comprovante de pagamento\n31/outubro/2026\nR$ 1200 BRL\nMP*LOJA QA\nTitular\nPessoa QA\nMeio de pagamento\nCartao", bbox)
	f := parseDocumentText(text)
	if f.Amount == nil || *f.Amount != 12 || f.PaymentDate != "2026-10-31" || f.PayerName != "Pessoa QA" || f.PayeeName != "MP*LOJA QA" {
		t.Fatal("raised cents or receipt fields incorrectly read")
	}
	if f.CustomerName != "" || f.CompetenceDate != "" {
		t.Fatal("fiscal attribution inferred")
	}
	flat := bytes.ReplaceAll(bbox, []byte(`yMax="21"`), []byte(`yMax="34"`))
	if normalizePDFCurrency("R$ 1200 BRL", flat) != "R$ 1200 BRL" {
		t.Fatal("decimal invented without superscript geometry")
	}
}
