package erp

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrForbidden             = errors.New("erp: forbidden")
	ErrValidation            = errors.New("erp: validation")
	ErrTenantRequired        = errors.New("erp: tenant required")
	ErrStoreRequired         = errors.New("erp: store required")
	ErrStoreNotFound         = errors.New("erp: store not found")
	ErrManualSyncDisabled    = errors.New("erp: manual sync disabled")
	ErrNotImplemented        = errors.New("erp: not implemented")
	ErrSourceNotConfigured   = errors.New("erp: source not configured")
	ErrSourcePathOutsideRoot = errors.New("erp: source path outside root")
	ErrSourceHostKeyRequired = errors.New("erp: source host key required")
	ErrUnsupportedDataType   = errors.New("erp: unsupported data type")
	ErrSyncAlreadyRunning    = errors.New("erp: sync already running")
	ErrSyncRateLimited       = errors.New("erp: sync rate limited")
)

type ErrCSVEncoding struct {
	SourceName string
	Cause      error
}

func (err *ErrCSVEncoding) Error() string {
	if strings.TrimSpace(err.SourceName) == "" {
		return fmt.Sprintf("erp: csv encoding invalid: %v", err.Cause)
	}
	return fmt.Sprintf("erp: csv encoding invalid for %s: %v", err.SourceName, err.Cause)
}

func (err *ErrCSVEncoding) Unwrap() error {
	return err.Cause
}

type ErrCSVTooLarge struct {
	SourceName string
	MaxBytes   int64
	GotBytes   int64
}

func (err *ErrCSVTooLarge) Error() string {
	return fmt.Sprintf(
		"erp: csv too large for %s: max %d bytes, got at least %d bytes",
		firstNonEmpty(strings.TrimSpace(err.SourceName), "csv"),
		err.MaxBytes,
		err.GotBytes,
	)
}

type ErrCSVColumnCountMismatch struct {
	SourceName string
	DataType   string
	LineNumber int
	Expected   int
	Got        int
}

func (err *ErrCSVColumnCountMismatch) Error() string {
	return fmt.Sprintf(
		"erp: csv column count mismatch for %s line %d: expected %d, got %d",
		firstNonEmpty(strings.TrimSpace(err.SourceName), strings.TrimSpace(err.DataType), "csv"),
		err.LineNumber,
		err.Expected,
		err.Got,
	)
}

type ErrCSVFilenameInvalid struct {
	Name  string
	Cause error
}

func (err *ErrCSVFilenameInvalid) Error() string {
	if err.Cause == nil {
		return fmt.Sprintf("erp: csv filename invalid: %s", err.Name)
	}
	return fmt.Sprintf("erp: csv filename invalid: %s: %v", err.Name, err.Cause)
}

func (err *ErrCSVFilenameInvalid) Unwrap() error {
	return err.Cause
}

type ErrCSVHeaderMismatch struct {
	SourceName string
	DataType   string
	Expected   []string
	Got        []string
}

func (err *ErrCSVHeaderMismatch) Error() string {
	return fmt.Sprintf(
		"erp: csv header mismatch for %s: expected [%s], got [%s]",
		firstNonEmpty(strings.TrimSpace(err.SourceName), strings.TrimSpace(err.DataType), "csv"),
		strings.Join(err.Expected, ";"),
		strings.Join(err.Got, ";"),
	)
}

type ErrCSVRowParse struct {
	SourceName string
	DataType   string
	LineNumber int
	Field      string
	Cause      error
}

func (err *ErrCSVRowParse) Error() string {
	field := strings.TrimSpace(err.Field)
	if field == "" {
		field = "row"
	}
	return fmt.Sprintf(
		"erp: csv row parse failed for %s line %d field %s: %v",
		firstNonEmpty(strings.TrimSpace(err.SourceName), strings.TrimSpace(err.DataType), "csv"),
		err.LineNumber,
		field,
		err.Cause,
	)
}

func (err *ErrCSVRowParse) Unwrap() error {
	return err.Cause
}
