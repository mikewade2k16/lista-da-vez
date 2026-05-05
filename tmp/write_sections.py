import os

content = """\
package settings

import (
\t"context"
\t"encoding/json"
\t"errors"
\t"strings"
\t"time"

\t"github.com/jackc/pgx/v5"
)

// coreSettingsScanRow e o DTO de leitura da tabela tenant_operation_core_settings.
// Fase 8: scan por nome via pgx.RowToStructByName elimina mapeamento posicional manual.
// Adicionar um campo novo requer apenas incluir a coluna no SELECT e o campo aqui.
type coreSettingsScanRow struct {
\tTenantID                           string    `db:"tenant_id"`
\tSelectedOperationTemplateID        string    `db:"selected_operation_template_id"`
\tMaxConcurrentServices              int       `db:"max_concurrent_services"`
\tMaxConcurrentServicesPerConsultant int       `db:"max_concurrent_services_per_consultant"`
\tTimingFastCloseMinutes             int       `db:"timing_fast_close_minutes"`
\tTimingLongServiceMinutes           int       `db:"timing_long_service_minutes"`
\tTimingLowSaleAmount                float64   `db:"timing_low_sale_amount"`
\tServiceCancelWindowSeconds         int       `db:"service_cancel_window_seconds"`
\tTestModeEnabled                    bool      `db:"test_mode_enabled"`
\tAutoFillFinishModal                bool      `db:"auto_fill_finish_modal"`
\tUpdatedAt                          time.Time `db:"updated_at"`
}

// alertSettingsScanRow e o DTO de leitura da tabela tenant_alert_settings.
// Fase 8: scan por nome via pgx.RowToStructByName elimina mapeamento posicional manual.
type alertSettingsScanRow struct {
\tAlertMinConversionRate float64 `db:"alert_min_conversion_rate"`
\tAlertMaxQueueJumpRate  float64 `db:"alert_max_queue_jump_rate"`
\tAlertMinPaScore        float64 `db:"alert_min_pa_score"`
\tAlertMinTicketAverage  float64 `db:"alert_min_ticket_average"`
}

// GetOperationSection carrega a secao de operacao exclusivamente das tabelas novas.
// Fase 9: fallback legado (tenant_operation_settings) removido.
func (repository *PostgresRepository) GetOperationSection(ctx context.Context, tenantID string) (OperationSectionRecord, bool, error) {
\tsection, found, err := repository.getCoreSettingsFromNew(ctx, tenantID)
\tif !found || err != nil {
\t\treturn section, found, err
\t}
\tif alerts, alertsFound, alertErr := repository.getAlertSettingsFromNew(ctx, tenantID); alertErr == nil && alertsFound {
\t\tsection.AlertSettings = alerts
\t}
\treturn section, true, nil
}

// getCoreSettingsFromNew le tenant_operation_core_settings usando pgx.RowToStructByName.
// Fase 8: sem scan posicional; novo campo = nova coluna no SELECT + novo campo no DTO.
func (repository *PostgresRepository) getCoreSettingsFromNew(ctx context.Context, tenantID string) (OperationSectionRecord, bool, error) {
\trow, err := pgx.RowToStructByName[coreSettingsScanRow](repository.pool.QueryRow(ctx, `
\t\tselect
\t\t\ttenant_id::text,
\t\t\tselected_operation_template_id,
\t\t\tmax_concurrent_services,
\t\t\tmax_concurrent_services_per_consultant,
\t\t\ttiming_fast_close_minutes,
\t\t\ttiming_long_service_minutes,
\t\t\ttiming_low_sale_amount,
\t\t\tservice_cancel_window_seconds,
\t\t\ttest_mode_enabled,
\t\t\tauto_fill_finish_modal,
\t\t\tupdated_at
\t\tfrom tenant_operation_core_settings
\t\twhere tenant_id = $1::uuid
\t\tlimit 1;
\t`, tenantID))
\tif err != nil {
\t\tif errors.Is(err, pgx.ErrNoRows) {
\t\t\treturn OperationSectionRecord{}, false, nil
\t\t}
\t\treturn OperationSectionRecord{}, false, err
\t}
\treturn OperationSectionRecord{
\t\tTenantID:                    row.TenantID,
\t\tSelectedOperationTemplateID: row.SelectedOperationTemplateID,
\t\tCoreSettings: OperationCoreSettings{
\t\t\tMaxConcurrentServices:              row.MaxConcurrentServices,
\t\t\tMaxConcurrentServicesPerConsultant: row.MaxConcurrentServicesPerConsultant,
\t\t\tTimingFastCloseMinutes:             row.TimingFastCloseMinutes,
\t\t\tTimingLongServiceMinutes:           row.TimingLongServiceMinutes,
\t\t\tTimingLowSaleAmount:                row.TimingLowSaleAmount,
\t\t\tServiceCancelWindowSeconds:         row.ServiceCancelWindowSeconds,
\t\t\tTestModeEnabled:                    row.TestModeEnabled,
\t\t\tAutoFillFinishModal:                row.AutoFillFinishModal,
\t\t},
\t\tCreatedAt: row.UpdatedAt,
\t\tUpdatedAt: row.UpdatedAt,
\t}, true, nil
}

// getAlertSettingsFromNew le tenant_alert_settings usando pgx.RowToStructByName.
// Fase 8: sem scan posicional; novo campo = nova coluna no SELECT + novo campo no DTO.
func (repository *PostgresRepository) getAlertSettingsFromNew(ctx context.Context, tenantID string) (AlertSettings, bool, error) {
\trow, err := pgx.RowToStructByName[alertSettingsScanRow](repository.pool.QueryRow(ctx, `
\t\tselect
\t\t\talert_min_conversion_rate,
\t\t\talert_max_queue_jump_rate,
\t\t\talert_min_pa_score,
\t\t\talert_min_ticket_average
\t\tfrom tenant_alert_settings
\t\twhere tenant_id = $1::uuid
\t\tlimit 1;
\t`, tenantID))
\tif err != nil {
\t\tif errors.Is(err, pgx.ErrNoRows) {
\t\t\treturn AlertSettings{}, false, nil
\t\t}
\t\treturn AlertSettings{}, false, err
\t}
\treturn AlertSettings{
\t\tAlertMinConversionRate: row.AlertMinConversionRate,
\t\tAlertMaxQueueJumpRate:  row.AlertMaxQueueJumpRate,
\t\tAlertMinPaScore:        row.AlertMinPaScore,
\t\tAlertMinTicketAverage:  row.AlertMinTicketAverage,
\t}, true, nil
}

// GetModalSection carrega a secao de modal exclusivamente da tabela nova.
// Fase 9: fallback legado (tenant_operation_settings) removido.
func (repository *PostgresRepository) GetModalSection(ctx context.Context, tenantID string) (ModalSectionRecord, bool, error) {
\treturn repository.getModalSectionFromNew(ctx, tenantID)
}

func (repository *PostgresRepository) getModalSectionFromNew(ctx context.Context, tenantID string) (ModalSectionRecord, bool, error) {
\tvar (
\t\ttenantIDStr        string
\t\tfinishFlowMode     string
\t\tconfigJSON         []byte
\t\tupdatedAt          time.Time
\t\tselectedTemplateID string
\t)
\terr := repository.pool.QueryRow(ctx, `
\t\tselect
\t\t\tf.tenant_id::text,
\t\t\tf.finish_flow_mode,
\t\t\tf.config,
\t\t\tf.updated_at,
\t\t\tcoalesce(
\t\t\t\tc.selected_operation_template_id,
\t\t\t\t'joalheria-padrao'
\t\t\t) as selected_operation_template_id
\t\tfrom tenant_finish_modal_settings f
\t\tleft join tenant_operation_core_settings c on c.tenant_id = f.tenant_id
\t\twhere f.tenant_id = $1::uuid
\t\tlimit 1;
\t`, tenantID).Scan(&tenantIDStr, &finishFlowMode, &configJSON, &updatedAt, &selectedTemplateID)
\tif err != nil {
\t\tif errors.Is(err, pgx.ErrNoRows) {
\t\t\treturn ModalSectionRecord{}, false, nil
\t\t}
\t\treturn ModalSectionRecord{}, false, err
\t}

\tvar modalConfig ModalConfig
\tif err := json.Unmarshal(configJSON, &modalConfig); err != nil {
\t\treturn ModalSectionRecord{}, false, err
\t}
\tmodalConfig.FinishFlowMode = finishFlowMode

\treturn ModalSectionRecord{
\t\tTenantID:                    tenantIDStr,
\t\tSelectedOperationTemplateID: selectedTemplateID,
\t\tModalConfig:                 modalConfig,
\t\tUpdatedAt:                   updatedAt,
\t}, true, nil
}

func (repository *PostgresRepository) GetOptionGroup(ctx context.Context, tenantID string, kind string) ([]OptionItem, error) {
\treturn repository.loadOptionsByKind(ctx, tenantID, kind)
}

func (repository *PostgresRepository) GetProductCatalog(ctx context.Context, tenantID string) ([]ProductItem, error) {
\treturn repository.loadProducts(ctx, tenantID)
}

// UpsertOperationSection grava core e alertas exclusivamente nas tabelas novas.
// Fase 9: escrita legacy (tenant_operation_settings) removida; erros agora sao fatais.
func (repository *PostgresRepository) UpsertOperationSection(ctx context.Context, section OperationSectionRecord) (OperationSectionRecord, error) {
\tsection = normalizeOperationSectionRecord(section)
\tif err := upsertAlertSettingsToNew(ctx, repository.pool, section.TenantID, section.AlertSettings); err != nil {
\t\treturn OperationSectionRecord{}, err
\t}
\tif err := upsertCoreSettingsToNew(ctx, repository.pool, section); err != nil {
\t\treturn OperationSectionRecord{}, err
\t}
\tsaved, found, err := repository.getCoreSettingsFromNew(ctx, section.TenantID)
\tif err != nil {
\t\treturn OperationSectionRecord{}, err
\t}
\tif !found {
\t\treturn OperationSectionRecord{}, ErrTenantNotFound
\t}
\tif alerts, alertsFound, alertErr := repository.getAlertSettingsFromNew(ctx, section.TenantID); alertErr == nil && alertsFound {
\t\tsaved.AlertSettings = alerts
\t}
\treturn saved, nil
}

func upsertCoreSettingsToNew(ctx context.Context, queryer execQueryer, section OperationSectionRecord) error {
\t_, err := queryer.Exec(ctx, `
\t\tinsert into tenant_operation_core_settings (
\t\t\ttenant_id,
\t\t\tselected_operation_template_id,
\t\t\tmax_concurrent_services,
\t\t\tmax_concurrent_services_per_consultant,
\t\t\ttiming_fast_close_minutes,
\t\t\ttiming_long_service_minutes,
\t\t\ttiming_low_sale_amount,
\t\t\tservice_cancel_window_seconds,
\t\t\ttest_mode_enabled,
\t\t\tauto_fill_finish_modal,
\t\t\tupdated_at
\t\t)
\t\tvalues ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
\t\ton conflict (tenant_id) do update
\t\tset
\t\t\tselected_operation_template_id         = excluded.selected_operation_template_id,
\t\t\tmax_concurrent_services                = excluded.max_concurrent_services,
\t\t\tmax_concurrent_services_per_consultant = excluded.max_concurrent_services_per_consultant,
\t\t\ttiming_fast_close_minutes              = excluded.timing_fast_close_minutes,
\t\t\ttiming_long_service_minutes            = excluded.timing_long_service_minutes,
\t\t\ttiming_low_sale_amount                 = excluded.timing_low_sale_amount,
\t\t\tservice_cancel_window_seconds          = excluded.service_cancel_window_seconds,
\t\t\ttest_mode_enabled                      = excluded.test_mode_enabled,
\t\t\tauto_fill_finish_modal                 = excluded.auto_fill_finish_modal,
\t\t\tupdated_at                             = now();
\t`,
\t\tsection.TenantID,
\t\tsection.SelectedOperationTemplateID,
\t\tsection.CoreSettings.MaxConcurrentServices,
\t\tsection.CoreSettings.MaxConcurrentServicesPerConsultant,
\t\tsection.CoreSettings.TimingFastCloseMinutes,
\t\tsection.CoreSettings.TimingLongServiceMinutes,
\t\tsection.CoreSettings.TimingLowSaleAmount,
\t\tsection.CoreSettings.ServiceCancelWindowSeconds,
\t\tsection.CoreSettings.TestModeEnabled,
\t\tsection.CoreSettings.AutoFillFinishModal,
\t)
\treturn err
}

func upsertAlertSettingsToNew(ctx context.Context, queryer execQueryer, tenantID string, alerts AlertSettings) error {
\t_, err := queryer.Exec(ctx, `
\t\tinsert into tenant_alert_settings (
\t\t\ttenant_id,
\t\t\talert_min_conversion_rate,
\t\t\talert_max_queue_jump_rate,
\t\t\talert_min_pa_score,
\t\t\talert_min_ticket_average,
\t\t\tupdated_at
\t\t)
\t\tvalues ($1::uuid, $2, $3, $4, $5, now())
\t\ton conflict (tenant_id) do update
\t\tset
\t\t\talert_min_conversion_rate = excluded.alert_min_conversion_rate,
\t\t\talert_max_queue_jump_rate = excluded.alert_max_queue_jump_rate,
\t\t\talert_min_pa_score        = excluded.alert_min_pa_score,
\t\t\talert_min_ticket_average  = excluded.alert_min_ticket_average,
\t\t\tupdated_at                = now();
\t`, tenantID, alerts.AlertMinConversionRate, alerts.AlertMaxQueueJumpRate, alerts.AlertMinPaScore, alerts.AlertMinTicketAverage)
\treturn err
}

// UpsertModalSection grava exclusivamente na tabela nova.
// Fase 9: escrita legacy (tenant_operation_settings) removida; erro agora e fatal.
func (repository *PostgresRepository) UpsertModalSection(ctx context.Context, section ModalSectionRecord) (ModalSectionRecord, error) {
\tsection = normalizeModalSectionRecord(section)
\tif err := upsertModalSectionToNew(ctx, repository.pool, section); err != nil {
\t\treturn ModalSectionRecord{}, err
\t}
\tsaved, found, err := repository.getModalSectionFromNew(ctx, section.TenantID)
\tif err != nil {
\t\treturn ModalSectionRecord{}, err
\t}
\tif !found {
\t\treturn ModalSectionRecord{}, ErrTenantNotFound
\t}
\treturn saved, nil
}

func upsertModalSectionToNew(ctx context.Context, queryer execQueryer, section ModalSectionRecord) error {
\tconfigJSON, err := json.Marshal(section.ModalConfig)
\tif err != nil {
\t\treturn err
\t}
\t_, err = queryer.Exec(ctx, `
\t\tinsert into tenant_finish_modal_settings (
\t\t\ttenant_id,
\t\t\tfinish_flow_mode,
\t\t\tschema_version,
\t\t\tconfig,
\t\t\tupdated_at
\t\t)
\t\tvalues ($1::uuid, $2, 1, $3::jsonb, now())
\t\ton conflict (tenant_id) do update
\t\tset
\t\t\tfinish_flow_mode = excluded.finish_flow_mode,
\t\t\tconfig           = excluded.config,
\t\t\tupdated_at       = now();
\t`, section.TenantID, section.ModalConfig.FinishFlowMode, string(configJSON))
\treturn err
}

// ApplyOperationTemplate aplica template operacional em transacao unica.
// Fase 9: escrita legacy das secoes operation e modal removida.
// tenant_operation_settings permanece como ancora de FK para opcoes e catalogo.
func (repository *PostgresRepository) ApplyOperationTemplate(ctx context.Context, record OperationTemplateApplyRecord) (time.Time, error) {
\ttenantID := strings.TrimSpace(record.TenantID)
\tif tenantID == "" {
\t\ttenantID = strings.TrimSpace(record.OperationSection.TenantID)
\t}
\tif tenantID == "" {
\t\ttenantID = strings.TrimSpace(record.ModalSection.TenantID)
\t}
\tif tenantID == "" {
\t\treturn time.Time{}, ErrValidation
\t}

\toperationSection := record.OperationSection
\toperationSection.TenantID = tenantID
\toperationSection = normalizeOperationSectionRecord(operationSection)

\tmodalSection := record.ModalSection
\tmodalSection.TenantID = tenantID
\tmodalSection.SelectedOperationTemplateID = operationSection.SelectedOperationTemplateID
\tmodalSection = normalizeModalSectionRecord(modalSection)

\tvisitReasonOptions := normalizeOptions(record.VisitReasonOptions, nil)
\tcustomerSourceOptions := normalizeOptions(record.CustomerSourceOptions, nil)

\ttx, err := repository.pool.Begin(ctx)
\tif err != nil {
\t\treturn time.Time{}, err
\t}

\tdefer func() {
\t\t_ = tx.Rollback(ctx)
\t}()

\tif err := upsertAlertSettingsToNew(ctx, tx, tenantID, operationSection.AlertSettings); err != nil {
\t\treturn time.Time{}, err
\t}
\tif err := upsertCoreSettingsToNew(ctx, tx, operationSection); err != nil {
\t\treturn time.Time{}, err
\t}
\tif err := upsertModalSectionToNew(ctx, tx, modalSection); err != nil {
\t\treturn time.Time{}, err
\t}

\t// Garante a linha ancora em tenant_operation_settings para FK de opcoes e catalogo.
\tif err := ensureConfigRow(ctx, tx, tenantID); err != nil {
\t\treturn time.Time{}, err
\t}

\tif err := replaceOptionGroupTx(ctx, tx, tenantID, optionKindVisitReason, visitReasonOptions); err != nil {
\t\treturn time.Time{}, err
\t}
\tif err := replaceOptionGroupTx(ctx, tx, tenantID, optionKindCustomerSource, customerSourceOptions); err != nil {
\t\treturn time.Time{}, err
\t}

\tupdatedAt, err := touchConfigRow(ctx, tx, tenantID)
\tif err != nil {
\t\treturn time.Time{}, err
\t}

\tif err := tx.Commit(ctx); err != nil {
\t\treturn time.Time{}, err
\t}

\treturn updatedAt, nil
}
"""

target = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                      "back", "internal", "modules", "settings", "store_postgres_sections.go")
with open(target, "w", encoding="utf-8", newline="\n") as f:
    f.write(content)
print(f"OK: {content.count(chr(10))} lines written to {target}")
