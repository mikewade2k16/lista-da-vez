package erp

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository    *PostgresRepository
	options       Options
	sourceFactory func(SourceOptions) (ErpSource, error)
}

func NewService(repository *PostgresRepository, options Options) *Service {
	if options.CSVMaxBytes <= 0 {
		options.CSVMaxBytes = defaultCSVMaxBytes
	}
	if options.ManualSyncMaxFiles <= 0 {
		options.ManualSyncMaxFiles = defaultManualSyncMaxFiles
	}
	if options.BackfillMaxFiles <= 0 {
		options.BackfillMaxFiles = defaultBackfillMaxFiles
	}
	if options.ManualSyncMinInterval <= 0 {
		options.ManualSyncMinInterval = defaultManualSyncMinInterval
	}
	return &Service{repository: repository, options: options, sourceFactory: NewSource}
}

func (service *Service) Status(ctx context.Context, principal auth.Principal, tenantID string, storeCode string) (StatusResponse, error) {
	if !canViewERPAdminDetails(principal) {
		return StatusResponse{}, ErrForbidden
	}

	store, err := service.resolveERPScope(ctx, principal, tenantID, storeCode)
	if err != nil {
		return StatusResponse{}, err
	}

	return service.repository.GetStatus(ctx, store)
}

func (service *Service) Products(ctx context.Context, principal auth.Principal, query ProductQuery) (ProductListResponse, error) {
	if !canViewERP(principal) {
		return ProductListResponse{}, ErrForbidden
	}

	store, err := service.resolveERPScope(ctx, principal, query.TenantID, query.StoreCode)
	if err != nil {
		return ProductListResponse{}, err
	}

	normalized := normalizeProductQuery(query)
	return service.repository.ListCurrentItems(ctx, store, normalized)
}

func (service *Service) Records(ctx context.Context, principal auth.Principal, query RawRecordsQuery) (RawRecordsListResponse, error) {
	if !canViewERP(principal) {
		return RawRecordsListResponse{}, ErrForbidden
	}

	normalized := normalizeRawRecordsQuery(query)
	if normalized.DataType == DataTypeItem {
		return RawRecordsListResponse{}, ErrUnsupportedDataType
	}
	if !isSupportedDataType(normalized.DataType) {
		return RawRecordsListResponse{}, ErrUnsupportedDataType
	}

	store, err := service.resolveERPScope(ctx, principal, normalized.TenantID, normalized.StoreCode)
	if err != nil {
		return RawRecordsListResponse{}, err
	}

	return service.repository.ListRawRecords(ctx, store, normalized)
}

func (service *Service) RecordsStats(ctx context.Context, principal auth.Principal, query RecordsStatsQuery) (RecordsStatsResponse, error) {
	if !canViewERPAdminDetails(principal) {
		return RecordsStatsResponse{}, ErrForbidden
	}
	store, err := service.resolveERPScope(ctx, principal, query.TenantID, query.StoreCode)
	if err != nil {
		return RecordsStatsResponse{}, err
	}
	normalized := RecordsStatsQuery{
		TenantID:       query.TenantID,
		StoreCode:      query.StoreCode,
		DataType:       strings.TrimSpace(strings.ToLower(query.DataType)),
		Search:         strings.TrimSpace(query.Search),
		SpecificSearch: strings.TrimSpace(query.SpecificSearch),
		DateFrom:       strings.TrimSpace(query.DateFrom),
		DateTo:         strings.TrimSpace(query.DateTo),
	}
	return service.repository.GetRecordsStats(ctx, store, normalized)
}

func (service *Service) Runs(ctx context.Context, principal auth.Principal, query RunsQuery) (SyncRunsListResponse, error) {
	if !canViewERPAdminDetails(principal) {
		return SyncRunsListResponse{}, ErrForbidden
	}

	normalized := normalizeRunsQuery(query)
	if normalized.DataType != "" && !isSupportedDataType(normalized.DataType) {
		return SyncRunsListResponse{}, ErrUnsupportedDataType
	}

	store, err := service.resolveERPScope(ctx, principal, normalized.TenantID, normalized.StoreCode)
	if err != nil {
		return SyncRunsListResponse{}, err
	}

	return service.repository.ListSyncRuns(ctx, store, normalized)
}

