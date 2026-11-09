package finance

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestReceiptExtractionDoesNotInventFiscalParties(t *testing.T) {
	text := `Comprovante de transferência
08 OUT 2026 - 21:08:43
Valor R$ 22.000,00
Tipo de transferência Pix
Destino
Nome 64.624.616 PESSOA QA
DO EXEMPLO
Agência 0001
Conta 1234
Origem
Nome EMPRESA QA LTDA
Agência 0001
Conta 5678
Nu Pagamentos S.A.
CNPJ 18.236.120/0001-58`
	fields := parseDocumentText(text)
	if fields.Amount == nil || *fields.Amount != 22000 || fields.PaymentDate != "2026-10-08" {
		t.Fatalf("value/date incorrect")
	}
	if fields.PayeeName != "64.624.616 PESSOA QA DO EXEMPLO" || fields.PayerName != "EMPRESA QA LTDA" {
		t.Fatal("parties incorrectly extracted")
	}
	if fields.CustomerName != "" || fields.CustomerDocument != "" || fields.CompetenceDate != "" {
		t.Fatal("invented fiscal attribution or used bank CNPJ")
	}
	view := documentView(DocumentDraft{Fields: fields})
	if len(view.MissingFields) != 4 || len(view.Warnings) != 3 {
		t.Fatal("missing fiscal review warnings")
	}
}
func TestReceiptAmbiguityAndInvalidImages(t *testing.T) {
	f := parseDocumentText("Valor R$ 100,00 total R$ 200,00 data 32/13/2026")
	if f.Amount != nil || f.PaymentDate != "" {
		t.Fatal("ambiguous value/date silently accepted")
	}
	for _, raw := range [][]byte{nil, []byte("<svg>not a PNG</svg>"), make([]byte, maxDocumentBytes+1)} {
		if validateDocumentImage(raw) == nil {
			t.Fatal("invalid image accepted")
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	if err := validateDocumentImage(out.Bytes()); err != nil {
		t.Fatal(err)
	}
}

// Opt-in real OCR test: the private fixture stays outside Git and is never printed.
func TestDocumentRealImageOCR(t *testing.T) {
	path := os.Getenv("FINANCE_OCR_TEST_IMAGE")
	if path == "" {
		t.Skip("private image not provided")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text, err := RunDocumentOCR(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	f := parseDocumentText(text)
	if f.Amount == nil || *f.Amount != 22000 || f.PaymentDate != "2026-10-08" || f.PayeeName == "" || f.PayerName == "" {
		t.Fatal("real receipt fields did not match expected amount/date/parties")
	}
	if f.CustomerName != "" || f.CustomerDocument != "" || f.CompetenceDate != "" {
		t.Fatal("fiscal data inferred from receipt")
	}
}

func TestLabeledAmountWithOCRCurrency(t *testing.T) {
	for _, text := range []string{"Comprovante\nValor AS 57,58\nValor original AS 57,58", "Comprovante\nValor\n57,58"} {
		f := parseDocumentText(text)
		if f.Amount == nil || *f.Amount != 57.58 {
			t.Fatal("labeled amount lost")
		}
	}
	if f := parseDocumentText("Valor AS 57,58\nValor original R$ 58,00"); f.Amount != nil {
		t.Fatal("ambiguous amount accepted")
	}
}
func TestRealSmallReceiptOCR(t *testing.T) {
	path := os.Getenv("FINANCE_OCR_TEST_SMALL_IMAGE")
	if path == "" {
		t.Skip("private small receipt not provided")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text, err := RunDocumentOCR(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	f := parseDocumentText(text)
	if f.Amount == nil || *f.Amount != 57.58 || f.PaymentDate != "2026-10-15" || f.PayeeName != "HOSTGATOR" || f.PayerName == "" || f.PayeeDocument != "15754475000140" {
		t.Fatal("small receipt missing expected payment data")
	}
	if f.CustomerName != "" || f.CustomerDocument != "" || f.CompetenceDate != "" {
		t.Fatal("fiscal roles inferred")
	}
}
