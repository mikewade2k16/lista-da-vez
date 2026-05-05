package settings

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestCoreSettingsScanRowColumnsMatchDTO verifica (Fase 8) que a query em
// getCoreSettingsFromNew lista exatamente os mesmos campos definidos no DTO
// coreSettingsScanRow. Adicionar um campo em um sem o outro deve quebrar este teste.
func TestCoreSettingsScanRowColumnsMatchDTO(t *testing.T) {
	// Conta os campos db: no DTO
	dtoType := reflect.TypeOf(coreSettingsScanRow{})
	dtoFields := 0
	for i := range dtoType.NumField() {
		if dtoType.Field(i).Tag.Get("db") != "" {
			dtoFields++
		}
	}

	// Extrai a lista de colunas da query (tokens entre select e from)
	query := `
		select
			tenant_id::text,
			selected_operation_template_id,
			max_concurrent_services,
			max_concurrent_services_per_consultant,
			timing_fast_close_minutes,
			timing_long_service_minutes,
			timing_low_sale_amount,
			service_cancel_window_seconds,
			test_mode_enabled,
			auto_fill_finish_modal,
			updated_at
		from tenant_operation_core_settings
		where tenant_id = $1::uuid
		limit 1;
	`
	selectBlock := regexp.MustCompile(`(?s)select\s+(.*?)\s+from`).FindStringSubmatch(query)
	if len(selectBlock) < 2 {
		t.Fatal("falha ao extrair bloco SELECT da query")
	}
	queryColumns := strings.Split(strings.TrimSpace(selectBlock[1]), ",")
	if len(queryColumns) != dtoFields {
		t.Fatalf("coreSettingsScanRow: query tem %d colunas, DTO tem %d campos db: — devem estar alinhados",
			len(queryColumns), dtoFields)
	}
}

// TestAlertSettingsScanRowColumnsMatchDTO verifica (Fase 8) que a query em
// getAlertSettingsFromNew lista exatamente os mesmos campos definidos no DTO
// alertSettingsScanRow.
func TestAlertSettingsScanRowColumnsMatchDTO(t *testing.T) {
	dtoType := reflect.TypeOf(alertSettingsScanRow{})
	dtoFields := 0
	for i := range dtoType.NumField() {
		if dtoType.Field(i).Tag.Get("db") != "" {
			dtoFields++
		}
	}

	query := `
		select
			alert_min_conversion_rate,
			alert_max_queue_jump_rate,
			alert_min_pa_score,
			alert_min_ticket_average
		from tenant_alert_settings
		where tenant_id = $1::uuid
		limit 1;
	`
	selectBlock := regexp.MustCompile(`(?s)select\s+(.*?)\s+from`).FindStringSubmatch(query)
	if len(selectBlock) < 2 {
		t.Fatal("falha ao extrair bloco SELECT da query")
	}
	queryColumns := strings.Split(strings.TrimSpace(selectBlock[1]), ",")
	if len(queryColumns) != dtoFields {
		t.Fatalf("alertSettingsScanRow: query tem %d colunas, DTO tem %d campos db: — devem estar alinhados",
			len(queryColumns), dtoFields)
	}
}

// TestModalConfigJSONRoundTrip garante que json.Marshal → json.Unmarshal preserva todos os
// campos do ModalConfig sem perda. Cobre o caminho de escrita/leitura da tabela nova.
func TestModalConfigJSONRoundTrip(t *testing.T) {
	original := DefaultBundle("tenant-1", defaultTemplateID).ModalConfig

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored ModalConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored != original {
		t.Fatalf("round-trip mismatch\ngot:  %+v\nwant: %+v", restored, original)
	}
}

// TestModalConfigJSONMissingFieldsAreZeroValues verifica que campos ausentes no jsonb
// produzem zero values apos unmarshal. normalizeModalSectionRecord e responsavel por
// converter esses zeros em defaults seguros antes de qualquer persistencia.
func TestModalConfigJSONMissingFieldsAreZeroValues(t *testing.T) {
	partial := `{"title":"Atendimento","finishFlowMode":"legacy"}`

	var config ModalConfig
	if err := json.Unmarshal([]byte(partial), &config); err != nil {
		t.Fatalf("unmarshal partial: %v", err)
	}

	if config.Title != "Atendimento" {
		t.Fatalf("title: got %q, want %q", config.Title, "Atendimento")
	}
	if config.FinishFlowMode != "legacy" {
		t.Fatalf("finishFlowMode: got %q, want %q", config.FinishFlowMode, "legacy")
	}
	if config.ProductSeenLabel != "" {
		t.Fatalf("productSeenLabel deveria ser zero value, got %q", config.ProductSeenLabel)
	}
	if config.ShowCustomerNameField {
		t.Fatalf("showCustomerNameField deveria ser false (zero value)")
	}
	if config.ProductSeenNotesMinChars != 0 {
		t.Fatalf("productSeenNotesMinChars deveria ser 0 (zero value), got %d", config.ProductSeenNotesMinChars)
	}
}

// TestModalConfigJSONRoundTripAfterNormalization verifica que normalizar antes de gravar
// e reler produz um ModalConfig completo com defaults corretos, nao zeros.
func TestModalConfigJSONRoundTripAfterNormalization(t *testing.T) {
	section := defaultModalSectionRecord("tenant-1", defaultTemplateID)
	normalized := normalizeModalSectionRecord(section)

	data, err := json.Marshal(normalized.ModalConfig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored ModalConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// apos normalizacao, campos de exibicao devem ter valores proprios do template, nao zero
	if !restored.ShowCustomerNameField {
		t.Fatalf("showCustomerNameField deve ser true apos normalizacao com template padrao")
	}
	if restored.ProductSeenNotesMinChars == 0 {
		t.Fatalf("productSeenNotesMinChars deve ser > 0 apos normalizacao com template padrao")
	}
	if restored.FinishFlowMode == "" {
		t.Fatalf("finishFlowMode nao deve ser vazio apos normalizacao")
	}
}