func (service *Service) Overview(ctx context.Context, principal auth.Principal, tenantID string, storeCode string) (SyncOverviewResponse, error) {
	if !canViewERPAdminDetails(principal) {
		return SyncOverviewResponse{}, ErrForbidden
	}

	store, err := service.resolveERPScope(ctx, principal, tenantID, storeCode)
	if err != nil {
		return SyncOverviewResponse{}, err
	}

	status, err := service.repository.GetStatus(ctx, store)
	if err != nil {
		return SyncOverviewResponse{}, err
	}

	source, err := service.newSource()
	if err != nil {
		return SyncOverviewResponse{}, err
	}
	defer source.Close()

	listedFiles, err := source.List(ctx, store.StoreCode)
	sourceUnavailable := err != nil

	fileStates, err := service.repository.ListLatestSyncFileStates(ctx, store)
	if err != nil {
		return SyncOverviewResponse{}, err
	}

	entityMap := make(map[string]SyncCoverageEntitySummary, len(supportedDataTypes))
	for _, dataType := range supportedDataTypes {
		entityMap[dataType] = SyncCoverageEntitySummary{DataType: dataType}
	}

	for _, typeStatus := range status.TypeStats {
		summary := entityMap[typeStatus.DataType]
		summary.RowsInBank = typeStatus.RawRows
		summary.SearchableRows = typeStatus.TotalRows
		summary.CurrentRows = typeStatus.CurrentRows
		entityMap[typeStatus.DataType] = summary
	}

	overview := SyncOverviewResponse{
		Store:      status.Store,
		SourceKind: source.Kind(),
		SourcePath: service.describeSource(source),
		Automatic: SyncAutomationSummary{
			Enabled:       service.options.SyncAutomaticEnabled,
			Interval:      service.options.SyncInterval.String(),
			HourUTC:       service.options.SyncHourUTC,
			DryRunDefault: service.options.SyncDryRunDefault,
		},
		Entities:         make([]SyncCoverageEntitySummary, 0, len(supportedDataTypes)),
		MissingFiles:     make([]SyncCoverageFileSummary, 0),
		AgentDocPath:     "back/internal/modules/erp/AGENT.md",
		AgentDocURL:      "/erp-agent.md",
		LastRun:          status.LastRun,
		LastImportedFile: status.LastImportedFile,
	}

	if sourceUnavailable {
		for _, state := range fileStates {
			meta, parseErr := parseCSVFilename(filepath.Base(state.SourceName))
			if parseErr != nil {
				continue
			}

			summary := entityMap[meta.DataType]
			summary.RemoteFilesTotal++
			overview.Totals.TotalFiles++
			if strings.EqualFold(strings.TrimSpace(state.Status), "imported") {
				summary.ImportedFiles++
				overview.Totals.ImportedFiles++
			} else {
				summary.PendingFiles++
				overview.Totals.PendingFiles++
				overview.MissingFiles = append(overview.MissingFiles, SyncCoverageFileSummary{
					SourceName:    meta.OriginalName,
					DataType:      meta.DataType,
					DataReference: meta.DataReference,
					Imported:      false,
					Status:        firstNonEmpty(strings.TrimSpace(state.Status), "pending"),
					RecordCount:   state.RecordCount,
					ImportedAt:    state.ImportedAt,
					SourceKind:    state.SourceKind,
				})
			}
			entityMap[meta.DataType] = summary
		}

		for _, dataType := range supportedDataTypes {
			overview.Entities = append(overview.Entities, entityMap[dataType])
		}
		sort.Slice(overview.MissingFiles, func(left int, right int) bool {
			if overview.MissingFiles[left].DataType == overview.MissingFiles[right].DataType {
				return overview.MissingFiles[left].DataReference.Before(overview.MissingFiles[right].DataReference)
			}
			return overview.MissingFiles[left].DataType < overview.MissingFiles[right].DataType
		})
		overview.FullyImported = overview.Totals.TotalFiles > 0 && overview.Totals.PendingFiles == 0
		return overview, nil
	}

	for _, fileInfo := range listedFiles {
		meta, parseErr := parseCSVFilename(filepath.Base(fileInfo.Name))
		if parseErr != nil {
			continue
		}

		summary := entityMap[meta.DataType]
		summary.RemoteFilesTotal++
		overview.Totals.TotalFiles++

		state, hasState := fileStates[meta.OriginalName]
		imported := hasState && strings.EqualFold(strings.TrimSpace(state.Status), "imported")
		if imported {
			summary.ImportedFiles++
			overview.Totals.ImportedFiles++
		} else {
			summary.PendingFiles++
			overview.Totals.PendingFiles++
			modTime := fileInfo.ModTime
			missing := SyncCoverageFileSummary{
				SourceName:    meta.OriginalName,
				DataType:      meta.DataType,
				DataReference: meta.DataReference,
				ModTime:       &modTime,
				SizeBytes:     fileInfo.Size,
				Imported:      false,
				Status:        "not_imported",
			}
			if hasState {
				missing.Status = firstNonEmpty(strings.TrimSpace(state.Status), "pending")
				missing.RecordCount = state.RecordCount
				missing.ImportedAt = state.ImportedAt
				missing.SourceKind = state.SourceKind
			}
			overview.MissingFiles = append(overview.MissingFiles, missing)
		}

		entityMap[meta.DataType] = summary
	}

	for _, dataType := range supportedDataTypes {
		overview.Entities = append(overview.Entities, entityMap[dataType])
	}

	sort.Slice(overview.MissingFiles, func(left int, right int) bool {
		if overview.MissingFiles[left].DataType == overview.MissingFiles[right].DataType {
			return overview.MissingFiles[left].DataReference.Before(overview.MissingFiles[right].DataReference)
		}
		return overview.MissingFiles[left].DataType < overview.MissingFiles[right].DataType
	})

	overview.FullyImported = overview.Totals.TotalFiles > 0 && overview.Totals.PendingFiles == 0
	return overview, nil
}

