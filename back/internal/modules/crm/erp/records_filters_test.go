package erp

import (
	"strings"
	"testing"
)

func TestNormalizeDateField(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "batch_date exato", raw: "batch_date", want: "batch_date"},
		{name: "batch_date maiusculo", raw: "BATCH_DATE", want: "batch_date"},
		{name: "batch_date com espacos", raw: "  batch_date  ", want: "batch_date"},
		{name: "batch_date misto com espacos", raw: " Batch_Date ", want: "batch_date"},
		{name: "vazio cai em order_date", raw: "", want: "order_date"},
		{name: "order_date explicito", raw: "order_date", want: "order_date"},
		{name: "valor arbitrario cai em order_date", raw: "x", want: "order_date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDateField(tt.raw); got != tt.want {
				t.Errorf("normalizeDateField(%q) = %q, quer %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseOptionalCents(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "vazio", raw: "", want: 0},
		{name: "so espacos", raw: "   ", want: 0},
		{name: "nao numerico", raw: "abc", want: 0},
		{name: "negativo vira zero", raw: "-5", want: 0},
		{name: "zero", raw: "0", want: 0},
		{name: "valor positivo", raw: "12345", want: 12345},
		{name: "valor com espacos nas pontas", raw: " 100 ", want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseOptionalCents(tt.raw); got != tt.want {
				t.Errorf("parseOptionalCents(%q) = %d, quer %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestResolveOrderDateColumn(t *testing.T) {
	tests := []struct {
		name           string
		dataType       string
		dateField      string
		wantColumn     string
		wantBatchStyle bool
	}{
		{name: "order com batch_date usa lote", dataType: DataTypeOrder, dateField: "batch_date", wantColumn: "source_batch_date", wantBatchStyle: true},
		{name: "order com order_date usa compra", dataType: DataTypeOrder, dateField: "order_date", wantColumn: "order_date", wantBatchStyle: false},
		{name: "order sem dateField usa compra", dataType: DataTypeOrder, dateField: "", wantColumn: "order_date", wantBatchStyle: false},
		{name: "ordercanceled com order_date usa compra", dataType: DataTypeOrderCanceled, dateField: "order_date", wantColumn: "order_date", wantBatchStyle: false},
		{name: "customer sempre lote", dataType: DataTypeCustomer, dateField: "order_date", wantColumn: "source_batch_date", wantBatchStyle: true},
		{name: "employee sempre lote", dataType: DataTypeEmployee, dateField: "", wantColumn: "source_batch_date", wantBatchStyle: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column, batchStyle := resolveOrderDateColumn(tt.dataType, tt.dateField)
			if column != tt.wantColumn {
				t.Errorf("resolveOrderDateColumn(%q, %q) column = %q, quer %q", tt.dataType, tt.dateField, column, tt.wantColumn)
			}
			if batchStyle != tt.wantBatchStyle {
				t.Errorf("resolveOrderDateColumn(%q, %q) batchStyle = %v, quer %v", tt.dataType, tt.dateField, batchStyle, tt.wantBatchStyle)
			}
		})
	}
}

func TestDateRangeClause(t *testing.T) {
	t.Run("modo lote inclusivo nas duas pontas", func(t *testing.T) {
		clause := dateRangeClause("source_batch_date", true, "$9", "$10")
		wants := []string{
			"source_batch_date::date >= $9::date",
			"source_batch_date::date <= $10::date",
		}
		for _, want := range wants {
			if !strings.Contains(clause, want) {
				t.Errorf("dateRangeClause batch nao contem %q; clause = %q", want, clause)
			}
		}
	})

	t.Run("modo order_date com fim exclusivo", func(t *testing.T) {
		clause := dateRangeClause("order_date", false, "$3", "$4")
		wants := []string{
			"order_date >= $3::date",
			"order_date < ($4::date + 1)",
		}
		for _, want := range wants {
			if !strings.Contains(clause, want) {
				t.Errorf("dateRangeClause order_date nao contem %q; clause = %q", want, clause)
			}
		}
	})
}

func TestActiveOrderCancelClause(t *testing.T) {
	t.Run("order ativo gera anti-join", func(t *testing.T) {
		clause := activeOrderCancelClause(DataTypeOrder)
		wants := []string{"not exists", "erp_order_canceled_raw", "o.order_id"}
		for _, want := range wants {
			if !strings.Contains(clause, want) {
				t.Errorf("activeOrderCancelClause(order) nao contem %q; clause = %q", want, clause)
			}
		}
	})

	t.Run("ordercanceled nao recebe anti-join", func(t *testing.T) {
		if clause := activeOrderCancelClause(DataTypeOrderCanceled); clause != "" {
			t.Errorf("activeOrderCancelClause(ordercanceled) = %q, quer vazio", clause)
		}
	})

	t.Run("customer nao recebe anti-join", func(t *testing.T) {
		if clause := activeOrderCancelClause(DataTypeCustomer); clause != "" {
			t.Errorf("activeOrderCancelClause(customer) = %q, quer vazio", clause)
		}
	})
}

func TestResolveRawRecordsSortColumn(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		sortBy   string
		want     string
	}{
		{name: "order total_amount_raw vira cents", dataType: DataTypeOrder, sortBy: "total_amount_raw", want: "total_amount_cents"},
		{name: "order order_date_raw vira order_date", dataType: DataTypeOrder, sortBy: "order_date_raw", want: "order_date"},
		{name: "order customer_id passa direto", dataType: DataTypeOrder, sortBy: "customer_id", want: "customer_id"},
		{name: "order source_batch_date passa direto", dataType: DataTypeOrder, sortBy: "source_batch_date", want: "source_batch_date"},
		{name: "order coluna inexistente faz fallback", dataType: DataTypeOrder, sortBy: "coluna_inexistente", want: "source_batch_date"},
		{name: "ordercanceled total_amount_raw vira cents", dataType: DataTypeOrderCanceled, sortBy: "total_amount_raw", want: "total_amount_cents"},
		{name: "customer name passa direto", dataType: DataTypeCustomer, sortBy: "name", want: "name"},
		{name: "customer total_amount_raw nao e valido faz fallback", dataType: DataTypeCustomer, sortBy: "total_amount_raw", want: "source_batch_date"},
		{name: "employee name passa direto", dataType: DataTypeEmployee, sortBy: "name", want: "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveRawRecordsSortColumn(tt.dataType, tt.sortBy); got != tt.want {
				t.Errorf("resolveRawRecordsSortColumn(%q, %q) = %q, quer %q", tt.dataType, tt.sortBy, got, tt.want)
			}
		})
	}
}
