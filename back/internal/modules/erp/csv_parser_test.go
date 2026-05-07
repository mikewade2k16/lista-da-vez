package erp

import (
	"bytes"
	"errors"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestParseCSVFilename(t *testing.T) {
	meta, err := parseCSVFilename("20240517042655_184-12583959000186-order-20240510010212.csv")
	if err != nil {
		t.Fatalf("parseCSVFilename() error = %v", err)
	}
	if meta.StoreCode != "184" {
		t.Fatalf("expected store code 184, got %q", meta.StoreCode)
	}
	if meta.StoreCNPJ != "12583959000186" {
		t.Fatalf("expected cnpj 12583959000186, got %q", meta.StoreCNPJ)
	}
	if meta.DataType != DataTypeOrder {
		t.Fatalf("expected data type %q, got %q", DataTypeOrder, meta.DataType)
	}
	if got := meta.DataReference.UTC().Format("2006-01-02 15:04:05"); got != "2024-05-10 01:02:12" {
		t.Fatalf("unexpected data reference %q", got)
	}
}

func TestParseCSVFilenameWithoutExtractedAt(t *testing.T) {
	meta, err := parseCSVFilename("184-12583959000186-customer-20260505010059.csv")
	if err != nil {
		t.Fatalf("parseCSVFilename() error = %v", err)
	}
	if meta.StoreCode != "184" {
		t.Fatalf("expected store code 184, got %q", meta.StoreCode)
	}
	if meta.DataType != DataTypeCustomer {
		t.Fatalf("expected data type %q, got %q", DataTypeCustomer, meta.DataType)
	}
	if !meta.ExtractedAt.IsZero() {
		t.Fatalf("expected zero extractedAt for ftp filename, got %v", meta.ExtractedAt)
	}
	if got := meta.DataReference.UTC().Format("2006-01-02 15:04:05"); got != "2026-05-05 01:00:59" {
		t.Fatalf("unexpected data reference %q", got)
	}
}

func TestParseCSVFilenameInvalid(t *testing.T) {
	_, err := parseCSVFilename("bad-file.csv")
	var invalidErr *ErrCSVFilenameInvalid
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected ErrCSVFilenameInvalid, got %v", err)
	}
}

