package omnichannel

import (
	"encoding/json"
	"testing"
)

func TestNormalizeCRMTags(t *testing.T) {
	got, set, err := normalizeCRMTags(json.RawMessage(`[" Cliente ","cliente","Instagram"]`))
	if err != nil || !set || string(got) != `["cliente","instagram"]` {
		t.Fatalf("tags=%s set=%v err=%v", got, set, err)
	}
	if _, _, err := normalizeCRMTags(json.RawMessage(`{"x":1}`)); err != ErrInvalidBody {
		t.Fatalf("object tags err=%v, want invalid body", err)
	}
}

func TestNormalizeCRMCustomFieldsRejectsNonObject(t *testing.T) {
	if _, _, err := normalizeCRMCustomFields(json.RawMessage(`[1]`)); err != ErrInvalidBody {
		t.Fatalf("array custom fields err=%v", err)
	}
	got, set, err := normalizeCRMCustomFields(json.RawMessage(`{"lead":"landing"}`))
	if err != nil || !set || string(got) != `{"lead":"landing"}` {
		t.Fatalf("custom fields=%s set=%v err=%v", got, set, err)
	}
}

func TestLeadOriginAllowlistFailClosed(t *testing.T) {
	if leadOriginAllowed("https://landing.example", nil) {
		t.Fatal("empty allowlist must not accept browser origin")
	}
	if !leadOriginAllowed("https://landing.example", []string{"https://landing.example"}) {
		t.Fatal("configured origin should be accepted")
	}
	if leadOriginAllowed("https://evil.example", []string{"https://landing.example"}) {
		t.Fatal("unexpected origin accepted")
	}
}

func TestValidCRMUUIDPair(t *testing.T) {
	const a = "00000000-0000-0000-0000-000000000001"
	const b = "00000000-0000-0000-0000-000000000002"
	if !validCRMUUIDPair(a, b) || validCRMUUIDPair(a, a) || validCRMUUIDPair("bad", b) {
		t.Fatal("uuid pair validation failed")
	}
}

func TestNormalizeContactSegmentFilterUsesClosedCRMShape(t *testing.T) {
	got, err := normalizeSegmentFilter(json.RawMessage(`{"status":"lead","channel":"whatsapp","tag":" Cliente "}`))
	if err != nil {
		t.Fatalf("normalize segment: %v", err)
	}
	if string(got) != `{"search":"","channel":"WHATSAPP","status":"new_lead","tag":"cliente","ownerId":"","source":"","lastSeenAfter":null,"lastSeenBefore":null}` {
		t.Fatalf("normalized segment=%s", got)
	}
	if _, err := normalizeSegmentFilter(json.RawMessage(`{"unknown":"x"}`)); err != ErrInvalidBody {
		t.Fatalf("unknown field err=%v", err)
	}
}
