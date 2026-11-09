package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
)

func plusAuthorizer(checker *auth.PostgresAccountMemberChecker, catalog *core.ModuleAccess) func(context.Context, finance.PlusScope, string) error {
	return func(ctx context.Context, sc finance.PlusScope, permission string) error {
		if strings.HasPrefix(permission, "finance.") {
			principal, err := checker.ResolveAccountPrincipal(ctx, sc.AccountID, sc.ActorID)
			if err != nil {
				if errors.Is(err, auth.ErrAccountPrincipalDenied) {
					return &finance.PlusError{Code: "permission_denied", Status: 403}
				}
				return err
			}
			enabled, err := catalog.Enabled(ctx, sc.AccountID, "finance")
			if err != nil {
				return err
			}
			if !enabled {
				return &finance.PlusError{Code: "module_disabled", Status: 403}
			}
			if !finance.CanFinancePermission(principal, permission) {
				return &finance.PlusError{Code: "permission_denied", Status: 403}
			}
			return nil
		}

		member, e := catalog.ActiveMember(ctx, sc.AccountID, sc.ActorID)
		if e != nil {
			return e
		}
		if !member {
			return &finance.PlusError{Code: "permission_denied", Status: 403}
		}
		module := "finance"
		if len(permission) >= 11 && permission[:11] == "accounting." {
			module = "accounting"
		}
		enabled, e := catalog.Enabled(ctx, sc.AccountID, module)
		if e != nil {
			return e
		}
		if !enabled {
			return &finance.PlusError{Code: "module_disabled", Status: 403}
		}
		permissions, e := checker.ResolveAccountPermissions(ctx, sc.AccountID, sc.ActorID)
		if e != nil {
			return e
		}
		for _, v := range permissions {
			if v == permission {
				return nil
			}
		}
		return &finance.PlusError{Code: "permission_denied", Status: 403}
	}
}
func plusDecode(raw json.RawMessage, value any) error {
	if len(raw) > 12*1024 {
		return &finance.PlusError{Code: "invalid_input", Status: 400}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(value); e != nil {
		return &finance.PlusError{Code: "invalid_input", Status: 400}
	}
	if e := decoder.Decode(new(any)); !errors.Is(e, io.EOF) {
		return &finance.PlusError{Code: "invalid_input", Status: 400}
	}
	return nil
}
func accountingPlusPort(module *accounting.Module) func(context.Context, finance.PlusScope, finance.PlusAction) (json.RawMessage, error) {
	return func(ctx context.Context, scope finance.PlusScope, action finance.PlusAction) (json.RawMessage, error) {
		svc := module.Service()
		if svc == nil {
			return nil, &finance.PlusError{Code: "capability_unavailable", Status: 503}
		}
		sc := accounting.Scope{AccountID: scope.AccountID, ActorID: scope.ActorID, IssuerID: scope.IssuerID, CorrelationID: scope.RequestID}
		var value any
		var e error
		switch action.ToolID {
		case "accounting.obligations.read.v1":
			var in struct {
				Period       string `json:"period"`
				ObligationID string `json:"obligationId"`
			}
			if e = plusDecode(action.Input, &in); e != nil {
				return nil, e
			}
			value, e = svc.ReadObligations(ctx, sc, in.Period, in.ObligationID, 1, 20)
		case "accounting.obligation.finance.prepare.v1":
			var in struct {
				ObligationID string `json:"obligationId"`
				accounting.ObligationAction
			}
			if e = plusDecode(action.Input, &in); e != nil {
				return nil, e
			}
			value, e = svc.ObligationCommand(ctx, sc, in.ObligationID, "prepare-finance", scope.RequestID+":obligation-finance", in.ObligationAction)
		case "accounting.issuer.read.v1":
			value, e = svc.Read(ctx, sc, "issuer", "", "", "", 1, 20)
		case "accounting.diagnosis.read.v1":
			var in struct {
				DiagnosisID string `json:"diagnosisId"`
			}
			if e = plusDecode(action.Input, &in); e != nil {
				return nil, e
			}
			kind := "diagnosis"
			if in.DiagnosisID == "" {
				kind = "diagnoses"
			}
			value, e = svc.Read(ctx, sc, kind, in.DiagnosisID, "", "", 1, 20)
			if in.DiagnosisID == "" && e == nil {
				value = map[string]any{"diagnoses": value, "limit": 20}
			}
		case "accounting.document.review.v1":
			var in struct {
				DocumentID string `json:"documentId"`
			}
			if e = plusDecode(action.Input, &in); e != nil {
				return nil, e
			}
			value, e = svc.Read(ctx, sc, "review", in.DocumentID, "", "", 1, 20)
		case "accounting.diagnosis.save.v1":
			var in accounting.DiagnosisInput
			if e = plusDecode(action.Input, &in); e != nil {
				return nil, e
			}
			value, e = svc.SaveDiagnosis(ctx, sc, scope.RequestID+":diagnosis", in)
		case "accounting.document.prepare.v1":
			var in accounting.PrepareInput
			if e = plusDecode(action.Input, &in); e != nil {
				return nil, e
			}
			value, e = svc.Prepare(ctx, sc, scope.RequestID+":prepare", in)
		default:
			return nil, &finance.PlusError{Code: "tool_not_registered", Status: 400}
		}
		if e != nil {
			var domain *accounting.Error
			if errors.As(e, &domain) {
				return nil, &finance.PlusError{Code: domain.Code, Status: domain.Status}
			}
			return nil, e
		}
		return json.Marshal(value)
	}
}
func calendarPlusPorts(module *finance.Module) calendar.AssistantPlusPorts {
	translate := func(e error) error {
		if e == nil {
			return nil
		}
		var domain *finance.PlusError
		if errors.As(e, &domain) {
			return &calendar.AssistantPlusError{Code: domain.Code, Status: domain.Status}
		}
		switch {
		case errors.Is(e, finance.ErrSheetNotFound) || errors.Is(e, finance.ErrFiscalNotFound):
			return &calendar.AssistantPlusError{Code: "not_found", Status: 404}
		case errors.Is(e, finance.ErrFiscalConflict):
			return &calendar.AssistantPlusError{Code: "version_conflict", Status: 409}
		case errors.Is(e, finance.ErrFiscalInvalid):
			return &calendar.AssistantPlusError{Code: "invalid_input", Status: 400}
		default:
			return e
		}
	}
	return calendar.AssistantPlusPorts{
		ValidateOutput: func(ctx context.Context, scope calendar.AssistantPlusScope, raw json.RawMessage) error {
			if module.Plus() == nil {
				return &calendar.AssistantPlusError{Code: "capability_unavailable", Status: 503}
			}
			return translate(module.Plus().ValidateOutput(ctx, finance.PlusScope{AccountID: scope.AccountID, ActorID: scope.ActorID, IssuerID: scope.IssuerID}, raw))
		},
		Authorize: func(ctx context.Context, scope calendar.AssistantPlusScope) error {
			if module.Plus() == nil {
				return &calendar.AssistantPlusError{Code: "capability_unavailable", Status: 503}
			}
			return translate(module.Plus().Authorize(ctx, finance.PlusScope{AccountID: scope.AccountID, ActorID: scope.ActorID, IssuerID: scope.IssuerID, RequestID: scope.RequestID}))
		},
		Execute: func(ctx context.Context, scope calendar.AssistantPlusScope, req calendar.ChatAskRequest) (json.RawMessage, error) {
			in := finance.PlusInvocation{CompetenceDate: req.CompetenceDate, Question: req.Question, FinanceSheetID: req.FinanceSheetID, FinanceDraftID: req.FinanceDraftID, DiagnosisID: req.DiagnosisID, DocumentID: req.DocumentID, Month: req.Month}
			if req.Action != nil {
				in.Action = &finance.PlusAction{ToolID: req.Action.ToolID, Input: req.Action.Input}
			}
			value, e := module.Plus().Execute(ctx, finance.PlusScope{AccountID: scope.AccountID, ActorID: scope.ActorID, IssuerID: scope.IssuerID, RequestID: scope.RequestID}, in)
			if e != nil {
				return nil, translate(e)
			}
			return json.Marshal(value)
		},
	}
}
