package erp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository    *PostgresRepository
	options       Options
	sourceFactory func(SourceOptions) (ErpSource, error)
}

func NewService(repository *PostgresRepository, options Options) *Service {
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
	if err != nil {
		return SyncOverviewResponse{}, err
	}

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

func (service *Service) BootstrapItems(ctx context.Context, principal auth.Principal, input ItemBootstrapInput) (ItemBootstrapResult, error) {
	result, err := service.Bootstrap(ctx, principal, BootstrapInput{
		TenantID:   input.TenantID,
		StoreCode:  input.StoreCode,
		DataType:   DataTypeItem,
		SourcePath: input.SourcePath,
	})
	if err != nil {
		return ItemBootstrapResult{}, err
	}

	return ItemBootstrapResult{
		OK:            result.OK,
		RunID:         result.RunID,
		Store:         result.Store,
		DataType:      result.DataType,
		SourcePath:    result.SourcePath,
		FilesSeen:     result.FilesSeen,
		FilesImported: result.FilesImported,
		FilesSkipped:  result.FilesSkipped,
		RowsRead:      result.RowsRead,
		RowsImported:  result.RowsImported,
		StartedAt:     result.StartedAt,
		FinishedAt:    result.FinishedAt,
		StoreCNPJ:     result.StoreCNPJ,
	}, nil
}

func (service *Service) Bootstrap(ctx context.Context, principal auth.Principal, input BootstrapInput) (BootstrapResult, error) {
	if !canEditERP(principal) {
		return BootstrapResult{}, ErrForbidden
	}
	if !service.manualSyncAllowed() {
		return BootstrapResult{}, ErrManualSyncDisabled
	}

	dataType := strings.TrimSpace(strings.ToLower(input.DataType))
	if dataType == "" {
		dataType = DataTypeItem
	}
	if !isSupportedDataType(dataType) {
		return BootstrapResult{}, ErrUnsupportedDataType
	}

	store, err := service.resolveERPScope(ctx, principal, input.TenantID, input.StoreCode)
	if err != nil {
		return BootstrapResult{}, err
	}

	sourcePath, err := service.resolveSourcePath(dataType, input.SourcePath)
	if err != nil {
		return BootstrapResult{}, err
	}

	run, err := service.repository.StartSyncRun(ctx, store, dataType, SyncModeBootstrapMarkdown, sourcePath, SyncTriggeredByManual)
	if err != nil {
		return BootstrapResult{}, err
	}

	filesSeen := 0
	filesImported := 0
	filesSkipped := 0
	rowsRead := 0
	rowsImported := 0
	storeCNPJ := strings.TrimSpace(store.StoreCNPJ)
	finishedAt := time.Now().UTC()

	streamErr := service.streamAndImport(ctx, dataType, sourcePath, run.ID, store, &filesSeen, &filesImported, &filesSkipped, &rowsRead, &rowsImported, &storeCNPJ)
	finishedAt = time.Now().UTC()

	if streamErr != nil {
		_ = service.repository.FinishSyncRun(
			ctx,
			run.ID,
			SyncStatusFailed,
			filesSeen,
			filesImported,
			filesSkipped,
			rowsRead,
			rowsImported,
			storeCNPJ,
			finishedAt,
			streamErr.Error(),
		)
		return BootstrapResult{}, streamErr
	}

	if err := service.repository.FinishSyncRun(
		ctx,
		run.ID,
		SyncStatusSucceeded,
		filesSeen,
		filesImported,
		filesSkipped,
		rowsRead,
		rowsImported,
		storeCNPJ,
		finishedAt,
		"",
	); err != nil {
		return BootstrapResult{}, err
	}

	store.StoreCNPJ = firstNonEmpty(store.StoreCNPJ, storeCNPJ)
	return BootstrapResult{
		OK:            true,
		RunID:         run.ID,
		Store:         store,
		DataType:      dataType,
		SourcePath:    sourcePath,
		FilesSeen:     filesSeen,
		FilesImported: filesImported,
		FilesSkipped:  filesSkipped,
		RowsRead:      rowsRead,
		RowsImported:  rowsImported,
		StartedAt:     run.StartedAt,
		FinishedAt:    finishedAt,
		StoreCNPJ:     store.StoreCNPJ,
	}, nil
}

func (service *Service) IngestStore(ctx context.Context, principal auth.Principal, input IngestInput) (IngestResult, error) {
	if !canEditERP(principal) {
		return IngestResult{}, ErrForbidden
	}
	if !service.manualSyncAllowed() {
		return IngestResult{}, ErrManualSyncDisabled
	}

	store, err := service.resolveERPScope(ctx, principal, input.TenantID, input.StoreCode)
	if err != nil {
		return IngestResult{}, err
	}

	return service.ingestStoreResolved(ctx, store, input)
}

func (service *Service) IngestAllStores(ctx context.Context, input IngestInput) ([]IngestResult, error) {
	stores, err := service.repository.ListActiveStores(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]IngestResult, 0, len(stores))
	for _, store := range stores {
		scopedInput := input
		scopedInput.TenantID = store.TenantID
		scopedInput.StoreCode = store.StoreCode

		result, err := service.ingestStoreResolved(ctx, store, scopedInput)
		if err != nil {
			return results, err
		}
		if result.RunID == "" && len(result.RunIDs) == 0 && result.FilesSeen == 0 && len(result.FilesFailed) == 0 {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

func (service *Service) ingestStoreResolved(ctx context.Context, store StoreScope, input IngestInput) (IngestResult, error) {
	normalizedTypes, err := normalizeIngestDataTypes(input.DataType)
	if err != nil {
		return IngestResult{}, err
	}

	source, err := service.newSource()
	if err != nil {
		return IngestResult{}, err
	}
	defer source.Close()

	listedFiles, err := source.List(ctx, store.StoreCode)
	if err != nil {
		return IngestResult{}, err
	}

	selectedTypeSet := make(map[string]struct{}, len(normalizedTypes))
	for _, dataType := range normalizedTypes {
		selectedTypeSet[dataType] = struct{}{}
	}

	grouped := make(map[string][]sourceCandidate, len(normalizedTypes))
	for _, fileInfo := range listedFiles {
		meta, parseErr := parseCSVFilename(filepath.Base(fileInfo.Name))
		if parseErr != nil {
			continue
		}
		if _, ok := selectedTypeSet[meta.DataType]; !ok {
			continue
		}
		grouped[meta.DataType] = append(grouped[meta.DataType], sourceCandidate{info: fileInfo, meta: meta})
	}

	result := IngestResult{
		OK:        true,
		Store:     store,
		DataType:  strings.TrimSpace(strings.ToLower(input.DataType)),
		DataTypes: append([]string{}, normalizedTypes...),
		DryRun:    input.DryRun,
		StartedAt: time.Now().UTC(),
		StoreCNPJ: strings.TrimSpace(store.StoreCNPJ),
	}
	if len(normalizedTypes) == 1 {
		result.DataType = normalizedTypes[0]
	}

	triggeredBy := firstNonEmpty(strings.TrimSpace(strings.ToLower(input.TriggeredBy)), SyncTriggeredByManual)
	for _, dataType := range normalizedTypes {
		candidates := grouped[dataType]
		if len(candidates) == 0 && strings.TrimSpace(input.DataType) == "" {
			continue
		}
		sort.Slice(candidates, func(left int, right int) bool {
			leftExtractedAt := effectiveSourceExtractedAt(candidates[left])
			rightExtractedAt := effectiveSourceExtractedAt(candidates[right])
			if leftExtractedAt.Equal(rightExtractedAt) {
				return candidates[left].info.Name < candidates[right].info.Name
			}
			return leftExtractedAt.Before(rightExtractedAt)
		})
		if input.MaxFiles > 0 && len(candidates) > input.MaxFiles {
			candidates = candidates[:input.MaxFiles]
		}

		run, err := service.repository.StartSyncRun(ctx, store, dataType, SyncModeCSVFTP, service.describeSource(source), triggeredBy)
		if err != nil {
			return IngestResult{}, err
		}
		if result.RunID == "" {
			result.RunID = run.ID
			result.StartedAt = run.StartedAt
		}
		result.RunIDs = append(result.RunIDs, run.ID)

		runFilesSeen := 0
		runFilesImported := 0
		runFilesSkipped := 0
		runRowsRead := 0
		runRowsImported := 0
		runStoreCNPJ := result.StoreCNPJ
		runFailures := make([]FileFailure, 0)

		for _, candidate := range candidates {
			runFilesSeen++
			result.FilesSeen++

			batch, rowCount, batchErr := service.loadCSVBatch(ctx, source, candidate)
			runRowsRead += rowCount
			result.RowsRead += rowCount
			if batchErr != nil {
				failure := FileFailure{SourceName: candidate.info.Name, Message: batchErr.Error()}
				runFailures = append(runFailures, failure)
				result.FilesFailed = append(result.FilesFailed, failure)
				result.OK = false
				_ = service.repository.UpdateSyncRunProgress(ctx, run.ID, runFilesSeen, runFilesImported, runFilesSkipped, runRowsRead, runRowsImported, runStoreCNPJ)
				continue
			}

			checksum := batchChecksum(batch)
			sourceName := batchSourceName(batch)
			if input.DryRun {
				exists, existsErr := service.repository.SyncFileExists(ctx, store, dataType, sourceName, checksum)
				if existsErr != nil {
					failure := FileFailure{SourceName: candidate.info.Name, Message: existsErr.Error()}
					runFailures = append(runFailures, failure)
					result.FilesFailed = append(result.FilesFailed, failure)
					result.OK = false
				} else if exists {
					runFilesSkipped++
					result.FilesSkipped++
				} else {
					runFilesImported++
					runRowsImported += rowCount
					result.FilesImported++
					result.RowsImported += rowCount
					if result.StoreCNPJ == "" {
						result.StoreCNPJ = batchStoreCNPJ(batch)
					}
				}
				_ = service.repository.UpdateSyncRunProgress(ctx, run.ID, runFilesSeen, runFilesImported, runFilesSkipped, runRowsRead, runRowsImported, runStoreCNPJ)
				continue
			}

			importResult, importErr := service.importBatch(ctx, run.ID, store, dataType, batch, time.Now().UTC())
			if importErr != nil {
				failure := FileFailure{SourceName: candidate.info.Name, Message: importErr.Error()}
				runFailures = append(runFailures, failure)
				result.FilesFailed = append(result.FilesFailed, failure)
				result.OK = false
			} else if importResult.Imported {
				runFilesImported++
				runRowsImported += importResult.Rows
				result.FilesImported++
				result.RowsImported += importResult.Rows
				runStoreCNPJ = firstNonEmpty(runStoreCNPJ, importResult.StoreCNPJ)
				result.StoreCNPJ = firstNonEmpty(result.StoreCNPJ, importResult.StoreCNPJ)
			} else {
				runFilesSkipped++
				result.FilesSkipped++
			}

			_ = service.repository.UpdateSyncRunProgress(ctx, run.ID, runFilesSeen, runFilesImported, runFilesSkipped, runRowsRead, runRowsImported, runStoreCNPJ)
		}

		status := SyncStatusSucceeded
		errorMessage := ""
		if len(runFailures) > 0 {
			status = SyncStatusFailed
			errorMessage = runFailures[0].Message
		}
		finishedAt := time.Now().UTC()
		if err := service.repository.FinishSyncRun(ctx, run.ID, status, runFilesSeen, runFilesImported, runFilesSkipped, runRowsRead, runRowsImported, runStoreCNPJ, finishedAt, errorMessage); err != nil {
			return IngestResult{}, err
		}
		result.FinishedAt = finishedAt
	}

	if result.FinishedAt.IsZero() {
		result.FinishedAt = time.Now().UTC()
	}
	result.Duration = result.FinishedAt.Sub(result.StartedAt).String()
	return result, nil
}

func (service *Service) resolveERPScope(ctx context.Context, principal auth.Principal, tenantID string, requestedStoreCode string) (StoreScope, error) {
	normalizedStoreCode := strings.TrimSpace(requestedStoreCode)
	if normalizedStoreCode != "" {
		return service.repository.ResolveStoreScope(ctx, principal, tenantID, normalizedStoreCode)
	}

	preferredStoreCode := strings.TrimSpace(service.options.RootStoreCode)
	if preferredStoreCode != "" {
		store, err := service.repository.ResolveStoreScope(ctx, principal, tenantID, preferredStoreCode)
		if err == nil {
			return store, nil
		}
		if !errors.Is(err, ErrStoreNotFound) && !errors.Is(err, ErrForbidden) {
			return StoreScope{}, err
		}
	}

	return service.repository.ResolveDefaultERPScope(ctx, principal, tenantID)
}

func (service *Service) streamAndImport(ctx context.Context, dataType string, sourcePath string, runID string, store StoreScope, filesSeen *int, filesImported *int, filesSkipped *int, rowsRead *int, rowsImported *int, storeCNPJ *string) error {
	validateStore := func(batchStore string) error {
		if strings.TrimSpace(batchStore) != strings.TrimSpace(store.StoreCode) {
			return fmt.Errorf("%w: consolidado da loja %s nao confere com a loja solicitada %s", ErrValidation, batchStore, store.StoreCode)
		}
		return nil
	}

	switch dataType {
	case DataTypeItem:
		return StreamItemConsolidated(sourcePath, func(batch itemConsolidatedBatch) error {
			*filesSeen = *filesSeen + 1
			if err := validateStore(batch.StoreCode); err != nil {
				return err
			}
			*rowsRead += len(batch.Rows)
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(batch.StoreCNPJ)
			}
			result, err := service.repository.ImportItemBatch(ctx, itemBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: batch, ImportedAt: time.Now().UTC()})
			if err != nil {
				return err
			}
			if result.Imported {
				*filesImported = *filesImported + 1
				*rowsImported += result.Rows
			} else {
				*filesSkipped = *filesSkipped + 1
			}
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(result.StoreCNPJ)
			}
			return nil
		})
	case DataTypeCustomer:
		return StreamCustomerConsolidated(sourcePath, func(batch customerConsolidatedBatch) error {
			*filesSeen = *filesSeen + 1
			if err := validateStore(batch.StoreCode); err != nil {
				return err
			}
			*rowsRead += len(batch.Rows)
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(batch.StoreCNPJ)
			}
			result, err := service.repository.ImportCustomerBatch(ctx, customerBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: batch, ImportedAt: time.Now().UTC()})
			if err != nil {
				return err
			}
			if result.Imported {
				*filesImported = *filesImported + 1
				*rowsImported += result.Rows
			} else {
				*filesSkipped = *filesSkipped + 1
			}
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(result.StoreCNPJ)
			}
			return nil
		})
	case DataTypeEmployee:
		return StreamEmployeeConsolidated(sourcePath, func(batch employeeConsolidatedBatch) error {
			*filesSeen = *filesSeen + 1
			if err := validateStore(batch.StoreCode); err != nil {
				return err
			}
			*rowsRead += len(batch.Rows)
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(batch.StoreCNPJ)
			}
			result, err := service.repository.ImportEmployeeBatch(ctx, employeeBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: batch, ImportedAt: time.Now().UTC()})
			if err != nil {
				return err
			}
			if result.Imported {
				*filesImported = *filesImported + 1
				*rowsImported += result.Rows
			} else {
				*filesSkipped = *filesSkipped + 1
			}
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(result.StoreCNPJ)
			}
			return nil
		})
	case DataTypeOrder, DataTypeOrderCanceled:
		return StreamOrderConsolidated(sourcePath, dataType, func(batch orderConsolidatedBatch) error {
			*filesSeen = *filesSeen + 1
			if err := validateStore(batch.StoreCode); err != nil {
				return err
			}
			*rowsRead += len(batch.Rows)
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(batch.StoreCNPJ)
			}
			result, err := service.repository.ImportOrderBatch(ctx, orderBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: batch, ImportedAt: time.Now().UTC()})
			if err != nil {
				return err
			}
			if result.Imported {
				*filesImported = *filesImported + 1
				*rowsImported += result.Rows
			} else {
				*filesSkipped = *filesSkipped + 1
			}
			if *storeCNPJ == "" {
				*storeCNPJ = strings.TrimSpace(result.StoreCNPJ)
			}
			return nil
		})
	default:
		return ErrUnsupportedDataType
	}
}

