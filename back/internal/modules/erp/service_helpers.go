package erp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

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

type syncRunStart struct {
	ID        string
	StartedAt time.Time
}

func normalizeProductQuery(query ProductQuery) ProductQuery {
	normalized := ProductQuery{
		TenantID:         strings.TrimSpace(query.TenantID),
		StoreCode:        strings.TrimSpace(query.StoreCode),
		IdentifierPrefix: strings.TrimSpace(query.IdentifierPrefix),
		Search:           strings.TrimSpace(query.Search),
		Page:             query.Page,
		PageSize:         query.PageSize,
		SortBy:           strings.TrimSpace(query.SortBy),
		SortDir:          strings.TrimSpace(strings.ToLower(query.SortDir)),
		DateFrom:         strings.TrimSpace(query.DateFrom),
		DateTo:           strings.TrimSpace(query.DateTo),
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
		Dedup:          query.Dedup,
		SortBy:         strings.TrimSpace(query.SortBy),
		SortDir:        strings.TrimSpace(strings.ToLower(query.SortDir)),
		DateFrom:       strings.TrimSpace(query.DateFrom),
		DateTo:         strings.TrimSpace(query.DateTo),
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

func normalizeCRMOverviewQuery(query CRMOverviewQuery) (CRMOverviewQuery, error) {
	normalized := CRMOverviewQuery{
		TenantID:        strings.TrimSpace(query.TenantID),
		StoreCode:       strings.TrimSpace(query.StoreCode),
		DateFrom:        query.DateFrom.UTC(),
		DateTo:          query.DateTo.UTC(),
		DateFromHasTime: query.DateFromHasTime,
		DateToHasTime:   query.DateToHasTime,
	}

	if !normalized.DateFrom.IsZero() && !normalized.DateFromHasTime {
		normalized.DateFrom = time.Date(normalized.DateFrom.Year(), normalized.DateFrom.Month(), normalized.DateFrom.Day(), 0, 0, 0, 0, time.UTC)
	}
	if !normalized.DateTo.IsZero() && !normalized.DateToHasTime {
		normalized.DateTo = time.Date(normalized.DateTo.Year(), normalized.DateTo.Month(), normalized.DateTo.Day(), 0, 0, 0, 0, time.UTC)
	}
	if !normalized.DateFrom.IsZero() && !normalized.DateTo.IsZero() && normalized.DateTo.Before(normalized.DateFrom) {
		return CRMOverviewQuery{}, ErrValidation
	}

	return normalized, nil
}

func isSupportedDataType(dataType string) bool {
	for _, value := range supportedDataTypes {
		if value == dataType {
			return true
		}
	}
	return false
}

func sortSourceCandidatesForIngest(candidates []sourceCandidate, fileStates map[string]syncFileImportState, triggeredBy string) {
	sort.Slice(candidates, func(left int, right int) bool {
		leftImported := sourceCandidateAlreadyImported(candidates[left], fileStates)
		rightImported := sourceCandidateAlreadyImported(candidates[right], fileStates)
		if leftImported != rightImported {
			return !leftImported
		}

		leftSortTime := sourceCandidateSortTime(candidates[left])
		rightSortTime := sourceCandidateSortTime(candidates[right])
		if leftSortTime.Equal(rightSortTime) {
			return candidates[left].info.Name < candidates[right].info.Name
		}

		if triggeredBy == SyncTriggeredByBackfill {
			return leftSortTime.Before(rightSortTime)
		}
		return leftSortTime.After(rightSortTime)
	})
}

func sourceCandidateAlreadyImported(candidate sourceCandidate, fileStates map[string]syncFileImportState) bool {
	if len(fileStates) == 0 {
		return false
	}
	state, ok := fileStates[candidate.meta.OriginalName]
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(state.Status), "imported")
}

func filterSourceCandidatesForIngest(candidates []sourceCandidate, fileStates map[string]syncFileImportState) []sourceCandidate {
	if len(fileStates) == 0 || len(candidates) == 0 {
		return candidates
	}
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if sourceCandidateAlreadyImported(candidate, fileStates) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func sourceCandidateSortTime(candidate sourceCandidate) time.Time {
	if !candidate.meta.ExtractedAt.IsZero() {
		return candidate.meta.ExtractedAt.UTC()
	}
	if !candidate.meta.DataReference.IsZero() {
		return candidate.meta.DataReference.UTC()
	}
	if !candidate.info.ModTime.IsZero() {
		return candidate.info.ModTime.UTC()
	}
	return time.Time{}
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
