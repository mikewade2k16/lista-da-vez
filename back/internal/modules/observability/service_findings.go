package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	findingIDPattern = regexp.MustCompile(`^[A-Z]-\d+$`)
	dueDatePattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	findingStatuses = map[string]bool{"open": true, "fixed": true, "accepted": true}
)

const (
	maxFindingOwner = 120
	maxFindingNote  = 2000
)

// ErrInvalidFinding carrega a mensagem (em português) devolvida no 400.
type ErrInvalidFinding struct{ Message string }

func (e ErrInvalidFinding) Error() string { return e.Message }

// optionalDate distingue campo ausente, null (limpa o prazo) e data informada.
type optionalDate struct {
	Set   bool
	Value *string
}

func (o *optionalDate) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	o.Value = &raw
	return nil
}

// findingPatch é o corpo do PATCH; ponteiros nulos = campo não enviado.
type findingPatch struct {
	Status  *string      `json:"status"`
	Owner   *string      `json:"owner"`
	DueDate optionalDate `json:"dueDate"`
	Note    *string      `json:"note"`
}

func (p findingPatch) empty() bool {
	return p.Status == nil && p.Owner == nil && !p.DueDate.Set && p.Note == nil
}

// validateFindingPatch normaliza (trim) e valida os campos enviados.
func validateFindingPatch(p findingPatch) (findingPatch, error) {
	if p.empty() {
		return p, ErrInvalidFinding{"Informe ao menos um campo para alterar."}
	}
	if p.Status != nil {
		status := strings.TrimSpace(*p.Status)
		if !findingStatuses[status] {
			return p, ErrInvalidFinding{"Status inválido: use open, fixed ou accepted."}
		}
		p.Status = &status
	}
	if p.Owner != nil {
		owner := strings.TrimSpace(*p.Owner)
		if utf8.RuneCountInString(owner) > maxFindingOwner {
			return p, ErrInvalidFinding{"Responsável deve ter até 120 caracteres."}
		}
		p.Owner = &owner
	}
	if p.Note != nil {
		note := strings.TrimSpace(*p.Note)
		if utf8.RuneCountInString(note) > maxFindingNote {
			return p, ErrInvalidFinding{"Nota deve ter até 2000 caracteres."}
		}
		p.Note = &note
	}
	if p.DueDate.Set && p.DueDate.Value != nil {
		value := strings.TrimSpace(*p.DueDate.Value)
		if _, err := time.Parse("2006-01-02", value); err != nil || !dueDatePattern.MatchString(value) {
			return p, ErrInvalidFinding{"Prazo inválido: use YYYY-MM-DD."}
		}
		p.DueDate.Value = &value
	}
	return p, nil
}

// findingOverdue: aberto com prazo anterior a hoje (datas YYYY-MM-DD comparam como texto).
func findingOverdue(status string, dueDate *string, today string) bool {
	return status == "open" && dueDate != nil && *dueDate != "" && *dueDate < today
}

// todayLocal devolve a data de hoje no fuso do painel (America/Sao_Paulo; UTC-3 se
// a base de fusos não existir na imagem).
func todayLocal(now time.Time) string {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.FixedZone("BRT", -3*3600)
	}
	return now.In(loc).Format("2006-01-02")
}

// UpdateFinding valida o id e o corpo e grava a edição do achado.
func (s *Service) UpdateFinding(ctx context.Context, id string, patch findingPatch, actor alertActor) (securityFinding, error) {
	if !findingIDPattern.MatchString(id) {
		return securityFinding{}, ErrInvalidFinding{"Achado inválido."}
	}
	clean, err := validateFindingPatch(patch)
	if err != nil {
		return securityFinding{}, err
	}
	return s.store.UpdateFinding(ctx, id, clean, actor, todayLocal(time.Now()))
}

func isInvalidFinding(err error) (ErrInvalidFinding, bool) {
	var invalid ErrInvalidFinding
	return invalid, errors.As(err, &invalid)
}