func normalizeProductQuery(query ProductQuery) ProductQuery {
	normalized := ProductQuery{
		TenantID:         strings.TrimSpace(query.TenantID),
		StoreCode:        strings.TrimSpace(query.StoreCode),
		IdentifierPrefix: strings.TrimSpace(query.IdentifierPrefix),
		Search:           strings.TrimSpace(query.Search),
		Page:             query.Page,
		PageSize:         query.PageSize,
	}
	if normalized.Page <= 0 {
		normalized.Page = 1
	}
	if normalized.PageSize <= 0 {
		normalized.PageSize = defaultPageSize
	}
	if normalized.PageSize > maxPageSize {
		normalized.PageSize = maxPageSize
	}
	return normalized
}

func normalizeRawRecordsQuery(query RawRecordsQuery) RawRecordsQuery {
	normalized := RawRecordsQuery{
		TenantID:       strings.TrimSpace(query.TenantID),
		StoreCode:      strings.TrimSpace(query.StoreCode),
		DataType:       strings.TrimSpace(strings.ToLower(query.DataType)),
		Search:         strings.TrimSpace(query.Search),
		SpecificSearch: strings.TrimSpace(query.SpecificSearch),
		Page:           query.Page,
		PageSize:       query.PageSize,
	}
	if normalized.Page <= 0 {
		normalized.Page = 1
	}
	if normalized.PageSize <= 0 {
		normalized.PageSize = defaultPageSize
	}
	if normalized.PageSize > maxPageSize {
		normalized.PageSize = maxPageSize
	}
	return normalized
}

