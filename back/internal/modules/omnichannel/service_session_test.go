package omnichannel

import (
	"context"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

func TestSessionViewDoesNotTreatConfiguredPhoneAsConnected(t *testing.T) {
	phone := "5511999999999"
	svc := &SessionService{qr: newQRCache()}
	inst := sessionInstance{
		InstanceName: "primary",
		Provider:     "evolution",
		PhoneNumber:  &phone,
		IsActive:     true,
	}

	view := svc.viewFor(context.Background(), "account-a", inst, nil)
	if view.Connected {
		t.Fatal("configured phone number must not be treated as an active provider session")
	}
	if got := view.ConnectionState["instance"].(map[string]any)["state"]; got != "close" {
		t.Fatalf("connection state = %v, want close", got)
	}
}

func TestSessionViewUsesAuthoritativeProviderState(t *testing.T) {
	phone := "5511999999999"
	svc := &SessionService{qr: newQRCache()}
	inst := sessionInstance{InstanceName: "primary", Provider: "evolution", PhoneNumber: &phone}

	connecting := svc.viewFor(context.Background(), "account-a", inst, &channel.SessionState{Connected: false})
	if connecting.Connected {
		t.Fatal("provider connecting state must remain disconnected")
	}

	connected := svc.viewFor(context.Background(), "account-a", inst, &channel.SessionState{Connected: true})
	if !connected.Connected {
		t.Fatal("provider connected state must be reflected in the session view")
	}
	if got := connected.ConnectionState["instance"].(map[string]any)["state"]; got != "open" {
		t.Fatalf("connection state = %v, want open", got)
	}
}
