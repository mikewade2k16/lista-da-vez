package accounting

import (
	"context"
	"encoding/json"
	"testing"
)

// CT-10: saved identity avoids repeated identity questions, but does not invent
// the service, recipient or amount of a new operation.
func TestPilotAsksOnlyMissingOperationData(t *testing.T) {
	p := Profile{Document: ptr("00000000000000"), MunicipalityCode: ptr("0000000"), TaxRegime: ptr("synthetic")}
	in := DiagnosisInput{Intent: "service", CompetenceDate: ptr("2026-11-04"), Answers: map[string]Answer{}}
	for _, step := range []struct{ want, field string }{{"Q8", "service"}, {"Q9", "customer"}, {"Q12", "amount"}, {"Q14", ""}} {
		if got := nextQuestion(p, in); got != step.want {
			t.Fatalf("wanted %s, got %s", step.want, got)
		}
		if step.field != "" {
			in.Answers[step.field] = Answer{Value: ptr("synthetic"), Confirmation: "declared", Origin: ptr("declaration")}
		}
	}
	// Document alone cannot establish municipality, regime or service description.
	if got := nextQuestion(Profile{Document: p.Document}, DiagnosisInput{Intent: "service", Answers: map[string]Answer{}}); got != "Q4" {
		t.Fatalf("identity inferred missing registration: %s", got)
	}
	in.Answers["amount"] = Answer{Confirmation: "unknown"}
	if got := nextQuestion(p, in); got != "Q12" {
		t.Fatalf("unknown treated as answered: %s", got)
	}
}

func TestPilotSavedDiagnosisSurvivesRestartWithoutIssuance(t *testing.T) {
	s, sc, pool := fixture(t)
	ctx := context.Background()
	in := DiagnosisInput{ExpectedProfileVersion: 1, Intent: "service", CompetenceDate: ptr("2026-11-04"), Answers: map[string]Answer{}}
	raw, err := s.SaveDiagnosis(ctx, sc, "pilot-diagnosis-start", in)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, raw, "nextQuestionId") != "Q8" {
		t.Fatal("saved issuer identity asked again")
	}
	in.DiagnosisID = ptr(field(t, raw, "id"))
	in.ExpectedVersion = 1
	for _, name := range []string{"service", "customer", "amount"} {
		in.Answers[name] = Answer{Value: ptr("synthetic"), Confirmation: "declared", Origin: ptr("declaration")}
	}
	raw, err = s.SaveDiagnosis(ctx, sc, "pilot-diagnosis-complete", in)
	if err != nil {
		t.Fatal(err)
	}
	reloaded := NewService(NewStore(pool), &testPolicy{}, s.queue)
	replay, err := reloaded.SaveDiagnosis(ctx, sc, "pilot-diagnosis-complete", in)
	if err != nil {
		t.Fatal(err)
	}
	var saved, again struct {
		ID, NextQuestionID string
		Version            int64
		Answers            map[string]Answer
	}
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(replay, &again); err != nil {
		t.Fatal(err)
	}
	if saved.ID != again.ID || again.Version != 2 || again.NextQuestionID != "Q14" || len(again.Answers) != 3 {
		t.Fatal("persisted diagnosis/replay changed")
	}
	var effects int
	if err = pool.QueryRow(ctx, `select (select count(*) from accounting.submission_authorizations where account_id=$1::uuid)+(select count(*) from accounting.executions where account_id=$1::uuid)`, sc.AccountID).Scan(&effects); err != nil {
		t.Fatal(err)
	}
	if effects != 0 {
		t.Fatal("diagnosis created issuance effect")
	}
}
