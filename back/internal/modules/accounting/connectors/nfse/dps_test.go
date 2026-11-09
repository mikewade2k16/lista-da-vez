package nfse

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Synthetic values only: codes are XSD-valid placeholders, not a fiscal scenario.
func syntheticDPSInput() DPSInput {
	return DPSInput{
		Environment: "2", DPSID: "DPS" + strings.Repeat("1", 42), IssuedAt: time.Date(2026, 11, 6, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		AppVersion: "omni-qa", Series: "1", Number: "17", CompetenceDate: "2026-11-06", EmitterType: "1", IssueMunicipality: "2800308",
		Provider: DPSParty{CNPJ: "12345678000195"}, SimplesOption: "1", SpecialRegime: "0",
		Customer:            &DPSParty{CPF: "12345678909", Name: "Cliente Sintético QA"},
		ServiceMunicipality: "2800308", NationalTaxCode: "010101", Description: "Serviço sintético de teste",
		Amount: "1500.00", ISSQNTaxation: "1", ISSQNRetention: "1", TotalTaxIndicator: "0",
	}
}

func TestDPSPendingNeverInfersFiscalCodes(t *testing.T) {
	raw, pending, err := BuildDPS(DPSInput{Amount: "100.00", Description: "Serviço"})
	if err != nil || raw != nil || len(pending) < 10 {
		t.Fatal("empty fiscal codes produced XML", pending, err)
	}
	in := syntheticDPSInput()
	in.Description = "Serviço — com travessão"
	in.Amount = "1.500,00"
	in.TotalTaxIndicator, in.SimplesTotalPercent = "0", "6.00"
	_, pending, _ = BuildDPS(in)
	joined := strings.Join(pending, ";")
	for _, want := range []string{"Descrição", "Valor", "tributos totais"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing pending %q in %v", want, pending)
		}
	}
	first, pending, err := BuildDPS(syntheticDPSInput())
	second, _, _ := BuildDPS(syntheticDPSInput())
	if err != nil || len(pending) > 0 || !bytes.Equal(first, second) {
		t.Fatal("producer not deterministic", pending, err)
	}
	p, err := inspectXML(first)
	if err != nil || p.Root != "DPS" || p.DPSID != syntheticDPSInput().DPSID {
		t.Fatal("producer output not accepted by connector inspection", p, err)
	}
}

// officialSchemaRoot is the official XSD 1.01 package versioned in docs (hash manifest).
const officialSchemaRoot = "../../../../../../docs/accounting/nfse/xsd"

func officialSchemaFiles(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(officialSchemaRoot, "xsd-1.01-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ Files map[string]string }
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest.Files
}

// The official TSSerieDPS pattern "^0{0,4}\d{1,5}$" uses ^/$, which are literal
// characters in XSD regex, so xmllint rejects every ordinary series. This test pins
// that the unmodified package fails only on serie; the full structure is validated
// on a temporary copy with exactly those two characters removed. Repository files
// stay byte-identical to the official package.
const officialSeriesPattern = `<xs:pattern value="^0{0,4}\d{1,5}$"/>`

func TestDPSAgainstOfficialSchema(t *testing.T) {
	program, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("requires xmllint; official schema validation not executed")
	}
	files := officialSchemaFiles(t)
	official := filepath.Join(officialSchemaRoot, "1.01")
	if _, err = NewSchemaValidator(SchemaSet{Directory: official, Version: "1.01", DPSFile: "DPS_v1.01.xsd", NFSeFile: "NFSe_v1.01.xsd", Files: files}); err != nil {
		t.Fatal("official package rejected by validator", err)
	}
	sample, _, _ := BuildDPS(syntheticDPSInput())
	cmd := exec.CommandContext(context.Background(), program, "--nonet", "--noout", "--schema", filepath.Join(official, "DPS_v1.01.xsd"), "-")
	cmd.Stdin = bytes.NewReader(sample)
	out, _ := cmd.CombinedOutput()
	if errorsFound := strings.Count(string(out), "validity error"); errorsFound != 1 || !strings.Contains(string(out), "serie") {
		t.Fatalf("unmodified official XSD: expected only the serie anchor quirk, got:\n%s", out)
	}
	normalized := t.TempDir()
	copied := map[string]string{}
	for name := range files {
		raw, e := os.ReadFile(filepath.Join(official, name))
		if e != nil {
			t.Fatal(e)
		}
		if name == "tiposSimples_v1.01.xsd" {
			if !bytes.Contains(raw, []byte(officialSeriesPattern)) {
				t.Fatal("official series pattern changed; review the quirk")
			}
			raw = bytes.Replace(raw, []byte(officialSeriesPattern), []byte(`<xs:pattern value="0{0,4}\d{1,5}"/>`), 1)
		}
		if e = os.WriteFile(filepath.Join(normalized, name), raw, 0o600); e != nil {
			t.Fatal(e)
		}
		copied[name] = sha(raw)
	}
	validator, err := NewSchemaValidator(SchemaSet{Directory: normalized, Version: "1.01", DPSFile: "DPS_v1.01.xsd", NFSeFile: "NFSe_v1.01.xsd", Files: copied})
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*DPSInput){
		"with customer": func(*DPSInput) {},
		"no customer":   func(in *DPSInput) { in.Customer = nil },
		"cpf provider":  func(in *DPSInput) { in.Provider = DPSParty{CPF: "12345678909"} },
		"simples total": func(in *DPSInput) { in.TotalTaxIndicator, in.SimplesTotalPercent = "", "6.00" },
	} {
		in := syntheticDPSInput()
		mutate(&in)
		xmlBytes, pending, e := BuildDPS(in)
		if e != nil || len(pending) > 0 {
			t.Fatal(name, pending, e)
		}
		if e = validator.Validate(context.Background(), "dps", "1.01", xmlBytes); e != nil {
			t.Fatalf("%s: official XSD rejected producer output: %v\n%s", name, e, xmlBytes)
		}
	}
	bad := bytes.Replace(sample, []byte("<cTribNac>010101</cTribNac>"), []byte("<cTribNac>1</cTribNac>"), 1)
	if err = validator.Validate(context.Background(), "dps", "1.01", bad); err == nil {
		t.Fatal("official XSD accepted malformed code")
	}
}
