package accounting

import (
	"context"
	"encoding/json"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	obligationQualification ObligationQualification
	obligationFinance       ObligationFinance
	artifactReader          ArtifactReader
	knowledge               *knowledge.Service
	evidence                EvidenceVerifier
	store                   *Store
	policy                  Policy
	queue                   Queue
	personal                PersonalReader
	artifacts               ArtifactStore
	connectors              map[string]Connector
}

func NewService(store *Store, policy Policy, queue Queue) *Service {
	return &Service{store: store, policy: policy, queue: queue, connectors: map[string]Connector{}}
}

// WithConnectors is composition-only. Production composition deliberately supplies none.
func (s *Service) WithConnectors(connectors map[string]Connector) *Service {
	s.connectors = connectors
	return s
}
func (s *Service) WithPersonalReader(reader PersonalReader) *Service { s.personal = reader; return s }
func (s *Service) WithArtifacts(storage ArtifactStore) *Service      { s.artifacts = storage; return s }
func (s *Service) WithEvidenceVerifier(v EvidenceVerifier) *Service  { s.evidence = v; return s }
func (s *Service) check(ctx context.Context, sc Scope, permission string) error {
	if !validID(sc.AccountID) || !validID(sc.ActorID) {
		return failure("unauthorized", 401)
	}
	if s.policy == nil {
		return failure("permission_denied", 403)
	}
	return s.policy.Check(ctx, sc, permission)
}
func (s *Service) CreateIssuer(ctx context.Context, sc Scope, key string, in Command) (json.RawMessage, error) {
	if e := s.check(ctx, sc, "accounting.issuers.manage"); e != nil {
		return nil, e
	}
	if in.Profile == nil || strings.TrimSpace(in.Profile.LegalName) == "" || len(in.Profile.LegalName) > 200 || in.ExpectedVersion != 0 {
		return nil, failure("invalid_input", 400)
	}
	if in.EnableIssue {
		if e := s.check(ctx, sc, "accounting.documents.issue"); e != nil {
			return nil, e
		}
	}
	return s.store.command(ctx, sc, "issuer.create", key, in, "manage", func(tx pgx.Tx) (any, error) {
		var id string
		e := repositoryCreateIssuerQueryRowIssuers(ctx, tx, sc.AccountID, &id)
		if e != nil {
			return nil, e
		}
		// Issuance still requires account RBAC; creator's grant alone grants no authority.
		_, e = repositoryCreateIssuerExecIssuerMembers(ctx, tx, sc.AccountID, id, sc.ActorID, initialGrants(in.EnableIssue))
		if e != nil {
			return nil, e
		}
		data, e := json.Marshal(in.Profile)
		if e != nil {
			return nil, e
		}
		_, e = repositoryCreateIssuerExecIssuerProfiles(ctx, tx, sc.AccountID, id, data, sc.ActorID)
		return map[string]any{"id": id, "version": 1, "profileVersion": 1, "accessVersion": 1, "confirmation": "declared", "capabilities": map[string]bool{"diagnose": true, "prepare": true, "submit": false}}, e
	})
}
func (s *Service) UpdateIssuer(ctx context.Context, sc Scope, key string, in Command) (json.RawMessage, error) {
	if e := s.check(ctx, sc, "accounting.issuers.manage"); e != nil {
		return nil, e
	}
	if in.Profile == nil || strings.TrimSpace(in.Profile.LegalName) == "" || len(in.Profile.LegalName) > 200 {
		return nil, failure("invalid_input", 400)
	}
	return s.store.command(ctx, sc, "issuer.update", key, in, "manage", func(tx pgx.Tx) (any, error) {
		var v int64
		e := repositoryUpdateIssuerQueryRowIssuers(ctx, tx, sc.AccountID, sc.IssuerID, in.ExpectedVersion, &v)
		if e != nil {
			return nil, failure("version_conflict", 409)
		}
		b, e := json.Marshal(in.Profile)
		if e != nil {
			return nil, e
		}
		var pv int64
		e = repositoryUpdateIssuerQueryRowIssuerProfiles(ctx, tx, sc.AccountID, sc.IssuerID, &pv)
		if e != nil {
			return nil, e
		}
		_, e = repositoryUpdateIssuerExecIssuerProfiles(ctx, tx, sc.AccountID, sc.IssuerID, pv, b, sc.ActorID)
		return map[string]any{"id": sc.IssuerID, "version": v, "profileVersion": pv}, e
	})
}
func (s *Service) SaveMember(ctx context.Context, sc Scope, id, key string, in Command) (json.RawMessage, error) {
	if e := s.check(ctx, sc, "accounting.issuers.manage"); e != nil {
		return nil, e
	}
	if !validID(id) {
		return nil, failure("not_found", 404)
	}
	if in.Grants == nil || (in.Status != "active" && in.Status != "inactive") {
		return nil, failure("invalid_input", 400)
	}
	valid := map[string]string{"view": "accounting.issuers.view", "manage": "accounting.issuers.manage", "review": "accounting.documents.review", "issue": "accounting.documents.issue", "knowledge_view": "accounting.knowledge.view", "knowledge_manage": "accounting.knowledge.manage"}
	for _, g := range in.Grants {
		perm, ok := valid[g]
		if !ok {
			return nil, failure("invalid_input", 400)
		}
		if e := s.check(ctx, sc, perm); e != nil {
			return nil, e
		}
	}
	member, e := s.policy.Member(ctx, sc.AccountID, id)
	if e != nil {
		return nil, e
	}
	if !member {
		return nil, failure("not_found", 404)
	}
	return s.store.command(ctx, sc, "member."+id, key, in, "manage", func(tx pgx.Tx) (any, error) {
		for _, g := range in.Grants {
			if g == "issue" {
				if _, err := issuerLock(ctx, tx, sc, "issue"); err != nil {
					if !in.Confirmed {
						return nil, err
					}
					active, e := activeIssueGrant(ctx, tx, sc)
					if e != nil {
						return nil, e
					}
					if active {
						return nil, err
					}
				}
			}
		}
		var version int64
		e := repositorySaveMemberQueryRowIssuerMembers(ctx, tx, sc.AccountID, sc.IssuerID, id, in.Grants, in.Status, in.ExpectedVersion, &version)
		if in.ExpectedVersion > 0 {
			e = repositorySaveMemberQueryRowIssuerMembers2(ctx, tx, sc.AccountID, sc.IssuerID, id, in.Grants, in.Status, in.ExpectedVersion, &version)
		}
		if e != nil {
			return nil, failure("version_conflict", 409)
		}
		_, e = repositorySaveMemberExecIssuers(ctx, tx, sc.AccountID, sc.IssuerID)
		return map[string]any{"userId": id, "version": version, "status": in.Status, "grants": in.Grants}, e
	})
}
func (s *Service) SaveDiagnosis(ctx context.Context, sc Scope, key string, in DiagnosisInput) (json.RawMessage, error) {
	if e := s.check(ctx, sc, "accounting.issuers.manage"); e != nil {
		return nil, e
	}
	if !validAnswers(in) || (in.DiagnosisID != nil && !validID(*in.DiagnosisID)) {
		return nil, failure("invalid_input", 400)
	}
	return s.store.command(ctx, sc, "diagnosis.save", key, in, "manage", func(tx pgx.Tx) (any, error) {
		for field, answer := range in.Answers {
			if answer.Confirmation == "confirmed" {
				if s.evidence == nil {
					return nil, failure("evidence_verification_unavailable", 422)
				}
				if e := s.evidence.Verify(ctx, sc, field, answer, in.CompetenceDate); e != nil {
					return nil, e
				}
			}
		}
		p, pv, e := profile(ctx, tx, sc)
		if e != nil {
			return nil, e
		}
		if pv != in.ExpectedProfileVersion {
			return nil, failure("version_conflict", 409)
		}
		kind := "nfse"
		if in.Intent == "product" {
			kind = "nfe"
		}
		pending := pendingEntries(missing(p, Fields{CompetenceDate: in.CompetenceDate}, kind))
		a, e := json.Marshal(in.Answers)
		if e != nil {
			return nil, e
		}
		b, e := json.Marshal(pending)
		if e != nil {
			return nil, e
		}
		var id string
		var v int64
		if in.DiagnosisID == nil {
			if in.ExpectedVersion != 0 {
				return nil, failure("version_conflict", 409)
			}
			e = repositorySaveDiagnosisQueryRowDiagnoses(ctx, tx, sc.AccountID, sc.IssuerID, pv, in.Intent, in.CompetenceDate, a, b, &id, &v)
		} else {
			id = *in.DiagnosisID
			e = repositorySaveDiagnosisQueryRowDiagnoses2(ctx, tx, sc.AccountID, sc.IssuerID, id, pv, in.Intent, in.CompetenceDate, a, b, in.ExpectedVersion, &v)
			if e != nil {
				return nil, diagnosisConflict(ctx, tx, sc, id, e)
			}
		}
		return map[string]any{"id": id, "issuerId": sc.IssuerID, "version": v, "profileVersion": pv, "intent": in.Intent, "competenceDate": in.CompetenceDate, "answers": in.Answers, "pending": pending, "nextQuestionId": nextQuestion(p, in)}, e
	})
}
func nextQuestion(p Profile, in DiagnosisInput) string {
	present := func(name string, value *string) bool {
		a, ok := in.Answers[name]
		return nonempty(value) || (ok && (a.Confirmation == "declared" || a.Confirmation == "confirmed") && nonempty(a.Value))
	}
	switch {
	case !present("document", p.Document):
		return "Q3"
	case !present("municipalityCode", p.MunicipalityCode):
		return "Q4"
	case in.CompetenceDate == nil:
		return "Q5"
	case !present("taxRegime", p.TaxRegime):
		return "Q6"
	case in.Intent == "product" && !present("items", nil):
		return "Q10"
	case in.Intent != "product" && !present("service", nil):
		return "Q8"
	case in.Intent != "product" && !present("customer", nil):
		return "Q9"
	case !present("amount", nil):
		return "Q12"
	default:
		return "Q14"
	}
}
func diagnosisConflict(ctx context.Context, tx pgx.Tx, s Scope, id string, e error) error {
	if _, err := readDiagnosis(ctx, tx, s, id); err != nil {
		return err
	}
	return failure("version_conflict", 409)
}

func initialGrants(enableIssue bool) []string {
	grants := []string{"view", "manage", "review", "knowledge_view", "knowledge_manage"}
	if enableIssue {
		grants = append(grants, "issue")
	}
	return grants
}
