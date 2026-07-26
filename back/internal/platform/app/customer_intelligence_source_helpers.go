package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
)

type customerDataSourceEvidenceReader interface {
	GetSourceEvidence(
		context.Context,
		customerdata.SourceEvidenceRequest,
	) (customerdata.SourceEvidenceBundle, error)
}

type permanentSourceFailure struct {
	code string
}

func (failure permanentSourceFailure) Error() string {
	return "customer intelligence source is unavailable"
}

func (failure permanentSourceFailure) SourceFailureCode() string {
	return failure.code
}

func (failure permanentSourceFailure) SourceRetryable() bool {
	return false
}

func newPermanentSourceFailure(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "source_unavailable"
	}
	return permanentSourceFailure{code: code}
}

func decodeSourceOptions(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return newPermanentSourceFailure("source_config_invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return newPermanentSourceFailure("source_config_invalid")
	}
	return nil
}

func sourceSnapshot(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("customer intelligence source snapshot unavailable")
	}
	return raw, nil
}

func sourceEntityTypeAllowed(entityType string, allowed map[string]bool) bool {
	return allowed[strings.ToLower(strings.TrimSpace(entityType))]
}

var _ error = permanentSourceFailure{}
