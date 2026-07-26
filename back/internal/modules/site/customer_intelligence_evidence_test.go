package site

import (
	"strings"
	"testing"
	"time"
)

func TestProjectCustomerIntelligenceLeadExcludesIdentityAndRawPayload(t *testing.T) {
	lead := LeadView{
		SourceLabel:  "landing-page",
		Nome:         "Pessoa",
		Email:        "pessoa@example.com",
		Telefone:     "79999999999",
		Page:         "/campanha",
		Cupom:        "VIP",
		Consent:      true,
		ConsentLabel: "marketing",
		TrackingData: `{"utm_source":"ads"}`,
		PayloadRaw:   `{"secret":"raw"}`,
		Status:       "qualified",
		CreatedAt:    time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC),
	}
	fields := projectCustomerIntelligenceLead(lead, []string{
		"source_label",
		"page",
		"consent",
		"email",
		"payload_raw",
	})
	if fields["source_label"] != "landing-page" ||
		fields["page"] != "/campanha" ||
		fields["consent"] != true {
		t.Fatalf("unexpected projection: %v", fields)
	}
	if _, leaked := fields["email"]; leaked {
		t.Fatal("identity belongs to Customer Data and must not be copied")
	}
	if _, leaked := fields["payload_raw"]; leaked {
		t.Fatal("raw webhook payload must not be copied")
	}
}

func TestProjectCustomerIntelligenceLeadBoundsText(t *testing.T) {
	lead := LeadView{Page: strings.Repeat("x", maxCustomerIntelligenceSiteTextRunes+10)}
	fields := projectCustomerIntelligenceLead(lead, []string{"page"})
	if len([]rune(fields["page"].(string))) != maxCustomerIntelligenceSiteTextRunes {
		t.Fatal("site evidence text must be bounded")
	}
}
