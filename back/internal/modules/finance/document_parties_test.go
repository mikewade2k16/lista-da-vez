package finance

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDebitReceiptEstablishmentAndCardholder(t *testing.T) {
	text := `Comprovante de compra no débito
01 NOV 2026 08:57
Valor R$ 18,00
Dados da transação
Data e hora 01 NOV 2026 08:57
Estabelecimento MERCEARIA QA
NSU 845008
Dados do titular do cartão
Nome Pessoa QA da Silva
CPF ***.847.495-**
Número do cartão 5502.******.0545
Nu Pagamentos S.A. - Instituição de Pagamento
CNPJ 18.236.120/0001-58
ID da transação: TESTE`
	f := parseDocumentText(text)
	if f.PayerName != "Pessoa QA da Silva" || f.PayeeName != "MERCEARIA QA" {
		t.Fatal("debit card labels not recognized")
	}
	if f.Amount == nil || *f.Amount != 18 || f.PaymentDate != "2026-11-01" {
		t.Fatal("amount/date regression")
	}
	if f.CustomerName != "" || f.CustomerDocument != "" {
		t.Fatal("masked/bank document or fiscal party inferred")
	}
}
func TestPartyLabelsInlineAndSeparate(t *testing.T) {
	for _, text := range []string{
		"Comprovante Pix\nPAGADOR: Cliente QA\nCPF/CNPJ ***\nBENEFICIÁRIO: Empresa QA\nCNPJ ***",
		"Comprovante de compra\nEstabelecimento\nEmpresa QA\nNSU 123\nDados do titular do cartao\nNome\nCliente QA\nCPF ***",
		"Comprovante\nOrigem\nCliente QA\nNome\nAgência 123\nDestino\nNome Empresa QA\nConta 456",
	} {
		f := parseDocumentText(text)
		if f.PayerName != "Cliente QA" || f.PayeeName != "Empresa QA" {
			t.Fatal("party layout regression")
		}
	}
	f := parseDocumentText("Comprovante\nNome Não atribuir\nCPF ***\nNu Pagamentos S.A.\nCNPJ 18.236.120/0001-58")
	if f.PayerName != "" || f.PayeeName != "" {
		t.Fatal("unscoped name assigned")
	}
}
func TestRealDebitReceiptOCR(t *testing.T) {
	path := os.Getenv("FINANCE_OCR_TEST_DEBIT_IMAGE")
	if path == "" {
		t.Skip("private debit receipt not provided")
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
	if f.PayerName == "" || !strings.Contains(f.PayeeName, "ITABAIANA") || f.Amount == nil || *f.Amount != 18 || f.PaymentDate != "2026-11-01" {
		t.Fatal("real debit receipt did not extract expected fields")
	}
	if f.CustomerDocument != "" || strings.Contains(f.PayerName, "847") || strings.Contains(f.PayerName, "CNPJ") {
		t.Fatal("bank/card data mixed into identity")
	}
}

func TestPartyRecoveryPreservesManualAndFiscalData(t *testing.T) {
	original := DocumentFields{PayerName: "Nome corrigido", CustomerName: "Tomador confirmado", CustomerDocument: "12345678901", Service: FiscalTemplateFields{ServiceDescription: "Serviço confirmado"}}
	fields, changed := recoverDocumentParties(original, "Comprovante\nOrigem Nome OCR\nAgencia 1\nDestino Nome Recebedor QA\nConta 2")
	if !changed || fields.PayerName != "Nome corrigido" || fields.PayeeName != "Nome Recebedor QA" || fields.CustomerName != original.CustomerName || fields.CustomerDocument != original.CustomerDocument || fields.Service.ServiceDescription != original.Service.ServiceDescription {
		t.Fatal("recovery replaced manual/fiscal fields")
	}
	if _, changed = recoverDocumentParties(fields, "Comprovante\nDestino Outro nome"); changed {
		t.Fatal("recovery overwrote filled name")
	}
}

func TestPartyDocumentsBoundToReceiptSections(t *testing.T) {
	for _, text := range []string{
		"Comprovante Pix\nDestino\nNome Empresa QA\nCNPJ 13017462000163\nInstituição Banco QA\nTipo de conta Corrente\nOrigem\nNome Pessoa QA\nInstituição NU PAGAMENTOS - IP\nCPF ***.847.495-**\nInformações adicionais\nNu Pagamentos S.A.\nCNPJ 18.236.120/0001-58",
		"Comprovante Pix\nDestino\nNome Empresa QA\nCNPJ\n13.017.462/0001-63\nOrigem\nNome Pessoa QA\nCPF\n***.847.495-**\nNu Pagamentos S.A.\nCNPJ 18.236.120/0001-58",
	} {
		f := parseDocumentText(text)
		if f.PayeeDocument != "13017462000163" || f.PayerDocument != "" || f.CustomerDocument != "" {
			t.Fatal("party document discarded, inferred, or mixed with bank footer")
		}
		recovered, changed := recoverDocumentParties(DocumentFields{PayeeName: "Manual", PayeeDocument: "11111111111111"}, text)
		if !changed || recovered.PayeeName != "Manual" || recovered.PayeeDocument != "11111111111111" {
			t.Fatal("manual document overwritten")
		}
	}
}
func TestRealPaymentReceiptOCR(t *testing.T) {
	path := os.Getenv("FINANCE_OCR_TEST_PAYMENT_IMAGE")
	if path == "" {
		t.Skip("private payment receipt not provided")
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
	if f.PayeeName != "ENERGISA SE" || f.PayerName == "" || f.PayeeDocument != "13017462000163" || f.PayerDocument != "" || f.Amount == nil || *f.Amount != 98.15 || f.PaymentDate != "2026-10-17" {
		t.Fatal("payment receipt fields not extracted correctly")
	}
	if f.CustomerName != "" || f.CompetenceDate != "" {
		t.Fatal("fiscal roles inferred")
	}
}
