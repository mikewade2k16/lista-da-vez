package settings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// coreSettingsScanRow e o DTO de leitura da tabela tenant_operation_core_settings.
// Fase 8: scan por nome via pgx.RowToStructByName elimina mapeamento posicional manual.
// Adicionar um campo novo requer apenas incluir a coluna no SELECT e o campo aqui.
type coreSettingsScanRow struct {
	TenantID                           string    `db:"tenant_id"`
	SelectedOperationTemplateID        string    `db:"selected_operation_template_id"`
	MaxConcurrentServices              int       `db:"max_concurrent_services"`
	MaxConcurrentServicesPerConsultant int       `db:"max_concurrent_services_per_consultant"`
	TimingFastCloseMinutes             int       `db:"timing_fast_close_minutes"`
	TimingLongServiceMinutes           int       `db:"timing_long_service_minutes"`
	TimingLowSaleAmount                float64   `db:"timing_low_sale_amount"`
	ServiceCancelWindowSeconds         int       `db:"service_cancel_window_seconds"`
	TestModeEnabled                    bool      `db:"test_mode_enabled"`
	AutoFillFinishModal                bool      `db:"auto_fill_finish_modal"`
	UpdatedAt                          time.Time `db:"updated_at"`
}

// alertSettingsScanRow e o DTO de leitura da tabela tenant_alert_settings.
// Fase 8: scan por nome via pgx.RowToStructByName elimina mapeamento posicional manual.
type alertSettingsScanRow struct {
	AlertMinConversionRate float64 `db:"alert_min_conversion_rate"`
	AlertMaxQueueJumpRate  float64 `db:"alert_max_queue_jump_rate"`
	AlertMinPaScore        float64 `db:"alert_min_pa_score"`
	AlertMinTicketAverage  float64 `db:"alert_min_ticket_average"`
}

// GetOperationSection carrega a secao de operacao exclusivamente das tabelas novas.
// Fase 9: fallback legado (tenant_operation_settings) removido.
func (repository *PostgresRepository) GetOperationSection(ctx context.Context, tenantID string) (OperationSectionRecord, bool, error) {
	section, found, err := repository.getCoreSettingsFromNew(ctx, tenantID)
	if !found || err != nil {
		return section, found, err
	}
	if alerts, alertsFound, alertErr := repository.getAlertSettingsFromNew(ctx, tenantID); alertErr == nil && alertsFound {
		section.AlertSettings = alerts
	}
	return section, true, nil
}

// getCoreSettingsFromNew le tenant_operation_core_settings usando pgx.RowToStructByName.
// Fase 8: sem scan posicional; novo campo = nova coluna no SELECT + novo campo no DTO.
func (repository *PostgresRepository) getCoreSettingsFromNew(ctx context.Context, tenantID string) (OperationSectionRecord, bool, error) {
	rows, err := repository.pool.Query(ctx, `
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
	`, tenantID)
	if err != nil {
		return OperationSectionRecord{}, false, err
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[coreSettingsScanRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OperationSectionRecord{}, false, nil
		}
		return OperationSectionRecord{}, false, err
	}
	return OperationSectionRecord{
		TenantID:                    row.TenantID,
		SelectedOperationTemplateID: row.SelectedOperationTemplateID,
		CoreSettings: OperationCoreSettings{
			MaxConcurrentServices:              row.MaxConcurrentServices,
			MaxConcurrentServicesPerConsultant: row.MaxConcurrentServicesPerConsultant,
			TimingFastCloseMinutes:             row.TimingFastCloseMinutes,
			TimingLongServiceMinutes:           row.TimingLongServiceMinutes,
			TimingLowSaleAmount:                row.TimingLowSaleAmount,
			ServiceCancelWindowSeconds:         row.ServiceCancelWindowSeconds,
			TestModeEnabled:                    row.TestModeEnabled,
			AutoFillFinishModal:                row.AutoFillFinishModal,
		},
		CreatedAt: row.UpdatedAt,
		UpdatedAt: row.UpdatedAt,
	}, true, nil
}

