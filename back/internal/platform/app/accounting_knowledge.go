package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"strings"
)

// accountingKnowledgePorts grava o original da fonte no storage privado. O Knowledge
// referencia o original por UUID; o ID do Storage (32 hex) e convertido no adapter.
func accountingKnowledgePorts(originals privateOriginals) knowledge.Ports {
	return knowledge.Ports{IngestionSources: knowledge.OfficialIngestionSources(), ExtractPDF: finance.ExtractKnowledgePDF, PutOriginal: func(ctx context.Context, sc knowledge.Scope, key string, in knowledge.Import) (string, error) {
		ref, e := originals.Put(ctx, sc.AccountID, sc.ActorID, "accounting", "knowledge:"+sc.ResourceID+":"+key, in.FileName, in.ContentType, in.Content)
		if e != nil {
			return "", e
		}
		_, id, _ := strings.Cut(ref, ":")
		return id, nil
	}}
}
func accountingKnowledgePlusPort(module *accounting.Module) func(context.Context, finance.PlusScope, finance.PlusAction) (json.RawMessage, error) {
	return func(ctx context.Context, sc finance.PlusScope, action finance.PlusAction) (json.RawMessage, error) {
		svc := module.Service()
		if svc == nil || svc.Knowledge() == nil {
			return nil, &finance.PlusError{Code: "capability_unavailable", Status: 503}
		}
		var in knowledge.Query
		if e := plusDecode(action.Input, &in); e != nil {
			return nil, e
		}
		v, e := svc.Knowledge().Search(ctx, knowledge.Scope{AccountID: sc.AccountID, ActorID: sc.ActorID, ResourceID: sc.IssuerID}, in)
		var domain *knowledge.Error
		if errors.As(e, &domain) {
			return nil, &finance.PlusError{Code: domain.Code, Status: domain.Status}
		}
		var accountError *accounting.Error
		if errors.As(e, &accountError) {
			return nil, &finance.PlusError{Code: accountError.Code, Status: accountError.Status}
		}
		if e != nil {
			return nil, e
		}
		return json.Marshal(v)
	}
}

func accountingEvidenceValidator(module *accounting.Module) func(context.Context, finance.PlusScope, []string) error {
	return func(ctx context.Context, sc finance.PlusScope, ids []string) error {
		if module.Service() == nil || module.Service().Knowledge() == nil {
			return &finance.PlusError{Code: "capability_unavailable", Status: 503}
		}
		e := module.Service().Knowledge().ValidateReferences(ctx, knowledge.Scope{AccountID: sc.AccountID, ActorID: sc.ActorID, ResourceID: sc.IssuerID}, ids)
		var domain *knowledge.Error
		if errors.As(e, &domain) {
			return &finance.PlusError{Code: domain.Code, Status: domain.Status}
		}
		var accountError *accounting.Error
		if errors.As(e, &accountError) {
			return &finance.PlusError{Code: accountError.Code, Status: accountError.Status}
		}
		return e
	}
}

func publicKnowledgePorts(pool *pgxpool.Pool) (func(context.Context, []string) (json.RawMessage, error), func(context.Context, []string) error) {
	svc := knowledge.NewService(knowledge.NewStore(pool), knowledge.Ports{})
	return func(ctx context.Context, terms []string) (json.RawMessage, error) {
		result, err := svc.SearchPublic(ctx, terms)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}, svc.ValidatePublicReferences
}
