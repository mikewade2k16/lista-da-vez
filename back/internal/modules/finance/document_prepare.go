package finance

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type DocumentPreparation struct {
	Draft   DocumentDraft     `json:"draft"`
	Review  DocumentReview    `json:"review"`
	Session PortalSessionView `json:"session"`
}
type DocumentPrepareInput struct {
	ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
}

func (s *DocumentService) Prepare(ctx context.Context, account, user, id, expected string) (DocumentPreparation, error) {
	if !isUUID(id) {
		return DocumentPreparation{}, ErrFiscalNotFound
	}
	d, err := s.store.Get(ctx, account, user, id)
	if err != nil {
		return DocumentPreparation{}, err
	}
	if d.UpdatedAt != expected {
		return DocumentPreparation{}, ErrFiscalConflict
	}
	d, err = s.applyPreparationTemplate(ctx, account, user, d)
	if err != nil {
		return DocumentPreparation{}, err
	}
	review, err := s.Review(ctx, account, user, id)
	if err != nil {
		return DocumentPreparation{}, err
	}
	if review.DraftVersion != d.UpdatedAt {
		return DocumentPreparation{}, ErrFiscalConflict
	}
	profile, err := s.fiscal.GetProfile(ctx, account, user)
	if err != nil {
		return DocumentPreparation{}, err
	}
	if profile.UpdatedAt != review.ProfileVersion {
		return DocumentPreparation{}, ErrFiscalConflict
	}
	session := portalSessionView(storedPortalSession{}, "disconnected", false)
	if profile.CredentialsSet || profile.Profile.PortalLoginMethod == "govbr" {
		session, err = s.fiscal.ConnectPortal(ctx, account, user, review.ProfileVersion)
		if err != nil {
			return DocumentPreparation{}, err
		}
	}
	return DocumentPreparation{Draft: d, Review: review, Session: session}, nil
}
func (s *DocumentService) applyPreparationTemplate(ctx context.Context, account, user string, d DocumentDraft) (DocumentDraft, error) {
	f := d.Fields.Service
	if d.Fields.TemplateID != "" || f.ServiceDescription != "" || f.ServiceCode != "" || f.CNAE != "" || f.NBS != "" || f.ISSRate != nil || f.ISSWithheld != nil || f.Notes != "" {
		return d, nil
	}
	templates, err := s.fiscal.ListTemplates(ctx, account, user)
	if err != nil {
		return d, err
	}
	for _, t := range templates {
		if t.IsDefault {
			d.Fields.TemplateID = t.ID
			d.Fields.Service = t.Fields
			return s.Save(ctx, account, user, d.ID, DocumentDraftInput{Fields: d.Fields, ExpectedUpdatedAt: d.UpdatedAt})
		}
	}
	return d, nil
}

// Explicit fiscal values only; receipt roles and payment dates are never promoted implicitly.
func (s *DocumentService) UpdateFromConversation(ctx context.Context, account, user, id, text string) (DocumentDraft, bool, error) {
	if !isUUID(id) {
		return DocumentDraft{}, false, ErrFiscalNotFound
	}
	d, err := s.store.Get(ctx, account, user, id)
	if err != nil {
		return d, false, err
	}
	changed := false
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = normalizedDocumentLabel(key)
		value = strings.TrimSpace(value)
		switch key {
		case "tomador", "nome do tomador":
			d.Fields.CustomerName = value
		case "cpf/cnpj do tomador", "documento do tomador":
			d.Fields.CustomerDocument = value
		case "competencia", "competencia do servico":
			parsed, e := time.Parse("02/01/2006", value)
			if e == nil {
				value = parsed.Format("2006-01-02")
			}
			d.Fields.CompetenceDate = value
		case "servico", "descricao do servico":
			d.Fields.Service.ServiceDescription = value
		case "valor", "valor do servico":
			value = strings.TrimSpace(strings.TrimPrefix(value, "R$"))
			if strings.Contains(value, ",") {
				value = strings.ReplaceAll(value, ".", "")
				value = strings.ReplaceAll(value, ",", ".")
			}
			n, e := strconv.ParseFloat(value, 64)
			if e != nil {
				return d, false, invalidFiscal("Informe o valor no formato 57,58.")
			}
			d.Fields.Amount = &n
		case "tomador pagador":
			d.Fields.CustomerName = d.Fields.PayerName
			d.Fields.CustomerDocument = d.Fields.PayerDocument
		case "tomador recebedor":
			d.Fields.CustomerName = d.Fields.PayeeName
			d.Fields.CustomerDocument = d.Fields.PayeeDocument
		case "template":
			templates, e := s.fiscal.ListTemplates(ctx, account, user)
			if e != nil {
				return d, false, e
			}
			found := false
			for _, t := range templates {
				if strings.EqualFold(t.Name, value) {
					d.Fields.TemplateID = t.ID
					d.Fields.Service = t.Fields
					found = true
					break
				}
			}
			if !found {
				return d, false, invalidFiscal("Template não encontrado no seu perfil.")
			}
		default:
			continue
		}
		changed = true
	}
	if !changed {
		return d, false, nil
	}
	saved, err := s.Save(ctx, account, user, id, DocumentDraftInput{Fields: d.Fields, ExpectedUpdatedAt: d.UpdatedAt})
	return saved, true, err
}