// getAlertSettingsFromNew le tenant_alert_settings usando pgx.RowToStructByName.
// Fase 8: sem scan posicional; novo campo = nova coluna no SELECT + novo campo no DTO.
func (repository *PostgresRepository) getAlertSettingsFromNew(ctx context.Context, tenantID string) (AlertSettings, bool, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			alert_min_conversion_rate,
			alert_max_queue_jump_rate,
			alert_min_pa_score,
			alert_min_ticket_average
		from tenant_alert_settings
		where tenant_id = $1::uuid
		limit 1;
	`, tenantID)
	if err != nil {
		return AlertSettings{}, false, err
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[alertSettingsScanRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AlertSettings{}, false, nil
		}
		return AlertSettings{}, false, err
	}
	return AlertSettings{
		AlertMinConversionRate: row.AlertMinConversionRate,
		AlertMaxQueueJumpRate:  row.AlertMaxQueueJumpRate,
		AlertMinPaScore:        row.AlertMinPaScore,
		AlertMinTicketAverage:  row.AlertMinTicketAverage,
	}, true, nil
}

// GetModalSection carrega a secao de modal exclusivamente da tabela nova.
// Fase 9: fallback legado (tenant_operation_settings) removido.
func (repository *PostgresRepository) GetModalSection(ctx context.Context, tenantID string) (ModalSectionRecord, bool, error) {
	return repository.getModalSectionFromNew(ctx, tenantID)
}

func (repository *PostgresRepository) getModalSectionFromNew(ctx context.Context, tenantID string) (ModalSectionRecord, bool, error) {
	var (
		tenantIDStr        string
		finishFlowMode     string
		configJSON         []byte
		updatedAt          time.Time
		selectedTemplateID string
	)
	err := repository.pool.QueryRow(ctx, `
		select
			f.tenant_id::text,
			f.finish_flow_mode,
			f.config,
			f.updated_at,
			coalesce(
				c.selected_operation_template_id,
				'joalheria-padrao'
			) as selected_operation_template_id
		from tenant_finish_modal_settings f
		left join tenant_operation_core_settings c on c.tenant_id = f.tenant_id
		where f.tenant_id = $1::uuid
		limit 1;
	`, tenantID).Scan(&tenantIDStr, &finishFlowMode, &configJSON, &updatedAt, &selectedTemplateID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModalSectionRecord{}, false, nil
		}
		return ModalSectionRecord{}, false, err
	}

	var modalConfig ModalConfig
	if err := json.Unmarshal(configJSON, &modalConfig); err != nil {
		return ModalSectionRecord{}, false, err
	}
	modalConfig.FinishFlowMode = finishFlowMode

	return ModalSectionRecord{
		TenantID:                    tenantIDStr,
		SelectedOperationTemplateID: selectedTemplateID,
		ModalConfig:                 modalConfig,
		UpdatedAt:                   updatedAt,
	}, true, nil
}

func (repository *PostgresRepository) GetOptionGroup(ctx context.Context, tenantID string, kind string) ([]OptionItem, error) {
	return repository.loadOptionsByKind(ctx, tenantID, kind)
}

func (repository *PostgresRepository) GetProductCatalog(ctx context.Context, tenantID string) ([]ProductItem, error) {
	return repository.loadProducts(ctx, tenantID)
}

// UpsertOperationSection grava core e alertas exclusivamente nas tabelas novas.
// Fase 9: escrita legacy (tenant_operation_settings) removida; erros agora sao fatais.
func (repository *PostgresRepository) UpsertOperationSection(ctx context.Context, section OperationSectionRecord) (OperationSectionRecord, error) {
	section = normalizeOperationSectionRecord(section)
	if err := upsertAlertSettingsToNew(ctx, repository.pool, section.TenantID, section.AlertSettings); err != nil {
		return OperationSectionRecord{}, err
	}
	if err := upsertCoreSettingsToNew(ctx, repository.pool, section); err != nil {
		return OperationSectionRecord{}, err
	}
	saved, found, err := repository.getCoreSettingsFromNew(ctx, section.TenantID)
	if err != nil {
		return OperationSectionRecord{}, err
	}
	if !found {
		return OperationSectionRecord{}, ErrTenantNotFound
	}
	if alerts, alertsFound, alertErr := repository.getAlertSettingsFromNew(ctx, section.TenantID); alertErr == nil && alertsFound {
		saved.AlertSettings = alerts
	}
	return saved, nil
}

func upsertCoreSettingsToNew(ctx context.Context, queryer execQueryer, section OperationSectionRecord) error {
	_, err := queryer.Exec(ctx, `
		insert into tenant_operation_core_settings (
			tenant_id,
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
		)
		values ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		on conflict (tenant_id) do update
		set
			selected_operation_template_id         = excluded.selected_operation_template_id,
			max_concurrent_services                = excluded.max_concurrent_services,
			max_concurrent_services_per_consultant = excluded.max_concurrent_services_per_consultant,
			timing_fast_close_minutes              = excluded.timing_fast_close_minutes,
			timing_long_service_minutes            = excluded.timing_long_service_minutes,
			timing_low_sale_amount                 = excluded.timing_low_sale_amount,
			service_cancel_window_seconds          = excluded.service_cancel_window_seconds,
			test_mode_enabled                      = excluded.test_mode_enabled,
			auto_fill_finish_modal                 = excluded.auto_fill_finish_modal,
			updated_at                             = now();
	`,
		section.TenantID,
		section.SelectedOperationTemplateID,
		section.CoreSettings.MaxConcurrentServices,
		section.CoreSettings.MaxConcurrentServicesPerConsultant,
		section.CoreSettings.TimingFastCloseMinutes,
		section.CoreSettings.TimingLongServiceMinutes,
		section.CoreSettings.TimingLowSaleAmount,
		section.CoreSettings.ServiceCancelWindowSeconds,
		section.CoreSettings.TestModeEnabled,
		section.CoreSettings.AutoFillFinishModal,
	)
	return err
}

func upsertAlertSettingsToNew(ctx context.Context, queryer execQueryer, tenantID string, alerts AlertSettings) error {
	_, err := queryer.Exec(ctx, `
		insert into tenant_alert_settings (
			tenant_id,
			alert_min_conversion_rate,
			alert_max_queue_jump_rate,
			alert_min_pa_score,
			alert_min_ticket_average,
			updated_at
		)
		values ($1::uuid, $2, $3, $4, $5, now())
		on conflict (tenant_id) do update
		set
			alert_min_conversion_rate = excluded.alert_min_conversion_rate,
			alert_max_queue_jump_rate = excluded.alert_max_queue_jump_rate,
			alert_min_pa_score        = excluded.alert_min_pa_score,
			alert_min_ticket_average  = excluded.alert_min_ticket_average,
			updated_at                = now();
	`, tenantID, alerts.AlertMinConversionRate, alerts.AlertMaxQueueJumpRate, alerts.AlertMinPaScore, alerts.AlertMinTicketAverage)
	return err
}

// UpsertModalSection grava exclusivamente na tabela nova.
// Fase 9: escrita legacy (tenant_operation_settings) removida; erro agora e fatal.
func (repository *PostgresRepository) UpsertModalSection(ctx context.Context, section ModalSectionRecord) (ModalSectionRecord, error) {
	section = normalizeModalSectionRecord(section)
	if err := upsertModalSectionToNew(ctx, repository.pool, section); err != nil {
		return ModalSectionRecord{}, err
	}
	saved, found, err := repository.getModalSectionFromNew(ctx, section.TenantID)
	if err != nil {
		return ModalSectionRecord{}, err
	}
	if !found {
		return ModalSectionRecord{}, ErrTenantNotFound
	}
	return saved, nil
}

func upsertModalSectionToNew(ctx context.Context, queryer execQueryer, section ModalSectionRecord) error {
	configJSON, err := json.Marshal(section.ModalConfig)
	if err != nil {
		return err
	}
	_, err = queryer.Exec(ctx, `
		insert into tenant_finish_modal_settings (
			tenant_id,
			finish_flow_mode,
			schema_version,
			config,
			updated_at
		)
		values ($1::uuid, $2, 1, $3::jsonb, now())
		on conflict (tenant_id) do update
		set
			finish_flow_mode = excluded.finish_flow_mode,
			config           = excluded.config,
			updated_at       = now();
	`, section.TenantID, section.ModalConfig.FinishFlowMode, string(configJSON))
	return err
}

// ApplyOperationTemplate aplica template operacional em transacao unica.
// Fase 9: escrita legacy das secoes operation e modal removida.
// tenant_operation_settings permanece como ancora de FK para opcoes e catalogo.
func (repository *PostgresRepository) ApplyOperationTemplate(ctx context.Context, record OperationTemplateApplyRecord) (time.Time, error) {
	tenantID := strings.TrimSpace(record.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(record.OperationSection.TenantID)
	}
	if tenantID == "" {
		tenantID = strings.TrimSpace(record.ModalSection.TenantID)
	}
	if tenantID == "" {
		return time.Time{}, ErrValidation
	}

	operationSection := record.OperationSection
	operationSection.TenantID = tenantID
	operationSection = normalizeOperationSectionRecord(operationSection)

	modalSection := record.ModalSection
	modalSection.TenantID = tenantID
	modalSection.SelectedOperationTemplateID = operationSection.SelectedOperationTemplateID
	modalSection = normalizeModalSectionRecord(modalSection)

	visitReasonOptions := normalizeOptions(record.VisitReasonOptions, nil)
	customerSourceOptions := normalizeOptions(record.CustomerSourceOptions, nil)

	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := upsertAlertSettingsToNew(ctx, tx, tenantID, operationSection.AlertSettings); err != nil {
		return time.Time{}, err
	}
	if err := upsertCoreSettingsToNew(ctx, tx, operationSection); err != nil {
		return time.Time{}, err
	}
	if err := upsertModalSectionToNew(ctx, tx, modalSection); err != nil {
		return time.Time{}, err
	}

	// Garante a linha ancora em tenant_operation_settings para FK de opcoes e catalogo.
	if err := ensureConfigRow(ctx, tx, tenantID); err != nil {
		return time.Time{}, err
	}

	if err := replaceOptionGroupTx(ctx, tx, tenantID, optionKindVisitReason, visitReasonOptions); err != nil {
		return time.Time{}, err
	}
	if err := replaceOptionGroupTx(ctx, tx, tenantID, optionKindCustomerSource, customerSourceOptions); err != nil {
		return time.Time{}, err
	}

	updatedAt, err := touchConfigRow(ctx, tx, tenantID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}
