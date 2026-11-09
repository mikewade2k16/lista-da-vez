package finance

import (
	"context"
	"errors"
	"testing"
)

func TestFiscalValidationPreservesMissingAndRejectsUnsafeData(t *testing.T) {
	for _, p := range []FiscalProfile{{}, {PortalURL: "https://www.nfse.gov.br/EmissorNacional"}, {Document: "12345678901", State: "AL"}, {Document: "AB123456789012"}} {
		if err := normalizeFiscalProfile(&p); err != nil {
			t.Errorf("valid profile rejected: %v", err)
		}
	}
	for _, p := range []FiscalProfile{{PortalURL: "http://example.com"}, {PortalURL: "https://user:secret@example.com"}, {PortalURL: "https://127.0.0.1"}, {PortalURL: "https://portal.local"}, {PortalURL: "https://example.com?token=secret"}, {Document: "123"}, {State: "XX"}, {PostalCode: "abc"}, {MunicipalityCode: "123"}, {IssuerType: "invalid"}, {Email: "not-email"}} {
		if err := normalizeFiscalProfile(&p); !errors.Is(err, ErrFiscalInvalid) {
			t.Errorf("invalid profile accepted: %+v", p)
		}
	}
	if _, err := NewFiscalService(nil, nil).SaveCredentials(context.Background(), "account", "user", FiscalCredentialInput{}); !errors.Is(err, ErrFiscalVaultUnavailable) {
		t.Fatal("missing vault silently accepted")
	}
}
