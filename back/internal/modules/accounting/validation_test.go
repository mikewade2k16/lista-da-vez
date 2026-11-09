package accounting

import "testing"

func TestExactDecimalAndCivilDateValidation(t *testing.T) {
	for _, v := range []string{"0.00", "125.05", "999999999999999999.99"} {
		if !validFields(Fields{Amount: &v}) {
			t.Fatal(v)
		}
	}
	for _, v := range []string{"1e2", "NaN", "1.001", "-1.00", "1000000000000000000.00", "1,00"} {
		if validFields(Fields{Amount: &v}) {
			t.Fatal(v)
		}
	}
	if dateValid(ptr("2026-02-30")) || dateValid(ptr("2026-11-03T00:00:00Z")) {
		t.Fatal("invalid civil date accepted")
	}
	if !dateValid(ptr("2026-11-03")) {
		t.Fatal("valid date rejected")
	}
	if validAnswers(DiagnosisInput{Intent: "service", Answers: map[string]Answer{"taxRegime": {Value: ptr("synthetic"), Confirmation: "confirmed", Origin: ptr("declaration")}}}) {
		t.Fatal("declaration promoted to documentary evidence")
	}
}

func TestDiagnosisProgressesWithoutPromotingDeclarationsToProfile(t *testing.T) {
	p := Profile{LegalName: "Synthetic"}
	in := DiagnosisInput{Intent: "service", CompetenceDate: ptr("2026-11-03"), Answers: map[string]Answer{}}
	if nextQuestion(p, in) != "Q3" {
		t.Fatal("missing identity not asked")
	}
	for _, name := range []string{"document", "municipalityCode", "taxRegime", "service", "customer", "amount"} {
		in.Answers[name] = Answer{Value: ptr("synthetic"), Confirmation: "declared", Origin: ptr("declaration")}
	}
	if nextQuestion(p, in) != "Q14" {
		t.Fatal("saved answers were asked again")
	}
	if p.Document != nil {
		t.Fatal("declaration silently promoted to issuer profile")
	}
}
