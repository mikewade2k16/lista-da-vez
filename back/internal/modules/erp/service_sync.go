package erp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

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

	return ItemBootstrapResult(result), nil
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
	if rootStoreCode := strings.TrimSpace(service.options.RootStoreCode); rootStoreCode != "" {
		rootStores := make([]StoreScope, 0, 1)
		for _, store := range stores {
			if strings.EqualFold(strings.TrimSpace(store.StoreCode), rootStoreCode) {
				rootStores = append(rootStores, store)
			}
		}
		if len(rootStores) == 0 {
			return nil, ErrStoreNotFound
		}
		stores = rootStores
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
	normalizedInput, err := service.normalizeIngestInput(ctx, store, input)
	if err != nil {
		return IngestResult{}, err
	}
	input = normalizedInput

	source, err := service.newSourceForIngest(input)
	if err != nil {
		return IngestResult{}, err
	}
	defer source.Close()

	listedFiles, err := source.List(ctx, store.StoreCode)
	if err != nil {
		return IngestResult{}, err
	}
	fileStates, err := service.repository.ListLatestSyncFileStates(ctx, store)
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

	triggeredBy := input.TriggeredBy
	for _, dataType := range normalizedTypes {
		candidates := grouped[dataType]
		candidates = filterSourceCandidatesForIngest(candidates, fileStates)
		if len(candidates) == 0 && strings.TrimSpace(input.DataType) == "" {
			continue
		}
		sortSourceCandidatesForIngest(candidates, fileStates, triggeredBy)
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

func (service *Service) newSource() (ErpSource, error) {
	return service.newSourceWithRecursive(service.options.SourceRecursive)
}

func (service *Service) newSourceForIngest(input IngestInput) (ErpSource, error) {
	recursive := service.options.SourceRecursive
	if input.TriggeredBy == SyncTriggeredByBackfill {
		recursive = true
	}
	return service.newSourceWithRecursive(recursive)
}

func (service *Service) newSourceWithRecursive(recursive bool) (ErpSource, error) {
	factory := service.sourceFactory
	if factory == nil {
		factory = NewSource
	}
	return factory(SourceOptions{
		Kind:               firstNonEmpty(service.options.SourceKind, SourceKindLocal),
		Recursive:          recursive,
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
	if err := service.validateCSVSize(candidate); err != nil {
		return nil, 0, err
	}

	reader, err := source.Open(ctx, candidate.info.Name)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	baseBatch := sourceBatchMetadata(candidate, source.Kind())
	maxBytes := service.options.CSVMaxBytes
	switch candidate.meta.DataType {
	case DataTypeItem:
		batch := itemConsolidatedBatch{Rows: make([]ItemRawRecord, 0, 256)}
		applySourceBatchMetadataToItem(&batch, baseBatch)
		checksum, rowCount, err := StreamCSVWithLimit(reader, candidate.meta.DataType, candidate.meta, maxBytes, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(ItemRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	case DataTypeCustomer:
		batch := customerConsolidatedBatch{Rows: make([]CustomerRawRecord, 0, 256)}
		applySourceBatchMetadataToCustomer(&batch, baseBatch)
		checksum, rowCount, err := StreamCSVWithLimit(reader, candidate.meta.DataType, candidate.meta, maxBytes, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(CustomerRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	case DataTypeEmployee:
		batch := employeeConsolidatedBatch{Rows: make([]EmployeeRawRecord, 0, 128)}
		applySourceBatchMetadataToEmployee(&batch, baseBatch)
		checksum, rowCount, err := StreamCSVWithLimit(reader, candidate.meta.DataType, candidate.meta, maxBytes, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(EmployeeRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	case DataTypeOrder, DataTypeOrderCanceled:
		batch := orderConsolidatedBatch{Rows: make([]OrderRawRecord, 0, 256)}
		applySourceBatchMetadataToOrder(&batch, baseBatch)
		checksum, rowCount, err := StreamCSVWithLimit(reader, candidate.meta.DataType, candidate.meta, maxBytes, func(idx int, record any) error {
			batch.Rows = append(batch.Rows, record.(OrderRawRecord))
			return nil
		})
		batch.ChecksumSHA256 = checksum
		return batch, rowCount, err
	default:
		return nil, 0, ErrUnsupportedDataType
	}
}

func (service *Service) validateCSVSize(candidate sourceCandidate) error {
	maxBytes := service.options.CSVMaxBytes
	if maxBytes <= 0 || candidate.info.Size <= 0 || candidate.info.Size <= maxBytes {
		return nil
	}
	return &ErrCSVTooLarge{
		SourceName: candidate.meta.OriginalName,
		MaxBytes:   maxBytes,
		GotBytes:   candidate.info.Size,
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

func (service *Service) normalizeIngestInput(ctx context.Context, store StoreScope, input IngestInput) (IngestInput, error) {
	if input.MaxFiles < 0 {
		return IngestInput{}, ErrValidation
	}

	triggeredBy, err := normalizeSyncTriggeredBy(input.TriggeredBy)
	if err != nil {
		return IngestInput{}, err
	}
	input.TriggeredBy = triggeredBy

	switch triggeredBy {
	case SyncTriggeredByManual:
		if input.MaxFiles == 0 {
			input.MaxFiles = service.options.ManualSyncMaxFiles
		}
		if input.MaxFiles > service.options.ManualSyncMaxFiles {
			return IngestInput{}, ErrValidation
		}
	case SyncTriggeredByBackfill:
		if input.MaxFiles == 0 {
			input.MaxFiles = service.options.BackfillMaxFiles
		}
		if input.MaxFiles > service.options.BackfillMaxFiles {
			return IngestInput{}, ErrValidation
		}
	}

	if triggeredBy == SyncTriggeredByManual || triggeredBy == SyncTriggeredByBackfill {
		running, err := service.repository.HasRunningCSVSyncRun(ctx, store)
		if err != nil {
			return IngestInput{}, err
		}
		if running {
			return IngestInput{}, ErrSyncAlreadyRunning
		}

		recent, err := service.repository.HasRecentCSVSyncRun(ctx, store, time.Now().UTC().Add(-service.options.ManualSyncMinInterval))
		if err != nil {
			return IngestInput{}, err
		}
		if recent {
			return IngestInput{}, ErrSyncRateLimited
		}
	}

	return input, nil
}

func normalizeSyncTriggeredBy(raw string) (string, error) {
	normalized := firstNonEmpty(strings.TrimSpace(strings.ToLower(raw)), SyncTriggeredByManual)
	switch normalized {
	case SyncTriggeredByManual, SyncTriggeredByCron, SyncTriggeredByBackfill:
		return normalized, nil
	default:
		return "", ErrValidation
	}
}
