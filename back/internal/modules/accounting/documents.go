package accounting

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Prepare(ctx context.Context, sc Scope, key string, in PrepareInput) (json.RawMessage, error) {
	if e := s.check(ctx, sc, "accounting.documents.manage"); e != nil {
		return nil, e
	}
	if !validID(in.DiagnosisID) || in.ExpectedVersion != 0 || (in.Kind != "nfse" && in.Kind != "nfe") || !validFields(in.Fields) {
		return nil, failure("invalid_input", 400)
	}
	return s.store.command(ctx, sc, "document.prepare", key, in, "manage", func(tx pgx.Tx) (any, error) {
		p, pv, e := profile(ctx, tx, sc)
		if e != nil {
			return nil, e
		}
		if pv != in.ExpectedProfileVersion {
			return nil, failure("version_conflict", 409)
		}
		b, e := readDiagnosis(ctx, tx, sc, in.DiagnosisID)
		if e != nil {
			return nil, e
		}
		var diag struct {
			Version        int64 `json:"version"`
			ProfileVersion int64 `json:"profileVersion"`
		}
		if e = json.Unmarshal(b, &diag); e != nil {
			return nil, e
		}
		if diag.Version != in.ExpectedDiagnosisVersion || diag.ProfileVersion != pv {
			return nil, failure("version_conflict", 409)
		}
		var source json.RawMessage
		if in.Source != nil {
			if s.personal == nil {
				return nil, failure("capability_unavailable", 422)
			}
			imported, e := s.personal.ReadPersonalSource(ctx, sc, *in.Source)
			if e != nil {
				return nil, e
			}
			in.Fields.Service = mergeService(in.Fields.Service, imported.Service, false)
			source, e = json.Marshal(struct {
				ActorID   string         `json:"actorId"`
				Selection PersonalSource `json:"selection"`
			}{sc.ActorID, *in.Source})
			if e != nil {
				return nil, e
			}
		}
		fields, e := json.Marshal(in.Fields)
		if e != nil {
			return nil, e
		}
		var id string
		e = repositoryPrepareQueryRowDocuments(ctx, tx, sc.AccountID, sc.IssuerID, pv, in.DiagnosisID, diag.Version, in.Kind, fields, in.Fields.Amount, source, &id)
		return map[string]any{"id": id, "issuerId": sc.IssuerID, "version": 1, "state": "draft", "kind": in.Kind, "emissionAvailable": false, "pending": pendingEntries(missing(p, in.Fields, in.Kind))}, e
	})
}
func (s *Service) review(ctx context.Context, tx pgx.Tx, sc Scope, d Document, connector, environment string) (Review, error) {
	if connector == "" {
		connector = d.ValidatedConnector
		environment = d.ValidatedEnvironment
	}
	p, pv, e := profile(ctx, tx, sc)
	if e != nil {
		return Review{}, e
	}
	pending := missing(p, d.Fields, d.Kind)
	if pv != d.ProfileVersion {
		pending = append(pending, "profile_version")
	}
	var dv int64
	e = repositoryReviewQueryRowDiagnoses(ctx, tx, sc.AccountID, sc.IssuerID, d.DiagnosisID, &dv)
	if e != nil {
		return Review{}, e
	}
	if dv != d.DiagnosisVersion {
		pending = append(pending, "diagnosis_version")
	}
	if d.Source != nil {
		if s.personal == nil {
			pending = append(pending, "source_unavailable")
		} else {
			var source struct {
				ActorID   string         `json:"actorId"`
				Selection PersonalSource `json:"selection"`
			}
			if e = json.Unmarshal(d.Source, &source); e != nil {
				return Review{}, e
			}
			if _, e = s.personal.ReadPersonalSource(ctx, Scope{AccountID: sc.AccountID, ActorID: source.ActorID, IssuerID: sc.IssuerID}, source.Selection); e != nil {
				pending = append(pending, "source_stale")
			}
		}
	}
	r := Review{Document: d, Profile: p, Pending: pending, ConnectorID: connector, Environment: environment}
	if c, ok := s.connectors[connector]; ok {
		capability, e := c.Capability(ctx, sc)
		if e != nil {
			return r, e
		}
		r.EmissionAvailable = capability.CanSubmit && capability.CanLookup && capability.Kind == d.Kind && capability.Environment == environment
	}
	// State version changes do not change the reviewed payload fingerprint.
	canonical := d
	canonical.State = ""
	canonical.Version = 0
	r.Hash = hash(struct {
		Document               Document
		Profile                Profile
		Connector, Environment string
	}{canonical, p, connector, environment})
	return r, nil
}
func (s *Service) DocumentCommand(ctx context.Context, sc Scope, id, operation, key string, in Command) (json.RawMessage, error) {
	permission, grant := "accounting.documents.manage", "manage"
	switch operation {
	case "review", "revoke-review":
		permission, grant = "accounting.documents.review", "review"
	case "authorize", "execute":
		permission, grant = "accounting.documents.issue", "issue"
	}
	if e := s.check(ctx, sc, permission); e != nil {
		return nil, e
	}
	return s.store.command(ctx, sc, "document."+id+"."+operation, key, in, grant, func(tx pgx.Tx) (any, error) {
		d, e := loadDocument(ctx, tx, sc, id)
		if e != nil {
			return nil, e
		}
		if d.Version != in.ExpectedVersion {
			return nil, failure("version_conflict", 409)
		}
		switch operation {
		case "edit":
			if in.Source != nil {
				if s.personal == nil {
					return nil, failure("capability_unavailable", 422)
				}
				imported, err := s.personal.ReadPersonalSource(ctx, sc, *in.Source)
				if err != nil {
					return nil, err
				}
				replaceSelected := in.Fields == nil
				if in.Fields == nil {
					copyFields := d.Fields
					in.Fields = &copyFields
				}
				in.Fields.Service = mergeService(in.Fields.Service, imported.Service, replaceSelected)
				reference, err := json.Marshal(struct {
					ActorID   string         `json:"actorId"`
					Selection PersonalSource `json:"selection"`
				}{sc.ActorID, *in.Source})
				if err != nil {
					return nil, err
				}
				if err = updateDocumentSource(ctx, tx, sc, id, reference); err != nil {
					return nil, err
				}
			}
			if in.Fields == nil || !validFields(*in.Fields) {
				return nil, failure("invalid_input", 400)
			}
			if d.State == "submitting" || d.State == "unknown" || d.State == "authorized" {
				return nil, failure("state_conflict", 409)
			}
			var active bool
			e = repositoryDocumentCommandQueryRowExecutions(ctx, tx, sc.AccountID, sc.IssuerID, id, &active)
			if e != nil {
				return nil, e
			}
			if active {
				return nil, failure("execution_in_progress", 409)
			}
			_, pv, e := profile(ctx, tx, sc)
			if e != nil {
				return nil, e
			}
			var dv int64
			e = repositoryDocumentCommandQueryRowDiagnoses(ctx, tx, sc.AccountID, sc.IssuerID, d.DiagnosisID, &dv)
			if e != nil {
				return nil, e
			}
			fields, e := json.Marshal(in.Fields)
			if e != nil {
				return nil, e
			}
			_, e = repositoryDocumentCommandExecDocuments(ctx, tx, sc.AccountID, sc.IssuerID, id, fields, in.Fields.Amount, pv, dv)
			if e != nil {
				return nil, e
			}
			if e = invalidate(ctx, tx, sc, id); e != nil {
				return nil, e
			}
			return loadDocument(ctx, tx, sc, id)
		case "validate":
			if d.State != "draft" && d.State != "rejected" {
				return nil, failure("state_conflict", 409)
			}
			r, e := s.review(ctx, tx, sc, d, in.ConnectorID, in.Environment)
			if e != nil {
				return nil, e
			}
			if len(r.Pending) > 0 {
				return nil, failure("validation_failed", 422)
			}
			c, ok := s.connectors[in.ConnectorID]
			if !ok {
				if in.ConnectorID != "" {
					return nil, failure("coverage_unavailable", 422)
				}
				if e = recordValidation(ctx, tx, sc, id, in.ConnectorID, in.Environment); e != nil {
					return nil, e
				}
				return loadDocument(ctx, tx, sc, id)
			}
			if !r.EmissionAvailable {
				return nil, failure("coverage_unavailable", 422)
			}
			pending, e := c.Validate(ctx, sc, Submission{Document: d, Environment: in.Environment})
			if e != nil {
				return nil, e
			}
			if len(pending) > 0 {
				return nil, failure("validation_failed", 422)
			}
			if e = recordValidation(ctx, tx, sc, id, in.ConnectorID, in.Environment); e != nil {
				return nil, e
			}
			return loadDocument(ctx, tx, sc, id)
		case "review":
			if d.State != "validated" || !in.Confirmed {
				return nil, failure("state_conflict", 409)
			}
			r, e := s.review(ctx, tx, sc, d, in.ConnectorID, in.Environment)
			if e != nil {
				return nil, e
			}
			if len(r.Pending) > 0 || r.Hash != in.SnapshotHash || in.ExpectedProfileVersion != d.ProfileVersion {
				return nil, failure("snapshot_stale", 409)
			}
			b, e := json.Marshal(r)
			if e != nil {
				return nil, e
			}
			var reviewID string
			e = repositoryDocumentCommandQueryRowDocumentReviews(ctx, tx, sc.AccountID, sc.IssuerID, id, d.Version, d.ProfileVersion, b, r.Hash, sc.ActorID, &reviewID)
			if e != nil {
				return nil, e
			}
			if e = setDocumentState(ctx, tx, sc, id, "reviewed"); e != nil {
				return nil, e
			}
			return map[string]any{"documentId": id, "version": d.Version + 1, "state": "reviewed", "reviewId": reviewID, "snapshotHash": r.Hash, "emissionAvailable": false}, nil
		case "revoke-review":
			if !validID(in.ReviewID) {
				return nil, failure("not_found", 404)
			}
			var present bool
			e = repositoryDocumentCommandQueryRowDocumentReviews2(ctx, tx, sc.AccountID, sc.IssuerID, id, in.ReviewID, &present)
			if e != nil {
				return nil, e
			}
			if !present {
				return nil, failure("not_found", 404)
			}
			if d.State != "reviewed" && d.State != "validated" {
				return nil, failure("state_conflict", 409)
			}
			if e = invalidate(ctx, tx, sc, id); e != nil {
				return nil, e
			}
			if e = setDocumentState(ctx, tx, sc, id, "draft"); e != nil {
				return nil, e
			}
			return loadDocument(ctx, tx, sc, id)
		case "authorize":
			return s.authorize(ctx, tx, sc, d, in)
		case "execute":
			return s.execute(ctx, tx, sc, d, in)
		}
		return nil, failure("invalid_input", 400)
	})
}
func setDocumentState(ctx context.Context, tx pgx.Tx, s Scope, id, state string) error {
	_, e := repositorySetDocumentStateExecDocuments(ctx, tx, s.AccountID, s.IssuerID, id, state)
	return e
}

func mergeService(existing, selected *ServiceFields, replace bool) *ServiceFields {
	if existing == nil {
		existing = &ServiceFields{}
	}
	if selected == nil {
		return existing
	}
	copy := *existing
	if selected.Description != nil && (replace || copy.Description == nil) {
		copy.Description = selected.Description
	}
	if selected.ServiceCode != nil && (replace || copy.ServiceCode == nil) {
		copy.ServiceCode = selected.ServiceCode
	}
	if selected.CNAE != nil && (replace || copy.CNAE == nil) {
		copy.CNAE = selected.CNAE
	}
	if selected.NBS != nil && (replace || copy.NBS == nil) {
		copy.NBS = selected.NBS
	}
	return &copy
}
