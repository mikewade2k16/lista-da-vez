package realtime

import (
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestRealtimeTicketIsSingleUse(t *testing.T) {
	service := &Service{}
	principal := auth.Principal{
		UserID:   "user-1",
		TenantID: "tenant-1",
	}

	ticket, err := service.issueRealtimeTicket(principal, "account-1")
	if err != nil {
		t.Fatalf("issueRealtimeTicket() error = %v", err)
	}

	resolved, err := service.consumeRealtimeTicket(ticket)
	if err != nil {
		t.Fatalf("consumeRealtimeTicket() error = %v", err)
	}
	if resolved.UserID != "user-1" {
		t.Fatalf("resolved.UserID = %q, want user-1", resolved.UserID)
	}
	if resolved.AccountID != "account-1" {
		t.Fatalf("resolved.AccountID = %q, want account-1", resolved.AccountID)
	}

	_, err = service.consumeRealtimeTicket(ticket)
	if !errors.Is(err, errRealtimeTicketInvalid) {
		t.Fatalf("second consume error = %v, want errRealtimeTicketInvalid", err)
	}
}

func TestRealtimeTicketExpires(t *testing.T) {
	service := &Service{}
	service.tickets.items.Store("expired-ticket", realtimeTicket{
		Principal: auth.Principal{UserID: "user-1"},
		UserID:    "user-1",
		AccountID: "account-1",
		ExpiresAt: time.Now().UTC().Add(-time.Second),
	})

	_, err := service.consumeRealtimeTicket("expired-ticket")
	if !errors.Is(err, errRealtimeTicketInvalid) {
		t.Fatalf("consumeRealtimeTicket() error = %v, want errRealtimeTicketInvalid", err)
	}

	if _, ok := service.tickets.items.Load("expired-ticket"); ok {
		t.Fatal("expired ticket was not removed after consume")
	}
}
