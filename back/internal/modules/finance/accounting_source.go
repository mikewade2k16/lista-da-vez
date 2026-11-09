package finance

import (
	"context"
)

// AccountingSourceInput is an explicit personal-to-business sharing request.
// The owner always reads in the caller's account/user scope, including administrators.
type AccountingSourceInput struct {
	Owner, DraftID, ExpectedDraftUpdatedAt, ExpectedProfileUpdatedAt string
	TemplateID, ExpectedTemplateUpdatedAt                            *string
	SelectedFields                                                   []string
	ConfirmShareWithIssuer                                           bool
}
type AccountingSourceService struct {
	Description *string `json:"description,omitempty"`
	ServiceCode *string `json:"serviceCode,omitempty"`
	CNAE        *string `json:"cnae,omitempty"`
	NBS         *string `json:"nbs,omitempty"`
}
type AccountingSource struct{ Service AccountingSourceService }

func (m *Module) ReadAccountingSource(ctx context.Context, account, user string, in AccountingSourceInput) (AccountingSource, error) {
	var out AccountingSource
	if m.handle == nil || !in.ConfirmShareWithIssuer || in.Owner != "finance" || !isUUID(in.DraftID) || len(in.SelectedFields) == 0 {
		return out, ErrFiscalInvalid
	}
	d, e := m.handle.documents.store.Get(ctx, account, user, in.DraftID)
	if e != nil {
		return out, e
	}
	p, e := m.handle.fiscal.GetProfile(ctx, account, user)
	if e != nil {
		return out, e
	}
	if d.UpdatedAt != in.ExpectedDraftUpdatedAt || p.UpdatedAt != in.ExpectedProfileUpdatedAt {
		return out, ErrFiscalConflict
	}
	fields := d.Fields.Service
	if in.TemplateID != nil {
		templates, e := m.handle.fiscal.ListTemplates(ctx, account, user)
		if e != nil {
			return out, e
		}
		found := false
		for _, t := range templates {
			if t.ID == *in.TemplateID {
				if in.ExpectedTemplateUpdatedAt == nil || t.UpdatedAt != *in.ExpectedTemplateUpdatedAt {
					return out, ErrFiscalConflict
				}
				fields = t.Fields
				found = true
			}
		}
		if !found {
			return out, ErrFiscalNotFound
		}
	}
	for _, name := range in.SelectedFields {
		switch name {
		case "service.description":
			out.Service.Description = &fields.ServiceDescription
		case "service.serviceCode":
			out.Service.ServiceCode = &fields.ServiceCode
		case "service.cnae":
			out.Service.CNAE = &fields.CNAE
		case "service.nbs":
			out.Service.NBS = &fields.NBS
		default:
			return out, ErrFiscalInvalid
		}
	}
	return out, nil
}
