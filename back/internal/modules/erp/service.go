package erp

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

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
	if !canViewERP(principal) {
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
	if !canViewERP(principal) {
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
	if !canViewERP(principal) {
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
	if !canViewERP(principal) {
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

	return service.repository.GetCRMOverview(ctx, store, normalized)
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
