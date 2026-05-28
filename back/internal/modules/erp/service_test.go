package erp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStreamItemConsolidatedParsesBatches(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "item_184_consolidado.md")

	content := `# Consolidado Incremental de Itens - Loja 184

- ` + "`loja`: 184" + `
- ` + "`cnpj_loja`: 12583959000186" + `
- ` + "`tipo_arquivo`: item" + `
- ` + "`modo`: append por lote processado" + `
- ` + "`total_lotes_processados`:          1" + `
- ` + "`total_itens_consolidados`:      2" + `

## Ordem das Colunas

loja;cnpj_loja;arquivo_origem;data_lote;linha_origem;sku;name;description;supplierreference;brandname;seasonname;category1;category2;category3;size;color;unit;price;identifier;created_at;updated_at

---

## Lote 2026-04-07 - Loja 184 - 184-12583959000186-item-20260407010001.csv

- ` + "`processado_em`: 2026-04-23T16:23:33-03:00" + `
- ` + "`loja`: 184" + `
- ` + "`cnpj_loja`: 12583959000186" + `
- ` + "`arquivo_origem`: 184-12583959000186-item-20260407010001.csv" + `
- ` + "`data_lote`: 2026-04-07" + `
- ` + "`quantidade_itens`: 2" + `

### DadosCSV

184;12583959000186;184-12583959000186-item-20260407010001.csv;2026-04-07;1;2563;ANEL SOLITARIO;;;48;;JOIAS;SOLITARIO;;10;AMARELO;UN;1290000;2563;2013-10-19 15:05:08;2023-06-30 12:01:31
184;12583959000186;184-12583959000186-item-20260407010001.csv;2026-04-07;2;2564;ANEL SOLITARIO;;;48;;JOIAS;SOLITARIO;;11;AMARELO;UN;1290000;2564;2013-10-19 15:05:08;2023-06-30 12:01:31
`

	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	batches := make([]itemConsolidatedBatch, 0, 1)
	if err := StreamItemConsolidated(filePath, func(batch itemConsolidatedBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatalf("StreamItemConsolidated() error = %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	batch := batches[0]
	if batch.StoreCode != "184" {
		t.Fatalf("expected store 184, got %q", batch.StoreCode)
	}
	if batch.StoreCNPJ != "12583959000186" {
		t.Fatalf("expected cnpj, got %q", batch.StoreCNPJ)
	}
	if batch.SourceFileName != "184-12583959000186-item-20260407010001.csv" {
		t.Fatalf("unexpected source file %q", batch.SourceFileName)
	}
	if len(batch.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(batch.Rows))
	}
	if batch.Rows[0].SKU != "2563" {
		t.Fatalf("unexpected sku %q", batch.Rows[0].SKU)
	}
	if batch.Rows[0].PriceCents == nil || *batch.Rows[0].PriceCents != 1290000 {
		t.Fatalf("unexpected price %v", batch.Rows[0].PriceCents)
	}
	if batch.ChecksumSHA256 == "" {
		t.Fatal("expected checksum to be populated")
	}
}

func TestStreamCustomerConsolidatedParsesBatch(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "customer_184_consolidado.md")

	content := `# Consolidado Incremental de Clientes - Loja 184

- ` + "`loja`: 184" + `
- ` + "`cnpj_loja`: 12583959000186" + `
- ` + "`tipo_arquivo`: customer" + `

## Ordem das Colunas

loja;cnpj_loja;arquivo_origem;data_lote;linha_origem;name;nickname;cpf;email;phone;mobile;gender;birthday;street;number;complement;neighborhood;city;uf;country;zipcode;employee_id;store_id;registered_at;original_id;identifier;tags

---

## Lote 2026-04-07 - Loja 184 - 184-12583959000186-customer-20260407010001.csv

- ` + "`processado_em`: 2026-04-23T16:23:33-03:00" + `
- ` + "`loja`: 184" + `
- ` + "`cnpj_loja`: 12583959000186" + `
- ` + "`arquivo_origem`: 184-12583959000186-customer-20260407010001.csv" + `
- ` + "`data_lote`: 2026-04-07" + `

### DadosCSV

184;12583959000186;184-12583959000186-customer-20260407010001.csv;2026-04-07;1;MARIA;MA;12345678901;maria@email.com;7933334444;7999999999;F;1990-01-02;RUA A;10;;CENTRO;ARACAJU;SE;BR;49000000;206;12583959000186;2026-04-07 10:00:00;C001;CLI-001;vip
`

	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	batches := make([]customerConsolidatedBatch, 0, 1)
	if err := StreamCustomerConsolidated(filePath, func(batch customerConsolidatedBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatalf("StreamCustomerConsolidated() error = %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	batch := batches[0]
	if batch.StoreCNPJ != "12583959000186" {
		t.Fatalf("unexpected cnpj %q", batch.StoreCNPJ)
	}
	if len(batch.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(batch.Rows))
	}
	if batch.Rows[0].CPF != "12345678901" {
		t.Fatalf("unexpected cpf %q", batch.Rows[0].CPF)
	}
	if batch.Rows[0].StoreIDRaw != "12583959000186" {
		t.Fatalf("unexpected store_id %q", batch.Rows[0].StoreIDRaw)
	}
	if batch.Rows[0].RawPayload["store_id"] != "12583959000186" {
		t.Fatalf("expected raw payload store_id, got %#v", batch.Rows[0].RawPayload)
	}
}

func TestStreamOrderConsolidatedParsesBatch(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "order_184_consolidado.md")

	content := `# Consolidado Incremental de Pedidos - Loja 184

- ` + "`loja`: 184" + `
- ` + "`cnpj_loja`: 12583959000186" + `
- ` + "`tipo_arquivo`: order" + `

## Ordem das Colunas

loja;cnpj_loja;arquivo_origem;data_lote;linha_origem;order_id;identifier;store_id;customer_id;order_date;total_amount;product_return;sku;amount;quantity;employee_id;payment_type;total_exclusion;total_debit

---

## Lote 2026-04-07 - Loja 184 - 184-12583959000186-order-20260407010001.csv

- ` + "`processado_em`: 2026-04-23T16:23:33-03:00" + `
- ` + "`loja`: 184" + `
- ` + "`cnpj_loja`: 12583959000186" + `
- ` + "`arquivo_origem`: 184-12583959000186-order-20260407010001.csv" + `
- ` + "`data_lote`: 2026-04-07" + `

### DadosCSV

184;12583959000186;184-12583959000186-order-20260407010001.csv;2026-04-07;1;315001;3439-C1-20260401;56173889000163;69367485549;2026-04-01 11:09:25;170000;000;353505;170000;1;315;PIX 1x;000;
`

	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	batches := make([]orderConsolidatedBatch, 0, 1)
	if err := StreamOrderConsolidated(filePath, DataTypeOrder, func(batch orderConsolidatedBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatalf("StreamOrderConsolidated() error = %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	row := batches[0].Rows[0]
	if row.CustomerID != "69367485549" {
		t.Fatalf("unexpected customer_id %q", row.CustomerID)
	}
	if row.StoreIDRaw != "56173889000163" {
		t.Fatalf("unexpected store_id %q", row.StoreIDRaw)
	}
}

func TestSortSourceCandidatesForIngestPrioritizesMissingLatestFiles(t *testing.T) {
	importedOld := sourceCandidateForTest(t, "184-12583959000186-item-20260508010001.csv")
	missingNew := sourceCandidateForTest(t, "184-12583959000186-item-20260518010001.csv")
	missingOlder := sourceCandidateForTest(t, "184-12583959000186-item-20260512010001.csv")
	importedNew := sourceCandidateForTest(t, "184-12583959000186-item-20260517010001.csv")

	candidates := []sourceCandidate{importedOld, missingOlder, importedNew, missingNew}
	sortSourceCandidatesForIngest(candidates, map[string]syncFileImportState{
		importedOld.meta.OriginalName: {SourceName: importedOld.meta.OriginalName, Status: "imported"},
		importedNew.meta.OriginalName: {SourceName: importedNew.meta.OriginalName, Status: "imported"},
	}, SyncTriggeredByManual)

	got := []string{
		candidates[0].meta.OriginalName,
		candidates[1].meta.OriginalName,
		candidates[2].meta.OriginalName,
		candidates[3].meta.OriginalName,
	}
	want := []string{
		missingNew.meta.OriginalName,
		missingOlder.meta.OriginalName,
		importedNew.meta.OriginalName,
		importedOld.meta.OriginalName,
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("index %d: expected %q, got %q", index, want[index], got[index])
		}
	}
}

func TestSortSourceCandidatesForIngestKeepsBackfillChronological(t *testing.T) {
	missingNew := sourceCandidateForTest(t, "184-12583959000186-item-20260518010001.csv")
	missingOlder := sourceCandidateForTest(t, "184-12583959000186-item-20260512010001.csv")

	candidates := []sourceCandidate{missingNew, missingOlder}
	sortSourceCandidatesForIngest(candidates, nil, SyncTriggeredByBackfill)

	if candidates[0].meta.OriginalName != missingOlder.meta.OriginalName {
		t.Fatalf("expected oldest missing file first, got %q", candidates[0].meta.OriginalName)
	}
}

func TestFilterSourceCandidatesForIngestDropsImportedFiles(t *testing.T) {
	imported := sourceCandidateForTest(t, "184-12583959000186-item-20260508010001.csv")
	missing := sourceCandidateForTest(t, "184-12583959000186-item-20260518010001.csv")

	candidates := filterSourceCandidatesForIngest([]sourceCandidate{imported, missing}, map[string]syncFileImportState{
		imported.meta.OriginalName: {SourceName: imported.meta.OriginalName, Status: "imported"},
	})

	if len(candidates) != 1 {
		t.Fatalf("expected 1 missing candidate, got %d", len(candidates))
	}
	if candidates[0].meta.OriginalName != missing.meta.OriginalName {
		t.Fatalf("expected missing file, got %q", candidates[0].meta.OriginalName)
	}
}

func TestNewSourceForIngestForcesBackfillRecursive(t *testing.T) {
	service := NewService(nil, Options{SourceKind: SourceKindFTP})
	var got SourceOptions
	service.sourceFactory = func(options SourceOptions) (ErpSource, error) {
		got = options
		return noopSource{}, nil
	}

	source, err := service.newSourceForIngest(IngestInput{TriggeredBy: SyncTriggeredByBackfill})
	if err != nil {
		t.Fatalf("newSourceForIngest() error = %v", err)
	}
	_ = source.Close()

	if !got.Recursive {
		t.Fatal("expected backfill source to be recursive")
	}
}

func TestNewSourceForIngestKeepsManualRecursiveConfig(t *testing.T) {
	service := NewService(nil, Options{SourceKind: SourceKindFTP, SourceRecursive: false})
	var got SourceOptions
	service.sourceFactory = func(options SourceOptions) (ErpSource, error) {
		got = options
		return noopSource{}, nil
	}

	source, err := service.newSourceForIngest(IngestInput{TriggeredBy: SyncTriggeredByManual})
	if err != nil {
		t.Fatalf("newSourceForIngest() error = %v", err)
	}
	_ = source.Close()

	if got.Recursive {
		t.Fatal("expected manual sync source to keep recursive disabled")
	}
}

func sourceCandidateForTest(t *testing.T, name string) sourceCandidate {
	t.Helper()
	meta, err := parseCSVFilename(name)
	if err != nil {
		t.Fatalf("parseCSVFilename(%q) error = %v", name, err)
	}
	return sourceCandidate{
		info: SourceFileInfo{
			Name:    name,
			ModTime: time.Date(2026, 5, 18, 1, 0, 0, 0, time.UTC),
		},
		meta: meta,
	}
}

type noopSource struct{}

func (noopSource) List(context.Context, string) ([]SourceFileInfo, error) { return nil, nil }
func (noopSource) Open(context.Context, string) (io.ReadCloser, error)    { return nil, nil }
func (noopSource) Kind() string                                           { return SourceKindFTP }
func (noopSource) Close() error                                           { return nil }
