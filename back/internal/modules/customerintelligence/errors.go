package customerintelligence

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidInput                    = errors.New("customer intelligence: entrada invalida")
	ErrForbidden                       = errors.New("customer intelligence: acesso negado")
	ErrNotFound                        = errors.New("customer intelligence: recurso nao encontrado")
	ErrConflict                        = errors.New("customer intelligence: conflito de revisao ou estado")
	ErrRetentionPolicyApprovalRequired = errors.New(
		"customer intelligence: politica de retencao requer publicacao aprovada",
	)
	ErrCapabilityDisabled       = errors.New("customer intelligence: capability desabilitada")
	ErrPromptNotPublished       = errors.New("customer intelligence: prompt nao publicado")
	ErrPromptNotValidated       = errors.New("customer intelligence: prompt nao validado")
	ErrPromptEvaluationRequired = errors.New(
		"customer intelligence: avaliacao aprovada do prompt obrigatoria",
	)
	ErrAgentNotPublished     = errors.New("customer intelligence: agente nao publicado")
	ErrSecretsUnavailable    = errors.New("customer intelligence: cofre de segredos indisponivel")
	ErrProviderNotConfigured = errors.New("customer intelligence: provider nao configurado")
)

// RuntimeFailureKind is the stable machine-readable failure contract exposed
// to runtime consumers. A technical failure must never be translated into a
// successful no_reply decision.
type RuntimeFailureKind string

const (
	RuntimeFailureDisabled               RuntimeFailureKind = "disabled"
	RuntimeFailureNotAuthorized          RuntimeFailureKind = "not_authorized"
	RuntimeFailureInvalidInput           RuntimeFailureKind = "invalid_input"
	RuntimeFailureTimeout                RuntimeFailureKind = "timeout"
	RuntimeFailureTemporarilyUnavailable RuntimeFailureKind = "temporarily_unavailable"
	RuntimeFailureInvalidResult          RuntimeFailureKind = "invalid_result"
	RuntimeFailureBudgetExceeded         RuntimeFailureKind = "budget_exceeded"
	RuntimeFailurePermanent              RuntimeFailureKind = "permanent_failure"
	RuntimeFailureShadowNoEffect         RuntimeFailureKind = "shadow_no_effect"
)

// RuntimeFailure is intentionally safe to log: Error never includes the
// provider response, prompt, customer input, secret or wrapped error text.
// Cause remains available to errors.Is/errors.As inside trusted Go code.
type RuntimeFailure struct {
	Kind           RuntimeFailureKind
	Code           string
	Retryable      bool
	ShadowDecision *InteractionDecision
	Cause          error
}

func (e *RuntimeFailure) Error() string {
	if e == nil {
		return "customer intelligence: runtime failure"
	}
	return fmt.Sprintf("customer intelligence: runtime %s (%s)", e.Kind, e.Code)
}

func (e *RuntimeFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newRuntimeFailure(
	kind RuntimeFailureKind,
	code string,
	retryable bool,
	cause error,
) error {
	return &RuntimeFailure{
		Kind: kind, Code: code, Retryable: retryable, Cause: cause,
	}
}

func newShadowNoEffectFailure(decision InteractionDecision) error {
	copy := decision
	return &RuntimeFailure{
		Kind:           RuntimeFailureShadowNoEffect,
		Code:           "shadow_no_effect",
		Retryable:      false,
		ShadowDecision: &copy,
	}
}

// RuntimeFailureDetails lets consumers choose bounded retry or fail-open
// without parsing error strings.
func RuntimeFailureDetails(err error) (kind RuntimeFailureKind, code string, retryable bool, ok bool) {
	var failure *RuntimeFailure
	if !errors.As(err, &failure) {
		return "", "", false, false
	}
	return failure.Kind, failure.Code, failure.Retryable, true
}

// RuntimeShadowDecision returns the validated decision produced in shadow.
// Consumers may compare/audit it, but must never apply operational effects.
func RuntimeShadowDecision(err error) (InteractionDecision, bool) {
	var failure *RuntimeFailure
	if !errors.As(err, &failure) ||
		failure.Kind != RuntimeFailureShadowNoEffect ||
		failure.ShadowDecision == nil {
		return InteractionDecision{}, false
	}
	return *failure.ShadowDecision, true
}
