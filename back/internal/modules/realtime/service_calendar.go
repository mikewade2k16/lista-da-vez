package realtime

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	calendarmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
)

// calendarViewPermission e a permissao efetiva exigida para assinar o canal do
// calendario (espelho de PermTasksView no canal de tasks). platform_admin tem bypass.
const calendarViewPermission = "calendar.view"

// PublishCalendarEvent implementa calendar.Publisher (contrato C11): mapeia o evento
// lean do modulo calendar para o Event do transporte e publica no canal da conta. O
// modulo calendar seta o Type (calendar.*); aqui so repassamos e jogamos date/monthKey/
// status no Payload map (nao inchar o struct Event). Sem hub/type/conta => no-op.
func (service *Service) PublishCalendarEvent(_ context.Context, evt calendarmodule.RealtimeEvent) {
	if service.hub == nil {
		return
	}

	eventType := strings.TrimSpace(evt.Type)
	accountIDs := calendarPublishAccountIDs(evt)
	if eventType == "" || len(accountIDs) == 0 {
		return
	}

	payload := map[string]any{}
	if date := strings.TrimSpace(evt.Date); date != "" {
		payload["date"] = date
	}
	if monthKey := strings.TrimSpace(evt.MonthKey); monthKey != "" {
		payload["monthKey"] = monthKey
	}
	if status := strings.TrimSpace(evt.Status); status != "" {
		payload["status"] = status
	}

	baseEvent := Event{
		Type:       eventType,
		ResourceID: strings.TrimSpace(evt.ResourceID),
		Version:    evt.Version,
		SavedAt:    time.Now().UTC(),
	}
	if len(payload) > 0 {
		baseEvent.Payload = payload
	}

	for _, accountID := range accountIDs {
		realtimeEvent := baseEvent
		realtimeEvent.AccountID = accountID
		service.hub.Publish(calendarAccountTopic(accountID), realtimeEvent)
	}
}

func calendarPublishAccountIDs(evt calendarmodule.RealtimeEvent) []string {
	ids := make([]string, 0, 1+len(evt.ClientIDs))
	seen := make(map[string]struct{}, 1+len(evt.ClientIDs))
	for _, raw := range append([]string{evt.AccountID}, evt.ClientIDs...) {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// HandleCalendarSocket serve o canal de eventos do calendario (GET /v1/realtime/calendar
// ?scope=account&accountId=...). Autoriza a conta antes do upgrade (conta ativa +
// membership + calendar.view; platform_admin bypass) e reusa serveSubscriptionSocket.
func (service *Service) HandleCalendarSocket(w http.ResponseWriter, r *http.Request) {
	principal, ok := service.authenticateRealtimeRequest(w, r)
	if !ok {
		return
	}

	accountID, err := service.resolveCalendarAccount(r.Context(), principal, r)
	if err != nil {
		service.writeRealtimeAccessError(w, r, err, "Canal de calendario invalido.", "Canal de calendario nao encontrado.")
		return
	}

	service.serveSubscriptionSocket(w, r, calendarAccountTopic(accountID), tasksSubscriptionBuffer, Event{
		Type:      EventTypeConnected,
		AccountID: accountID,
		SavedAt:   time.Now().UTC(),
	}, nil, nil, service.readPumpWithRateLimit)
}

// resolveCalendarAccount resolve o accountId do canal do calendario a partir da query
// (scope=account) e o autoriza. platform_admin pode informar accountId; usuario comum
// so assina a propria conta (resolveRealtimeAccountID valida). Conta vazia => 400.
func (service *Service) resolveCalendarAccount(ctx context.Context, principal auth.Principal, r *http.Request) (string, error) {
	requested := strings.TrimSpace(r.URL.Query().Get("accountId"))
	accountID, err := service.resolveRealtimeAccountID(principal, requested)
	if err != nil {
		return "", err
	}
	if accountID == "" {
		return "", errRealtimeValidation
	}
	if err := service.authorizeCalendarAccount(ctx, principal, accountID); err != nil {
		return "", err
	}
	return accountID, nil
}

// authorizeCalendarAccount valida o acesso da conta ao canal do calendario (copia
// adaptada de authorizeTasksAccount trocando as permission keys por calendar.view):
// conta ativa + membership + permissao efetiva. platform_admin tem bypass apos a conta
// existir. Reusa hasAnyCoreTaskPermission (query generica por chave de permissao).
func (service *Service) authorizeCalendarAccount(ctx context.Context, principal auth.Principal, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errRealtimeValidation
	}
	if service.pool == nil {
		return errRealtimeUnavailable
	}

	var exists bool
	if err := service.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.accounts where id = $1::uuid and is_active = true
		)
	`, accountID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errRealtimeNotFound
	}

	if principal.Role == auth.RolePlatformAdmin {
		return nil
	}

	var member bool
	if err := service.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		)
	`, accountID, principal.UserID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return errRealtimeForbidden
	}

	hasPermission, err := service.hasAnyCoreTaskPermission(ctx, accountID, principal.UserID, []string{calendarViewPermission})
	if err != nil {
		return err
	}
	if hasPermission {
		return nil
	}

	if principal.PermissionsResolved && hasAnyString(principal.Permissions, calendarViewPermission) {
		return nil
	}

	return errRealtimeForbidden
}