func TestStreamCSVParsesSupportedTypes(t *testing.T) {
	tests := []struct {
		name         string
		dataType     string
		fileName     string
		content      string
		assertRecord func(t *testing.T, record any)
	}{
		{
			name:     "item",
			dataType: DataTypeItem,
			fileName: "20260413010001_184-12583959000186-item-20260413010001.csv",
			content:  "27709;BRINCO;0,79CT;;48;GEMAS;JOIAS;SOLITARIO;;11MM;AMARELO;PR;1783500;27709;2013-12-13 13:02:28;2026-04-04 14:29:43\n",
			assertRecord: func(t *testing.T, record any) {
				row, ok := record.(ItemRawRecord)
				if !ok {
					t.Fatalf("expected ItemRawRecord, got %T", record)
				}
				if row.SKU != "27709" || row.StoreCode != "184" || row.SourceBatchDate != "2026-04-13" {
					t.Fatalf("unexpected item row %#v", row)
				}
			},
		},
		{
			name:     "customer",
			dataType: DataTypeCustomer,
			fileName: "20260413010051_184-12583959000186-customer-20260413010051.csv",
			content:  "ROSANA;ROSA;81447981553;rosa@example.com;79998687125;79991418789;F;19790710;RUA A;143;AP 13;GRAGERU;ARACAJU;SE;BRASIL;49025390;155;12583959000186;20140324;230;04904070569;|MEDICA\n",
			assertRecord: func(t *testing.T, record any) {
				row, ok := record.(CustomerRawRecord)
				if !ok {
					t.Fatalf("expected CustomerRawRecord, got %T", record)
				}
				if row.CPF != "81447981553" || row.Identifier != "04904070569" {
					t.Fatalf("unexpected customer row %#v", row)
				}
			},
		},
		{
			name:     "employee",
			dataType: DataTypeEmployee,
			fileName: "20260413010050_184-12583959000186-employee-20260413010050.csv",
			content:  "BARBARA SANTANA;31327524000115;165;MINISTRO GERALDO BARRETO SOBRAL, 1;;ARACAJU;SE;49026010;1\n",
			assertRecord: func(t *testing.T, record any) {
				row, ok := record.(EmployeeRawRecord)
				if !ok {
					t.Fatalf("expected EmployeeRawRecord, got %T", record)
				}
				if row.OriginalID != "165" || row.StoreIDRaw != "31327524000115" {
					t.Fatalf("unexpected employee row %#v", row)
				}
			},
		},
		{
			name:     "order",
			dataType: DataTypeOrder,
			fileName: "20260413010054_184-12583959000186-order-20260413010054.csv",
			content:  "315199;28828-C1-20260406;12583959000186;05573999559;2026-04-06 12:52:05;8000;45000;361245;8000;1;16;VALE COMPRA 0x|PIX 1x;000;000\n",
			assertRecord: func(t *testing.T, record any) {
				row, ok := record.(OrderRawRecord)
				if !ok {
					t.Fatalf("expected OrderRawRecord, got %T", record)
				}
				if row.OrderID != "315199" || row.SKU != "361245" {
					t.Fatalf("unexpected order row %#v", row)
				}
			},
		},
		{
			name:     "order canceled",
			dataType: DataTypeOrderCanceled,
			fileName: "20260413010057_184-12583959000186-ordercanceled-20260413010057.csv",
			content:  "315247;3466-C1-20260406;56173889000163;77867351515;2026-04-06 17:58:39;50000;000;350937;50000;1;268;MAESTRO CREDITO GETNET 1x;000;\n",
			assertRecord: func(t *testing.T, record any) {
				row, ok := record.(OrderRawRecord)
				if !ok {
					t.Fatalf("expected OrderRawRecord, got %T", record)
				}
				if row.OrderID != "315247" || row.TotalAmountRaw != "50000" {
					t.Fatalf("unexpected canceled order row %#v", row)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta := mustCSVMeta(t, test.fileName)
			var seen []any
			checksumA, rowCount, err := StreamCSV(bytes.NewReader([]byte(test.content)), test.dataType, meta, func(idx int, record any) error {
				seen = append(seen, record)
				if idx != 1 {
					t.Fatalf("expected first row index 1, got %d", idx)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("StreamCSV() error = %v", err)
			}
			checksumB, rowCountB, err := StreamCSV(bytes.NewReader([]byte(test.content)), test.dataType, meta, func(idx int, record any) error {
				return nil
			})
			if err != nil {
				t.Fatalf("StreamCSV() second pass error = %v", err)
			}
			if rowCount != 1 || rowCountB != 1 {
				t.Fatalf("expected one row, got %d and %d", rowCount, rowCountB)
			}
			if checksumA == "" || checksumA != checksumB {
				t.Fatalf("expected stable checksum, got %q and %q", checksumA, checksumB)
			}
			if len(seen) != 1 {
				t.Fatalf("expected one callback row, got %d", len(seen))
			}
			test.assertRecord(t, seen[0])
		})
	}
}

func TestStreamCSVHandlesHeaderAndBOM(t *testing.T) {
	meta := mustCSVMeta(t, "20260413010001_184-12583959000186-item-20260413010001.csv")
	content := "\ufeffsku;name;description;supplierreference;brandname;seasonname;category1;category2;category3;size;color;unit;price;identifier;created_at;updated_at\n" +
		"27709;BRINCO;0,79CT;;48;GEMAS;JOIAS;SOLITARIO;;11MM;AMARELO;PR;1783500;27709;2013-12-13 13:02:28;2026-04-04 14:29:43\n"

	rows := 0
	_, rowCount, err := StreamCSV(bytes.NewReader([]byte(content)), DataTypeItem, meta, func(idx int, record any) error {
		rows++
		return nil
	})
	if err != nil {
		t.Fatalf("StreamCSV() error = %v", err)
	}
	if rowCount != 1 || rows != 1 {
		t.Fatalf("expected one row with BOM/header, got rowCount=%d callback=%d", rowCount, rows)
	}
}

func TestStreamCSVFallsBackToCP1252(t *testing.T) {
	meta := mustCSVMeta(t, "20260413010001_184-12583959000186-item-20260413010001.csv")
	encoded := mustEncodeCP1252(t, "27709;MAÇA;0,79CT;;48;GEMAS;JOIAS;SOLITARIO;;11MM;AMARELO;PR;1783500;27709;2013-12-13 13:02:28;2026-04-04 14:29:43\n")

	var parsed ItemRawRecord
	_, rowCount, err := StreamCSV(bytes.NewReader(encoded), DataTypeItem, meta, func(idx int, record any) error {
		parsed = record.(ItemRawRecord)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamCSV() error = %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected one row, got %d", rowCount)
	}
	if parsed.Name != "MAÇA" {
		t.Fatalf("expected decoded cp1252 value, got %q", parsed.Name)
	}
}

func TestStreamCSVRequiresHeaderWhenConfigured(t *testing.T) {
	previous := csvHeaderModesByType[DataTypeItem]
	csvHeaderModesByType[DataTypeItem] = csvHeaderRequired
	defer func() { csvHeaderModesByType[DataTypeItem] = previous }()

	meta := mustCSVMeta(t, "20260413010001_184-12583959000186-item-20260413010001.csv")
	_, _, err := StreamCSV(bytes.NewReader([]byte("27709;BRINCO;0,79CT;;48;GEMAS;JOIAS;SOLITARIO;;11MM;AMARELO;PR;1783500;27709;2013-12-13 13:02:28;2026-04-04 14:29:43\n")), DataTypeItem, meta, func(idx int, record any) error {
		return nil
	})
	var headerErr *ErrCSVHeaderMismatch
	if !errors.As(err, &headerErr) {
		t.Fatalf("expected ErrCSVHeaderMismatch, got %v", err)
	}
}

func TestStreamCSVRejectsWrongHeaderWhenConfigured(t *testing.T) {
	previous := csvHeaderModesByType[DataTypeItem]
	csvHeaderModesByType[DataTypeItem] = csvHeaderRequired
	defer func() { csvHeaderModesByType[DataTypeItem] = previous }()

	meta := mustCSVMeta(t, "20260413010001_184-12583959000186-item-20260413010001.csv")
	content := "sku;nome_errado;description;supplierreference;brandname;seasonname;category1;category2;category3;size;color;unit;price;identifier;created_at;updated_at\n" +
		"27709;BRINCO;0,79CT;;48;GEMAS;JOIAS;SOLITARIO;;11MM;AMARELO;PR;1783500;27709;2013-12-13 13:02:28;2026-04-04 14:29:43\n"
	_, _, err := StreamCSV(bytes.NewReader([]byte(content)), DataTypeItem, meta, func(idx int, record any) error {
		return nil
	})
	var headerErr *ErrCSVHeaderMismatch
	if !errors.As(err, &headerErr) {
		t.Fatalf("expected ErrCSVHeaderMismatch, got %v", err)
	}
}

func TestStreamCSVRejectsWrongColumnCount(t *testing.T) {
	meta := mustCSVMeta(t, "20260413010001_184-12583959000186-item-20260413010001.csv")
	_, _, err := StreamCSV(bytes.NewReader([]byte("27709;BRINCO;0,79CT\n")), DataTypeItem, meta, func(idx int, record any) error {
		return nil
	})
	var columnErr *ErrCSVColumnCountMismatch
	if !errors.As(err, &columnErr) {
		t.Fatalf("expected ErrCSVColumnCountMismatch, got %v", err)
	}
}

func TestStreamCSVRejectsInvalidRowValue(t *testing.T) {
	meta := mustCSVMeta(t, "20260413010001_184-12583959000186-item-20260413010001.csv")
	_, _, err := StreamCSV(bytes.NewReader([]byte("27709;BRINCO;0,79CT;;48;GEMAS;JOIAS;SOLITARIO;;11MM;AMARELO;PR;preco-invalido;27709;2013-12-13 13:02:28;2026-04-04 14:29:43\n")), DataTypeItem, meta, func(idx int, record any) error {
		return nil
	})
	var rowErr *ErrCSVRowParse
	if !errors.As(err, &rowErr) {
		t.Fatalf("expected ErrCSVRowParse, got %v", err)
	}
	if rowErr.Field != "price" {
		t.Fatalf("expected field price, got %q", rowErr.Field)
	}
}

func mustCSVMeta(t *testing.T, fileName string) csvFileMetadata {
	t.Helper()
	meta, err := parseCSVFilename(fileName)
	if err != nil {
		t.Fatalf("parseCSVFilename() error = %v", err)
	}
	return meta
}

func mustEncodeCP1252(t *testing.T, content string) []byte {
	t.Helper()
	encoded, err := charmap.Windows1252.NewEncoder().String(content)
	if err != nil {
		t.Fatalf("Windows1252 encode error = %v", err)
	}
	return []byte(encoded)
}
