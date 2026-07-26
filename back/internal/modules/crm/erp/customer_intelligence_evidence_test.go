package erp

import (
	"strings"
	"testing"
	"time"
)

func TestProjectCustomerIntelligenceERPFieldsIsAllowlistedAndMinimized(t *testing.T) {
	occurredAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	total := int64(125_000)
	raw := erpEvidenceRaw{
		EntityType:       CustomerIntelligenceEntityOrder,
		SourceBatchDate:  time.Date(2026, time.July, 23, 0, 0, 0, 0, time.UTC),
		OccurredAt:       &occurredAt,
		TotalAmountCents: &total,
		SKUs:             []string{"SKU-1", "SKU-1", "SKU-2"},
		PaymentType:      "cartao",
	}
	fields := projectCustomerIntelligenceERPFields(raw, []string{
		"order_date",
		"total_amount_cents",
		"skus",
		"cpf",
	})
	if fields["total_amount_cents"] != total {
		t.Fatalf("unexpected total: %v", fields)
	}
	if _, leaked := fields["cpf"]; leaked {
		t.Fatal("identity field must never be projected by the evidence facade")
	}
	skus, ok := fields["skus"].([]string)
	if !ok || len(skus) != 2 {
		t.Fatalf("expected deduplicated SKUs, got %#v", fields["skus"])
	}
}

func TestProjectCustomerIntelligenceCustomerUsesExactProfileProjection(t *testing.T) {
	raw := erpEvidenceRaw{
		EntityType:      CustomerIntelligenceEntityCustomer,
		SourceBatchDate: time.Date(2026, time.July, 23, 0, 0, 0, 0, time.UTC),
		Name:            "NOME LEGAL",
		Nickname:        "Nome preferido",
		City:            "Aracaju",
		Tags:            strings.Repeat("x", maxCustomerIntelligenceTextRunes+20),
	}
	fields := projectCustomerIntelligenceERPFields(raw, []string{
		"preferred_name",
		"city",
		"tags",
		"email",
	})
	if fields["preferred_name"] != "Nome preferido" || fields["city"] != "Aracaju" {
		t.Fatalf("unexpected customer projection: %v", fields)
	}
	if len([]rune(fields["tags"].(string))) != maxCustomerIntelligenceTextRunes {
		t.Fatal("tags must be bounded")
	}
	if _, leaked := fields["email"]; leaked {
		t.Fatal("raw identity must not be projected")
	}
}