func (service *Service) CRMOverview(ctx context.Context, principal auth.Principal, query CRMOverviewQuery) (CRMOverviewResponse, error) {
	if !canViewERP(principal) {
		return CRMOverviewResponse{}, ErrForbidden
	}

	normalized, err := normalizeCRMOverviewQuery(query)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	store, err := service.resolveERPScope(ctx, principal, normalized.TenantID, normalized.StoreCode)
	if err != nil {
		return CRMOverviewResponse{}, err
	}

	var allowedStoreIDs []string
	if requiresStoreScopedFilter(principal.Role) {
		allowedStoreIDs = principal.StoreIDs
	}

	return service.repository.GetCRMOverview(ctx, store, normalized, allowedStoreIDs)
}

// GoalStatsByConsultant devolve o atingimento de meta do mes por consultor de
// PERFIL (chave do map = ProfileConsultantID), no MESMO escopo e janela do mes
// que a pagina de consultores usa, para o numero bater EXATAMENTE com o
// goalProgress do /v1/erp/crm.
//
// Diferente de CRMOverview, este metodo e uma PONTE server-side (enriquecimento do
// snapshot da Operacao): NAO passa pelo gate canViewERP — o principal injetado
// pela composition root ja garante o escopo do tenant. A decisao de produto e
// "todos os operadores veem a meta de todos", entao o gate de gestao nao se aplica
// aqui. O escopo continua tenant-wide (allowedStoreIDs=nil) quando o papel nao
// exige filtro por loja, espelhando o caminho da pagina.
//
// `month` no formato "YYYY-MM". Vazio => mes corrente (UTC).
func (service *Service) GoalStatsByConsultant(ctx context.Context, principal auth.Principal, tenantID string, month string) (map[string]ConsultantGoalStat, error) {
	dateFrom, dateTo, err := monthWindow(month)
	if err != nil {
		return nil, err
	}

	store, err := service.resolveERPScope(ctx, principal, tenantID, "")
	if err != nil {
		return nil, err
	}

	query, err := normalizeCRMOverviewQuery(CRMOverviewQuery{
		TenantID: tenantID,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		return nil, err
	}

	var allowedStoreIDs []string
	if requiresStoreScopedFilter(principal.Role) {
		allowedStoreIDs = principal.StoreIDs
	}

	overview, err := service.repository.GetCRMOverview(ctx, store, query, allowedStoreIDs)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]ConsultantGoalStat, len(overview.Consultants))
	for _, consultant := range overview.Consultants {
		profileID := strings.TrimSpace(consultant.ProfileConsultantID)
		if profileID == "" {
			continue
		}
		remainingCents := maxCRMRemaining(consultant.MonthlyGoalCents, consultant.SalesCents)
		stats[profileID] = ConsultantGoalStat{
			MonthlyGoal:     centsToReais(consultant.MonthlyGoalCents),
			SoldValue:       centsToReais(consultant.SalesCents),
			RemainingToGoal: centsToReais(remainingCents),
			Progress:        consultant.GoalProgress,
			HasGoal:         consultant.MonthlyGoalCents > 0,
		}
	}

	return stats, nil
}