func normalizeRunsQuery(query RunsQuery) RunsQuery {
	normalized := RunsQuery{
		TenantID:  strings.TrimSpace(query.TenantID),
		StoreCode: strings.TrimSpace(query.StoreCode),
		DataType:  strings.TrimSpace(strings.ToLower(query.DataType)),
		Page:      query.Page,
		PageSize:  query.PageSize,
	}
	if normalized.Page <= 0 {
		normalized.Page = 1
	}
	if normalized.PageSize <= 0 {
		normalized.PageSize = defaultPageSize
	}
	if normalized.PageSize > maxPageSize {
		normalized.PageSize = maxPageSize
	}
	return normalized
}

func (service *Service) manualSyncAllowed() bool {
	if service.options.AllowManualSync {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(service.options.Env), "development")
}

func (service *Service) resolveSourcePath(dataType string, requestedPath string) (string, error) {
	trimmedRequested := strings.TrimSpace(requestedPath)
	trimmedRoot := strings.TrimSpace(service.options.SourceDir)

	var defaultRelative string
	switch dataType {
	case DataTypeItem:
		defaultRelative = strings.TrimSpace(service.options.BootstrapItemFile)
	case DataTypeCustomer:
		defaultRelative = strings.TrimSpace(service.options.BootstrapCustomerFile)
	case DataTypeEmployee:
		defaultRelative = strings.TrimSpace(service.options.BootstrapEmployeeFile)
	case DataTypeOrder:
		defaultRelative = strings.TrimSpace(service.options.BootstrapOrderFile)
	case DataTypeOrderCanceled:
		defaultRelative = strings.TrimSpace(service.options.BootstrapOrderCanceledFile)
	default:
		return "", ErrUnsupportedDataType
	}

	if trimmedRequested == "" {
		trimmedRequested = defaultRelative
	}
	if trimmedRequested == "" {
		return "", ErrSourceNotConfigured
	}

	var candidate string
	if filepath.IsAbs(trimmedRequested) {
		candidate = trimmedRequested
	} else {
		if trimmedRoot == "" {
			return "", ErrSourceNotConfigured
		}
		candidate = filepath.Join(trimmedRoot, trimmedRequested)
	}

	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if trimmedRoot != "" {
		absRoot, err := filepath.Abs(trimmedRoot)
		if err != nil {
			return "", err
		}
		if !isPathInside(absRoot, absCandidate) {
			return "", ErrSourcePathOutsideRoot
		}
	}
	if _, err := os.Stat(absCandidate); err != nil {
		return "", err
	}
	return absCandidate, nil
}

