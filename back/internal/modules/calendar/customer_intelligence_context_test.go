package calendar

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectBusinessContextKeepsBrandVoiceInBusinessSection(t *testing.T) {
	profile := ClientProfile{
		Segment:     "joias",
		Positioning: "luxo acessivel",
		BrandVoice:  "consultivo e direto",
		Extra: ProfileExtra{
			Audience: "compradores recorrentes",
		},
	}
	sections, warnings, err := projectBusinessContext(
		profile,
		[]string{businessContextSectionStrategy, businessContextSectionVoice},
		defaultBusinessContextMaxBytes,
	)
	if err != nil {
		t.Fatalf("projectBusinessContext: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if _, exists := sections[businessContextSectionVoice]; !exists {
		t.Fatal("brand voice must remain in the business voice section")
	}
	if _, exists := sections["subject"]; exists {
		t.Fatal("business context must not create a subject section")
	}
	var voice map[string]string
	if err := json.Unmarshal(sections[businessContextSectionVoice], &voice); err != nil {
		t.Fatalf("decode voice: %v", err)
	}
	if voice["brand_voice"] != profile.BrandVoice {
		t.Fatalf("unexpected voice: %v", voice)
	}
}

func TestProjectBusinessContextBoundsFieldsAndTotalBytes(t *testing.T) {
	profile := ClientProfile{
		Description: strings.Repeat("x", maxBusinessContextFieldRunes+50),
		BrandVoice:  strings.Repeat("y", 100),
	}
	sections, warnings, err := projectBusinessContext(
		profile,
		[]string{businessContextSectionStrategy, businessContextSectionVoice},
		180,
	)
	if err != nil {
		t.Fatalf("projectBusinessContext: %v", err)
	}
	if len(sections) != 1 || sections[businessContextSectionVoice] == nil {
		t.Fatalf("expected only the bounded voice section, got %v", sections)
	}
	if !containsString(warnings, businessContextWarningFieldTrimmed) ||
		!containsString(warnings, businessContextWarningByteLimit) {
		t.Fatalf("expected trim and byte warnings, got %v", warnings)
	}
}

func TestNormalizeBusinessContextSectionsRejectsUnknown(t *testing.T) {
	if _, err := normalizeBusinessContextSections([]string{"strategy", "raw_sql"}); err == nil {
		t.Fatal("expected unknown section to be rejected")
	}
}
