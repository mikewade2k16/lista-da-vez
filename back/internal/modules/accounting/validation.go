package accounting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"time"
)

var moneyPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,17})\.[0-9]{2}$`)
var quantityPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)
var ratePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,2})(\.[0-9]{1,6})?$`)

func dateValid(v *string) bool {
	if v == nil {
		return true
	}
	t, e := time.Parse("2006-01-02", *v)
	return e == nil && t.Format("2006-01-02") == *v
}
func hash(v any) string {
	b, _ := json.Marshal(v)
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}
func nonempty(v *string) bool { return v != nil && strings.TrimSpace(*v) != "" }
func validFields(f Fields) bool {
	if !dateValid(f.CompetenceDate) || len(f.Items) > 100 {
		return false
	}
	if f.Transport != nil && f.Transport.FreightAmount != nil && !moneyPattern.MatchString(*f.Transport.FreightAmount) {
		return false
	}
	if f.Payment != nil && f.Payment.Amount != nil && !moneyPattern.MatchString(*f.Payment.Amount) {
		return false
	}
	if f.Amount != nil && !moneyPattern.MatchString(*f.Amount) {
		return false
	}
	if f.Service != nil && f.Service.ISSRate != nil && !ratePattern.MatchString(*f.Service.ISSRate) {
		return false
	}
	for _, i := range f.Items {
		q, ok := new(big.Rat).SetString(i.Quantity)
		if !ok || q.Sign() <= 0 || !quantityPattern.MatchString(i.Quantity) || !moneyPattern.MatchString(i.UnitPrice) {
			return false
		}
		for _, v := range []*string{i.Discount, i.Addition} {
			if v != nil && !moneyPattern.MatchString(*v) {
				return false
			}
		}
	}
	return true
}
func missing(p Profile, f Fields, kind string) []string {
	out := []string{}
	for _, v := range []struct {
		name  string
		value *string
	}{{"document", p.Document}, {"taxRegime", p.TaxRegime}, {"municipalityCode", p.MunicipalityCode}, {"state", p.State}, {"competenceDate", f.CompetenceDate}, {"amount", f.Amount}} {
		if !nonempty(v.value) {
			out = append(out, v.name)
		}
	}
	if f.Amount != nil && *f.Amount == "0.00" {
		out = append(out, "positiveAmount")
	}
	switch kind {
	case "nfse":
		if f.Service == nil || !nonempty(f.Service.Description) {
			out = append(out, "service.description")
		}
		if f.Customer == nil || !nonempty(f.Customer.Name) || !nonempty(f.Customer.Document) {
			out = append(out, "customer")
		}
	case "nfe":
		if len(f.Items) == 0 {
			out = append(out, "items")
		}
		if f.Operation == nil {
			out = append(out, "operation")
		}
		if f.Recipient == nil || !nonempty(f.Recipient.Document) {
			out = append(out, "recipient")
		}
	}
	return out
}
func validAnswers(in DiagnosisInput) bool {
	if !dateValid(in.CompetenceDate) || (in.Intent != "service" && in.Intent != "product" && in.Intent != "mixed") {
		return false
	}
	allowed := strings.Fields("document legalName issuerType taxRegime municipality municipalityCode state address municipalRegistration stateRegistration service customer items recipient operation amount access")
	for name, a := range in.Answers {
		found := false
		for _, key := range allowed {
			if name == key {
				found = true
			}
		}
		if !found {
			return false
		}
		// Confirmation must also pass the owner verifier in SaveDiagnosis.
		switch a.Confirmation {
		case "confirmed":
			if a.Value == nil || len(*a.Value) > 4000 || !nonempty(a.EvidenceRef) || a.Origin == nil || (*a.Origin != "official_evidence" && *a.Origin != "operational_document") {
				return false
			}
		case "unknown":
			if a.Value != nil || a.Origin != nil || a.EvidenceRef != nil {
				return false
			}
		case "declared":
			if a.Value == nil || len(*a.Value) > 4000 || a.Origin == nil || *a.Origin != "declaration" || a.EvidenceRef != nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}

type Pending struct {
	Field  string   `json:"field"`
	Code   string   `json:"code"`
	Blocks []string `json:"blocks"`
}

func pendingEntries(fields []string) []Pending {
	out := make([]Pending, 0, len(fields))
	for _, f := range fields {
		out = append(out, Pending{Field: f, Code: "missing_data", Blocks: []string{"review", "submit"}})
	}
	return out
}