// monthWindow converte "YYYY-MM" (ou vazio => mes corrente) na janela [primeiro
// dia, ultimo dia] do mes em UTC, date-only — IGUAL ao default da pagina de
// consultores (web/app/domain/utils/consultant-transforms.ts): dateFrom = dia 1,
// dateTo = ultimo dia do mes. Mesma janela => mesmo numero.
func monthWindow(month string) (time.Time, time.Time, error) {
	trimmed := strings.TrimSpace(month)
	var year int
	var mon time.Month
	if trimmed == "" {
		now := time.Now().UTC()
		year, mon = now.Year(), now.Month()
	} else {
		parsed, err := time.Parse("2006-01", trimmed)
		if err != nil {
			return time.Time{}, time.Time{}, ErrValidation
		}
		year, mon = parsed.Year(), parsed.Month()
	}

	dateFrom := time.Date(year, mon, 1, 0, 0, 0, 0, time.UTC)
	dateTo := dateFrom.AddDate(0, 1, -1)
	return dateFrom, dateTo, nil
}

func (service *Service) ConsultantERPLinks(ctx context.Context, principal auth.Principal, tenantID string, storeCode string, employeeIDs []string) (ConsultantERPLinksResponse, error) {
	if !canEditERP(principal) {
		return ConsultantERPLinksResponse{}, ErrForbidden
	}

	store, err := service.resolveERPScope(ctx, principal, tenantID, storeCode)
	if err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	return service.repository.ListConsultantERPLinks(ctx, store, employeeIDs)
}

func (service *Service) UpsertConsultantERPLink(ctx context.Context, principal auth.Principal, input ConsultantERPLinkUpsertInput) (ConsultantERPLinksResponse, error) {
	if !canEditERP(principal) {
		return ConsultantERPLinksResponse{}, ErrForbidden
	}

	store, err := service.resolveERPScope(ctx, principal, input.TenantID, input.StoreCode)
	if err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	if err := service.repository.UpsertConsultantERPLink(ctx, store, input, principal.UserID); err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	return service.repository.ListConsultantERPLinks(ctx, store, input.EmployeeIDs)
}

func (service *Service) AutoLinkConsultantERP(ctx context.Context, principal auth.Principal, tenantID string, storeCode string, employeeIDs []string) (ConsultantERPLinksResponse, error) {
	if !canEditERP(principal) {
		return ConsultantERPLinksResponse{}, ErrForbidden
	}

	store, err := service.resolveERPScope(ctx, principal, tenantID, storeCode)
	if err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	if err := service.repository.AutoLinkConsultantERP(ctx, store, principal.UserID, employeeIDs); err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	return service.repository.ListConsultantERPLinks(ctx, store, employeeIDs)
}

func (service *Service) DeleteConsultantERPLink(ctx context.Context, principal auth.Principal, input ConsultantERPLinkDeleteInput) (ConsultantERPLinksResponse, error) {
	if !canEditERP(principal) {
		return ConsultantERPLinksResponse{}, ErrForbidden
	}

	store, err := service.resolveERPScope(ctx, principal, input.TenantID, input.StoreCode)
	if err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	if err := service.repository.DeleteConsultantERPLink(ctx, store, input.LinkID, principal.UserID); err != nil {
		return ConsultantERPLinksResponse{}, err
	}

	return service.repository.ListConsultantERPLinks(ctx, store, input.EmployeeIDs)
}

func (service *Service) resolveERPScope(ctx context.Context, principal auth.Principal, tenantID string, requestedStoreCode string) (StoreScope, error) {
	normalizedStoreCode := strings.TrimSpace(requestedStoreCode)

	preferredStoreCode := strings.TrimSpace(service.options.RootStoreCode)
	if preferredStoreCode != "" {
		return service.repository.ResolveRootStoreScope(ctx, principal, tenantID, preferredStoreCode)
	}

	if normalizedStoreCode != "" {
		return service.repository.ResolveStoreScope(ctx, principal, tenantID, normalizedStoreCode)
	}

	return service.repository.ResolveDefaultERPScope(ctx, principal, tenantID)
}