func isSupportedDataType(dataType string) bool {
	for _, value := range supportedDataTypes {
		if value == dataType {
			return true
		}
	}
	return false
}

func normalizeIngestDataTypes(raw string) ([]string, error) {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "" {
		return append([]string{}, supportedDataTypes...), nil
	}
	if !isSupportedDataType(normalized) {
		return nil, ErrUnsupportedDataType
	}
	return []string{normalized}, nil
}

func (service *Service) newSource() (ErpSource, error) {
	factory := service.sourceFactory
	if factory == nil {
		factory = NewSource
	}
	return factory(SourceOptions{
		Kind:               firstNonEmpty(service.options.SourceKind, SourceKindLocal),
		Recursive:          service.options.SourceRecursive,
		Environment:        service.options.Env,
		LocalDir:           service.options.SourceDir,
		Host:               service.options.FTPHost,
		Port:               service.options.FTPPort,
		Username:           service.options.FTPUser,
		Password:           service.options.FTPPassword,
		KeyPath:            service.options.FTPKeyPath,
		RemoteDir:          service.options.FTPRemoteDir,
		HostKeyFingerprint: service.options.FTPHostKey,
	})
}

func (service *Service) describeSource(source ErpSource) string {
	if source == nil {
		return ""
	}
	switch source.Kind() {
	case SourceKindLocal:
		return service.options.SourceDir
	case SourceKindFTP, SourceKindSFTP, SourceKindFTPS:
		return service.options.FTPRemoteDir
	default:
		return source.Kind()
	}
}

