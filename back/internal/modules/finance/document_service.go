package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"time"
)

type DocumentService struct {
	store     *DocumentStore
	fiscal    *FiscalService
	customers *FiscalCustomerStore
	ocr       DocumentOCR
	originals OriginalFiles
	slots     chan struct{}
}

func newDocumentService(poolStore *DocumentStore, fiscal *FiscalService) *DocumentService {
	return &DocumentService{store: poolStore, fiscal: fiscal, customers: &FiscalCustomerStore{pool: poolStore.pool}, ocr: RunDocumentOCR, slots: make(chan struct{}, 2)}
}
func (s *DocumentService) Extract(ctx context.Context, account, user, name string, raw []byte) (DocumentDraft, error) {
	d, err := s.extract(ctx, account, user, name, raw)
	if err == nil {
		sum := sha256.Sum256(raw)
		s.storeOriginal(ctx, account, user, d, assistantText(filepath.Base(strings.ReplaceAll(name, "\\", "/")), 180), hex.EncodeToString(sum[:]), raw)
	}
	return d, err
}

func (s *DocumentService) extract(ctx context.Context, account, user, name string, raw []byte) (DocumentDraft, error) {
	format, err := documentFormat(raw, name)
	if err != nil {
		return DocumentDraft{}, err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	var previous *DocumentDraft
	if existing, err := s.store.ByHash(ctx, account, user, hash); err == nil {
		parsed := parseDocumentText(existing.ExtractedText)
		if format != "image" || parsed.Amount != nil && parsed.PaymentDate != "" && parsed.PayerName != "" && parsed.PayeeName != "" {
			return s.ReidentifyParties(ctx, account, user, existing.ID, existing.UpdatedAt)
		}
		previous = &existing
	} else if !errors.Is(err, ErrFiscalNotFound) {
		return DocumentDraft{}, err
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return DocumentDraft{}, ErrDocumentOCR
	}
	text, err := extractDocumentText(ctx, raw, format, s.ocr)
	if err != nil {
		return DocumentDraft{}, err
	}
	if previous != nil {
		fields, _ := recoverDocumentParties(previous.Fields, text)
		recovered := parseDocumentText(text)
		if fields.Amount == nil {
			fields.Amount = recovered.Amount
		}
		if fields.PaymentDate == "" {
			fields.PaymentDate = recovered.PaymentDate
		}
		fields.DocumentType = recovered.DocumentType
		return s.store.saveExtraction(ctx, account, user, previous.ID, DocumentDraftInput{Fields: fields, ExpectedUpdatedAt: previous.UpdatedAt}, &text)
	}
	name = assistantText(filepath.Base(strings.ReplaceAll(name, "\\", "/")), 180)
	return s.store.Create(ctx, account, user, hash, name, text, parseDocumentText(text))
}
func (s *DocumentService) List(ctx context.Context, account, user string) ([]DocumentDraft, error) {
	return s.store.List(ctx, account, user)
}
func (s *DocumentService) Save(ctx context.Context, account, user, id string, in DocumentDraftInput) (DocumentDraft, error) {
	if !isUUID(id) {
		return DocumentDraft{}, ErrFiscalNotFound
	}
	if _, err := time.Parse(time.RFC3339Nano, in.ExpectedUpdatedAt); err != nil {
		return DocumentDraft{}, ErrFiscalConflict
	}
	f := &in.Fields
	f.CustomerName = strings.TrimSpace(f.CustomerName)
	f.CustomerDocument = strings.TrimSpace(f.CustomerDocument)
	if f.Amount != nil && (math.IsNaN(*f.Amount) || math.IsInf(*f.Amount, 0) || *f.Amount < 0 || *f.Amount > 1e12) {
		return DocumentDraft{}, invalidFiscal("Valor inválido.")
	}
	for _, date := range []string{f.PaymentDate, f.CompetenceDate} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return DocumentDraft{}, invalidFiscal("Data inválida.")
			}
		}
	}
	if len(f.PayerDocument) > 32 || len(f.PayeeDocument) > 32 {
		return DocumentDraft{}, invalidFiscal("CPF/CNPJ excede 32 caracteres.")
	}
	if len(f.CustomerName) > 300 || len(f.CustomerDocument) > 32 || len(f.PayerName) > 600 || len(f.PayeeName) > 600 || len(f.Service.ServiceDescription) > 4000 || len(f.Service.Notes) > 2000 {
		return DocumentDraft{}, invalidFiscal("Revise o tamanho dos campos.")
	}
	for _, code := range []string{f.Service.ServiceCode, f.Service.CNAE, f.Service.NBS} {
		if len(code) > 60 {
			return DocumentDraft{}, invalidFiscal("Código de serviço excede 60 caracteres.")
		}
	}
	if f.Service.ISSRate != nil && (math.IsNaN(*f.Service.ISSRate) || math.IsInf(*f.Service.ISSRate, 0) || *f.Service.ISSRate < 0 || *f.Service.ISSRate > 100) {
		return DocumentDraft{}, invalidFiscal("Alíquota deve estar entre 0 e 100.")
	}
	if f.TemplateID != "" {
		list, err := s.fiscal.ListTemplates(ctx, account, user)
		if err != nil {
			return DocumentDraft{}, err
		}
		found := false
		for _, t := range list {
			if t.ID == f.TemplateID {
				found = true
				break
			}
		}
		if !found {
			return DocumentDraft{}, ErrFiscalNotFound
		}
	}
	if f.DocumentType != "documento" && f.DocumentType != "comprovante" {
		return DocumentDraft{}, invalidFiscal("Tipo de documento inválido.")
	}
	return s.store.Save(ctx, account, user, id, in)
}

// ReidentifyParties fills only empty names and documents from saved OCR. Manual corrections and fiscal roles stay intact.
func (s *DocumentService) ReidentifyParties(ctx context.Context, account, user, id, expected string) (DocumentDraft, error) {
	if !isUUID(id) {
		return DocumentDraft{}, ErrFiscalNotFound
	}
	d, err := s.store.Get(ctx, account, user, id)
	if err != nil {
		return d, err
	}
	if expected != d.UpdatedAt {
		return d, ErrFiscalConflict
	}
	fields, changed := recoverDocumentParties(d.Fields, d.ExtractedText)
	if !changed {
		return d, nil
	}
	return s.Save(ctx, account, user, id, DocumentDraftInput{Fields: fields, ExpectedUpdatedAt: expected})
}

func (s *DocumentService) Get(ctx context.Context, account, user, id string) (DocumentDraft, error) {
	if !isUUID(id) {
		return DocumentDraft{}, ErrFiscalNotFound
	}
	return s.store.Get(ctx, account, user, id)
}
