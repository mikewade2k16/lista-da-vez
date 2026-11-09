package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/storage"
)

type accountingPolicy struct {
	checker *auth.PostgresAccountMemberChecker
	catalog *core.ModuleAccess
}

func (p accountingPolicy) Member(ctx context.Context, account, user string) (bool, error) {
	return p.catalog.ActiveMember(ctx, account, user)
}
func (p accountingPolicy) Check(ctx context.Context, s accounting.Scope, permission string) error {
	member, e := p.Member(ctx, s.AccountID, s.ActorID)
	if e != nil {
		return e
	}
	if !member {
		return &accounting.Error{Code: "permission_denied", Status: 403}
	}
	enabled, e := p.catalog.Enabled(ctx, s.AccountID, "accounting")
	if e != nil {
		return e
	}
	if !enabled {
		return &accounting.Error{Code: "module_disabled", Status: 403}
	}
	permissions, e := p.checker.ResolveAccountPermissions(ctx, s.AccountID, s.ActorID)
	if e != nil {
		return e
	}
	for _, v := range permissions {
		if v == permission {
			return nil
		}
	}
	return &accounting.Error{Code: "permission_denied", Status: 403}
}

type accountingPersonalReader struct {
	module  *finance.Module
	catalog *core.ModuleAccess
	checker *auth.PostgresAccountMemberChecker
}

func (p accountingPersonalReader) ReadPersonalSource(ctx context.Context, sc accounting.Scope, in accounting.PersonalSource) (accounting.PersonalSnapshot, error) {
	enabled, e := p.catalog.Enabled(ctx, sc.AccountID, "finance")
	if e != nil {
		return accounting.PersonalSnapshot{}, e
	}
	if !enabled {
		return accounting.PersonalSnapshot{}, &accounting.Error{Code: "capability_unavailable", Status: 422}
	}
	perms, e := p.checker.ResolveAccountPermissions(ctx, sc.AccountID, sc.ActorID)
	if e != nil {
		return accounting.PersonalSnapshot{}, e
	}
	allowed := false
	for _, perm := range perms {
		if perm == "finance.sheets.view" {
			allowed = true
		}
	}
	if !allowed {
		return accounting.PersonalSnapshot{}, &accounting.Error{Code: "permission_denied", Status: 403}
	}
	data, e := p.module.ReadAccountingSource(ctx, sc.AccountID, sc.ActorID, finance.AccountingSourceInput{Owner: in.Owner, DraftID: in.DraftID, ExpectedDraftUpdatedAt: in.ExpectedDraftUpdatedAt, ExpectedProfileUpdatedAt: in.ExpectedProfileUpdatedAt, TemplateID: in.TemplateID, ExpectedTemplateUpdatedAt: in.ExpectedTemplateUpdatedAt, SelectedFields: in.SelectedFields, ConfirmShareWithIssuer: in.ConfirmShareWithIssuer})
	if e != nil {
		switch {
		case errors.Is(e, finance.ErrFiscalNotFound):
			return accounting.PersonalSnapshot{}, &accounting.Error{Code: "not_found", Status: 404}
		case errors.Is(e, finance.ErrFiscalInvalid):
			return accounting.PersonalSnapshot{}, &accounting.Error{Code: "invalid_input", Status: 400}
		default:
			return accounting.PersonalSnapshot{}, &accounting.Error{Code: "source_stale_or_unavailable", Status: 409}
		}
	}
	// Convert only the explicit projection; never credentials or portal/session fields.
	b, e := json.Marshal(data.Service)
	if e != nil {
		return accounting.PersonalSnapshot{}, e
	}
	var service accounting.ServiceFields
	if e = json.Unmarshal(b, &service); e != nil {
		return accounting.PersonalSnapshot{}, e
	}
	reference, e := json.Marshal(in)
	return accounting.PersonalSnapshot{Service: &service, Reference: reference}, e
}

type accountingStorage struct{ module *storage.Module }

func (p accountingStorage) PutPrivate(ctx context.Context, s accounting.Scope, execution string, a accounting.Artifact) (string, error) {
	if p.module.Service() == nil {
		return "", fmt.Errorf("accounting storage unavailable")
	}
	name, mime := "", ""
	switch a.Kind {
	case "xml":
		name, mime = "document.xml", "application/xml"
	case "pdf":
		name, mime = "document.pdf", "application/pdf"
	case "protocol":
		name, mime = "protocol.json", "application/json"
	default:
		return "", fmt.Errorf("unsupported fiscal artifact")
	}
	object, e := p.module.Service().Upload(ctx, storage.UploadInput{AccountID: s.AccountID, SourceModule: "accounting", IdempotencyKey: execution + ":" + a.Kind + ":" + a.SHA256, FileName: name, ContentType: mime, Content: a.Content, CreatedBy: s.ActorID})
	return object.ID, e
}

func (p accountingStorage) OpenPrivate(ctx context.Context, sc accounting.Scope, id string) (io.ReadCloser, error) {
	if p.module.Service() == nil {
		return nil, fmt.Errorf("accounting storage unavailable")
	}
	_, content, e := p.module.Service().DownloadForSource(ctx, sc.AccountID, id, "accounting", "")
	if e != nil {
		return nil, e
	}
	return content.Body, nil
}