func (service *Service) loadCSVBatch(ctx context.Context, source ErpSource, candidate sourceCandidate) (any, int, error) {
	reader, err := source.Open(ctx, candidate.info.Name)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	baseBatch := sourceBatchMetadata(candidate, source.Kind())
	switch candidate.meta.DataType {
	case DataTypeItem:
		batch := itemConsolidatedBatch{Rows: make([]ItemRawRecord, 0, 256)}
		applySourceBatchMetadataToItem(&batch, baseBatch)
		checksum, rowCount, err := StreamCSV(reader, candidate.meta.DataType, candidate.meta, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(ItemRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	case DataTypeCustomer:
		batch := customerConsolidatedBatch{Rows: make([]CustomerRawRecord, 0, 256)}
		applySourceBatchMetadataToCustomer(&batch, baseBatch)
		checksum, rowCount, err := StreamCSV(reader, candidate.meta.DataType, candidate.meta, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(CustomerRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	case DataTypeEmployee:
		batch := employeeConsolidatedBatch{Rows: make([]EmployeeRawRecord, 0, 128)}
		applySourceBatchMetadataToEmployee(&batch, baseBatch)
		checksum, rowCount, err := StreamCSV(reader, candidate.meta.DataType, candidate.meta, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(EmployeeRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	case DataTypeOrder, DataTypeOrderCanceled:
		batch := orderConsolidatedBatch{Rows: make([]OrderRawRecord, 0, 256)}
		applySourceBatchMetadataToOrder(&batch, baseBatch)
		checksum, rowCount, err := StreamCSV(reader, candidate.meta.DataType, candidate.meta, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(OrderRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	default:
		return nil, 0, ErrUnsupportedDataType
	}
}

func (service *Service) importBatch(ctx context.Context, runID string, store StoreScope, dataType string, batch any, importedAt time.Time) (itemBatchImportResult, error) {
	switch typed := batch.(type) {
	case itemConsolidatedBatch:
		return service.repository.ImportItemBatch(ctx, itemBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: typed, ImportedAt: importedAt})
	case customerConsolidatedBatch:
		return service.repository.ImportCustomerBatch(ctx, customerBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: typed, ImportedAt: importedAt})
	case employeeConsolidatedBatch:
		return service.repository.ImportEmployeeBatch(ctx, employeeBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: typed, ImportedAt: importedAt})
	case orderConsolidatedBatch:
		return service.repository.ImportOrderBatch(ctx, orderBatchImportInput{RunID: runID, Store: store, DataType: dataType, Batch: typed, ImportedAt: importedAt})
	default:
		return itemBatchImportResult{}, ErrUnsupportedDataType
	}
}

type sourceCandidate struct {
	info SourceFileInfo
	meta csvFileMetadata
}

type sourceBatchMetadataInput struct {
	DataType            string
	StoreCode           string
	StoreCNPJ           string
	SourceFileName      string
	SourcePath          string
	SourceKind          string
	BatchDate           string
	SourceExtractedAt   *time.Time
	SourceDataReference *time.Time
	SourceSizeBytes     int64
}

func sourceBatchMetadata(candidate sourceCandidate, sourceKind string) sourceBatchMetadataInput {
	return sourceBatchMetadataInput{
		DataType:            candidate.meta.DataType,
		StoreCode:           candidate.meta.StoreCode,
		StoreCNPJ:           candidate.meta.StoreCNPJ,
		SourceFileName:      candidate.meta.OriginalName,
		SourcePath:          candidate.info.Name,
		SourceKind:          sourceKind,
		BatchDate:           formatCSVBatchDate(candidate.meta),
		SourceExtractedAt:   optionalTime(effectiveSourceExtractedAt(candidate)),
		SourceDataReference: optionalTime(candidate.meta.DataReference),
		SourceSizeBytes:     candidate.info.Size,
	}
}

func effectiveSourceExtractedAt(candidate sourceCandidate) time.Time {
	if !candidate.meta.ExtractedAt.IsZero() {
		return candidate.meta.ExtractedAt.UTC()
	}
	if !candidate.info.ModTime.IsZero() {
		return candidate.info.ModTime.UTC()
	}
	return time.Time{}
}

func applySourceBatchMetadataToItem(batch *itemConsolidatedBatch, meta sourceBatchMetadataInput) {
	batch.DataType = meta.DataType
	batch.StoreCode = meta.StoreCode
	batch.StoreCNPJ = meta.StoreCNPJ
	batch.SourceFileName = meta.SourceFileName
	batch.SourcePath = meta.SourcePath
	batch.SourceKind = meta.SourceKind
	batch.BatchDate = meta.BatchDate
	batch.SourceExtractedAt = meta.SourceExtractedAt
	batch.SourceDataReference = meta.SourceDataReference
	batch.SourceSizeBytes = meta.SourceSizeBytes
}

func applySourceBatchMetadataToCustomer(batch *customerConsolidatedBatch, meta sourceBatchMetadataInput) {
	batch.DataType = meta.DataType
	batch.StoreCode = meta.StoreCode
	batch.StoreCNPJ = meta.StoreCNPJ
	batch.SourceFileName = meta.SourceFileName
	batch.SourcePath = meta.SourcePath
	batch.SourceKind = meta.SourceKind
	batch.BatchDate = meta.BatchDate
	batch.SourceExtractedAt = meta.SourceExtractedAt
	batch.SourceDataReference = meta.SourceDataReference
	batch.SourceSizeBytes = meta.SourceSizeBytes
}

func applySourceBatchMetadataToEmployee(batch *employeeConsolidatedBatch, meta sourceBatchMetadataInput) {
	batch.DataType = meta.DataType
	batch.StoreCode = meta.StoreCode
	batch.StoreCNPJ = meta.StoreCNPJ
	batch.SourceFileName = meta.SourceFileName
	batch.SourcePath = meta.SourcePath
	batch.SourceKind = meta.SourceKind
	batch.BatchDate = meta.BatchDate
	batch.SourceExtractedAt = meta.SourceExtractedAt
	batch.SourceDataReference = meta.SourceDataReference
	batch.SourceSizeBytes = meta.SourceSizeBytes
}

func applySourceBatchMetadataToOrder(batch *orderConsolidatedBatch, meta sourceBatchMetadataInput) {
	batch.DataType = meta.DataType
	batch.StoreCode = meta.StoreCode
	batch.StoreCNPJ = meta.StoreCNPJ
	batch.SourceFileName = meta.SourceFileName
	batch.SourcePath = meta.SourcePath
	batch.SourceKind = meta.SourceKind
	batch.BatchDate = meta.BatchDate
	batch.SourceExtractedAt = meta.SourceExtractedAt
	batch.SourceDataReference = meta.SourceDataReference
	batch.SourceSizeBytes = meta.SourceSizeBytes
}

func batchChecksum(batch any) string {
	switch typed := batch.(type) {
	case itemConsolidatedBatch:
		return typed.ChecksumSHA256
	case customerConsolidatedBatch:
		return typed.ChecksumSHA256
	case employeeConsolidatedBatch:
		return typed.ChecksumSHA256
	case orderConsolidatedBatch:
		return typed.ChecksumSHA256
	default:
		return ""
	}
}

func batchSourceName(batch any) string {
	switch typed := batch.(type) {
	case itemConsolidatedBatch:
		return typed.SourceFileName
	case customerConsolidatedBatch:
		return typed.SourceFileName
	case employeeConsolidatedBatch:
		return typed.SourceFileName
	case orderConsolidatedBatch:
		return typed.SourceFileName
	default:
		return ""
	}
}

func batchStoreCNPJ(batch any) string {
	switch typed := batch.(type) {
	case itemConsolidatedBatch:
		return typed.StoreCNPJ
	case customerConsolidatedBatch:
		return typed.StoreCNPJ
	case employeeConsolidatedBatch:
		return typed.StoreCNPJ
	case orderConsolidatedBatch:
		return typed.StoreCNPJ
	default:
		return ""
	}
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func isPathInside(root string, candidate string) bool {
	cleanRoot := filepath.Clean(root)
	cleanCandidate := filepath.Clean(candidate)
	if cleanRoot == cleanCandidate {
		return true
	}
	rootWithSep := cleanRoot + string(os.PathSeparator)
	return strings.HasPrefix(cleanCandidate, rootWithSep)
}

func canViewERP(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionERPView)
	}

	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner, auth.RoleMarketing, auth.RoleDirector, auth.RoleManager:
		return true
	default:
		return false
	}
}

func canEditERP(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionERPEdit)
	}

	switch principal.Role {
	case auth.RolePlatformAdmin, auth.RoleOwner:
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type syncRunStart struct {
	ID        string
	StartedAt time.Time
}
