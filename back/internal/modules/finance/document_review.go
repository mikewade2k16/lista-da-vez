package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

type DocumentReview struct {
	Hash              string         `json:"hash"`
	DraftVersion      string         `json:"draftVersion"`
	ProfileVersion    string         `json:"profileVersion"`
	Issuer            FiscalProfile  `json:"issuer"`
	Fields            DocumentFields `json:"fields"`
	MissingFields     []string       `json:"missingFields"`
	Ready             bool           `json:"ready"`
	ApprovalID        string         `json:"approvalId"`
	ApprovedAt        string         `json:"approvedAt"`
	EmissionAvailable bool           `json:"emissionAvailable"`
}
type DocumentReviewInput struct {
	ReviewHash               string `json:"reviewHash"`
	ExpectedUpdatedAt        string `json:"expectedUpdatedAt"`
	ExpectedProfileUpdatedAt string `json:"expectedProfileUpdatedAt"`
	Confirmed                bool   `json:"confirmed"`
}

func buildDocumentReview(d DocumentDraft, p FiscalProfileView) DocumentReview {
	review := DocumentReview{DraftVersion: d.UpdatedAt, ProfileVersion: p.UpdatedAt, Issuer: p.Profile, Fields: d.Fields, MissingFields: []string{}}
	for _, field := range documentView(d).MissingFields {
		review.MissingFields = append(review.MissingFields, "Rascunho: "+field)
	}
	for _, field := range fiscalMissingFields(p.Profile) {
		review.MissingFields = append(review.MissingFields, "Emitente: "+field)
	}
	if d.Fields.Amount != nil && *d.Fields.Amount <= 0 {
		review.MissingFields = append(review.MissingFields, "Valor do serviço deve ser maior que zero")
	}
	document := strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(d.Fields.CustomerDocument))
	if document != "" && !fiscalDocumentPattern.MatchString(document) {
		review.MissingFields = append(review.MissingFields, "CPF/CNPJ do tomador em formato válido")
	}
	// Bind identity and both authoritative versions. Portal credentials never enter the snapshot.
	payload := struct {
		ID             string
		DraftVersion   string
		ProfileVersion string
		Fields         DocumentFields
		Issuer         FiscalProfile
	}{d.ID, d.UpdatedAt, p.UpdatedAt, d.Fields, p.Profile}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	review.Hash = hex.EncodeToString(sum[:])
	review.Ready = len(review.MissingFields) == 0
	return review
}
func (s *DocumentService) Review(ctx context.Context, account, user, id string) (DocumentReview, error) {
	if !isUUID(id) {
		return DocumentReview{}, ErrFiscalNotFound
	}
	d, err := s.store.Get(ctx, account, user, id)
	if err != nil {
		return DocumentReview{}, err
	}
	profile, err := s.fiscal.GetProfile(ctx, account, user)
	if err != nil {
		return DocumentReview{}, err
	}
	review := buildDocumentReview(d, profile)
	review.ApprovalID, review.ApprovedAt, err = s.store.ReviewApproval(ctx, account, user, id, review.Hash)
	return review, err
}
func (s *DocumentService) ApproveReview(ctx context.Context, account, user, id string, in DocumentReviewInput) (DocumentReview, error) {
	review, err := s.Review(ctx, account, user, id)
	if err != nil {
		return review, err
	}
	if in.ReviewHash != review.Hash || in.ExpectedUpdatedAt != review.DraftVersion || in.ExpectedProfileUpdatedAt != review.ProfileVersion {
		return review, ErrFiscalConflict
	}
	if !in.Confirmed {
		return review, invalidFiscal("Confirme a revisão dos dados antes de continuar.")
	}
	if !review.Ready {
		return review, invalidFiscal("Complete os dados pendentes do rascunho e do emitente antes de confirmar a revisão.")
	}
	review.ApprovalID, review.ApprovedAt, err = s.store.ApproveReview(ctx, account, user, id, review)
	return review, err
}
func (s *DocumentService) RevokeReview(ctx context.Context, account, user, id string, in DocumentReviewInput) (DocumentReview, error) {
	review, err := s.Review(ctx, account, user, id)
	if err != nil {
		return review, err
	}
	if in.ReviewHash != review.Hash || in.ExpectedUpdatedAt != review.DraftVersion || in.ExpectedProfileUpdatedAt != review.ProfileVersion {
		return review, ErrFiscalConflict
	}
	if err = s.store.RevokeReview(ctx, account, user, id, review.Hash); err != nil {
		return review, err
	}
	review.ApprovalID = ""
	review.ApprovedAt = ""
	return review, nil
}
