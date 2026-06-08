package realtime

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const realtimeTicketTTL = 30 * time.Second

var errRealtimeTicketInvalid = errors.New("realtime: invalid ticket")

type realtimeTicketStore struct {
	items sync.Map
}

type realtimeTicket struct {
	Principal auth.Principal
	UserID    string
	AccountID string
	ExpiresAt time.Time
}

type realtimeTicketResponse struct {
	Ticket string `json:"ticket"`
}

func (service *Service) HandleTicket(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := strings.TrimSpace(principal.AccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(r.Header.Get("X-Account-Id"))
	}

	ticket, err := service.issueRealtimeTicket(principal, accountID)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao emitir ticket realtime.")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, realtimeTicketResponse{Ticket: ticket})
}

func (service *Service) issueRealtimeTicket(principal auth.Principal, accountID string) (string, error) {
	now := time.Now().UTC()
	service.cleanupExpiredRealtimeTickets(now)

	ticketID, err := newRealtimeTicketID()
	if err != nil {
		return "", err
	}

	accountID = strings.TrimSpace(accountID)
	principal.AccountID = accountID
	expiresAt := now.Add(realtimeTicketTTL)

	service.tickets.items.Store(ticketID, realtimeTicket{
		Principal: principal,
		UserID:    strings.TrimSpace(principal.UserID),
		AccountID: accountID,
		ExpiresAt: expiresAt,
	})

	return ticketID, nil
}

func (service *Service) consumeRealtimeTicket(rawTicket string) (auth.Principal, error) {
	ticketID := strings.TrimSpace(rawTicket)
	if ticketID == "" {
		return auth.Principal{}, errRealtimeTicketInvalid
	}

	value, ok := service.tickets.items.LoadAndDelete(ticketID)
	if !ok {
		return auth.Principal{}, errRealtimeTicketInvalid
	}

	ticket, ok := value.(realtimeTicket)
	if !ok {
		return auth.Principal{}, errRealtimeTicketInvalid
	}

	if time.Now().UTC().After(ticket.ExpiresAt) {
		return auth.Principal{}, errRealtimeTicketInvalid
	}

	principal := ticket.Principal
	principal.UserID = strings.TrimSpace(ticket.UserID)
	principal.AccountID = strings.TrimSpace(ticket.AccountID)
	return principal, nil
}

func (service *Service) cleanupExpiredRealtimeTickets(now time.Time) {
	service.tickets.items.Range(func(key, value any) bool {
		ticket, ok := value.(realtimeTicket)
		if !ok || now.After(ticket.ExpiresAt) {
			service.tickets.items.Delete(key)
		}

		return true
	})
}

func (service *Service) authenticateRealtimeRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	if ticket := strings.TrimSpace(r.URL.Query().Get("ticket")); ticket != "" {
		principal, err := service.consumeRealtimeTicket(ticket)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return auth.Principal{}, false
		}

		return principal, true
	}

	token, source := legacyRealtimeTokenFromRequest(r)
	if source == "access_token" || source == "token" {
		slog.Warn("realtime_ws_query_token_deprecated", "path", r.URL.Path, "param", source)
	}

	principal, err := service.authenticator.AuthenticateToken(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrUserInactive):
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		default:
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar a sessao.")
		}
		return auth.Principal{}, false
	}

	return principal, true
}

func legacyRealtimeTokenFromRequest(r *http.Request) (string, string) {
	if token := strings.TrimSpace(r.URL.Query().Get("access_token")); token != "" {
		return token, "access_token"
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token, "token"
	}

	authorizationHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorizationHeader == "" {
		return "", ""
	}

	bearerToken, err := auth.ExtractBearerToken(authorizationHeader)
	if err != nil {
		return "", ""
	}

	return bearerToken, "authorization"
}

func newRealtimeTicketID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
