package erp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository *PostgresRepository
	options    Options
}

func NewService(repository *PostgresRepository, options Options) *Service {
	return &Service{repository: repository, options: options}
}

func (service *Service) Status(ctx context.Context, principal auth.Principal, tenantID string, storeCode string) (StatusResponse, error) {
	if !canViewERP(principal) {
		return StatusResponse{}, ErrForbidden
	}

	store, err := service.repository.ResolveStoreScope(ctx, principal, tenantID, storeCode)
	if err != nil {
		return StatusResponse{}, err
	}

	return service.repository.GetStatus(ctx, store)
}

func (service *Service) Products(ctx context.Context, principal auth.Principal, query ProductQuery) (ProductListResponse, error) {
	if !canViewERP(principal) {
		return ProductListResponse{}, ErrForbidden
	}

	store, err := service.repository.ResolveStoreScope(ctx, principal, query.TenantID, query.StoreCode)
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

	store, err := service.repository.ResolveStoreScope(ctx, principal, normalized.TenantID, normalized.StoreCode)
	if err != nil {
		return RawRecordsListResponse{}, err
	}

	return service.repository.ListRawRecords(ctx, store, normalized)
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

	store, err := service.repository.ResolveStoreScope(ctx, principal, input.TenantID, input.StoreCode)
	if err != nil {
		return BootstrapResult{}, err
	}

	sourcePath, err := service.resolveSourcePath(dataType, input.SourcePath)
	if err != nil {
		return BootstrapResult{}, err
	}

	run, err := service.repository.StartSyncRun(ctx, store, dataType, SyncModeBootstrapMarkdown, sourcePath)
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
