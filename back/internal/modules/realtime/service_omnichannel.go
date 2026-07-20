package realtime

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	omnichannelmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

// omnichannelViewPermission e a permissao efetiva exigida para assinar o canal do atendimento
// (espelho de calendarViewPermission). platform_admin tem bypass apos a conta existir.
const omnichannelViewPermission = "omnichannel.conversations.view"

// PublishOmnichannelEvent implementa omnichannel.Publisher (spec F5): repassa o evento ja montado
// pelo call-site (shape completo, camelCase) para o Event do transporte e publica no canal da
// conta. O modulo omnichannel seta o Type (message.*/conversation.*); aqui so entregamos. Sem
// hub / Type vazio / AccountID vazio => no-op (espelho de PublishCalendarEvent).
//
// Cinto e suspensorio (spec F5 §Sanitizacao): mesmo o call-site ja sanitizando, um payload com
// mediaUrl data: e zerado aqui — NUNCA base64 no WS. O front busca a midia por GET .../media.
func (service *Service) PublishOmnichannelEvent(_ context.Context, evt omnichannelmodule.RealtimeEvent) {
	if service.hub == nil {
		return
	}

	eventType := strings.TrimSpace(evt.Type)
	accountID := strings.TrimSpace(evt.AccountID)
	if eventType == "" || accountID == "" {
		return
	}

	payload := evt.Payload
	if payload != nil {
		if raw, ok := payload["mediaUrl"].(string); ok && strings.HasPrefix(strings.TrimSpace(raw), "data:") {
			payload["mediaUrl"] = nil
		}
	}

	service.hub.Publish(omnichannelAccountTopic(accountID), Event{
		Type:       eventType,
		AccountID:  accountID,
		ResourceID: strings.TrimSpace(evt.ResourceID),
		Payload:    payload,
		SavedAt:    time.Now().UTC(),
	})
}

// HandleOmnichannelSocket serve o canal do atendimento (GET /v1/realtime/omnichannel
// ?scope=account&accountId=...). Autoriza a conta antes do upgrade (conta ativa + membership +
// omnichannel.conversations.view; platform_admin bypass) e reusa serveSubscriptionSocket.
func (service *Service) HandleOmnichannelSocket(w http.ResponseWriter, r *http.Request) {
	principal, ok := service.authenticateRealtimeRequest(w, r)
	if !ok {
		return
	}

	accountID, err := service.resolveOmnichannelAccount(r.Context(), principal, r)
	if err != nil {
		service.writeRealtimeAccessError(w, r, err, "Canal de atendimento invalido.", "Canal de atendimento nao encontrado.")
		return
	}

	service.serveSubscriptionSocket(w, r, omnichannelAccountTopic(accountID), tasksSubscriptionBuffer, Event{
		Type:      EventTypeConnected,
		AccountID: accountID,
		SavedAt:   time.Now().UTC(),
	}, nil, nil, service.readPumpWithRateLimit)
}

// resolveOmnichannelAccount resolve o accountId do canal a partir da query (scope=account) e o
// autoriza. platform_admin pode informar accountId; usuario comum so assina a propria conta
// (resolveRealtimeAccountID valida). Conta vazia => 400.
func (service *Service) resolveOmnichannelAccount(ctx context.Context, principal auth.Principal, r *http.Request) (string, error) {
	requested := strings.TrimSpace(r.URL.Query().Get("accountId"))
	accountID, err := service.resolveRealtimeAccountID(principal, requested)
	if err != nil {
		// resolveRealtimeAccountID so falha com errRealtimeForbidden: usuario comum forjando
		// OUTRA conta (ou sem escopo). Fora de escopo => 404 (enumeration, canonico §10 e
		// Verificavel #5 da spec F5), NUNCA 403. O 403 fica reservado a PERMISSAO faltando,
		// ja tratado em authorizeOmnichannelAccount. (Divergencia deliberada do calendar, que
		// propaga o 403.) A conta propria do usuario resolve normalmente (requested vazio ou
		// == a conta do Principal).
		return "", errRealtimeNotFound
	}
	if accountID == "" {
		return "", errRealtimeValidation
	}
	if err := service.authorizeOmnichannelAccount(ctx, principal, accountID); err != nil {
		return "", err
	}
	return accountID, nil
}

// authorizeOmnichannelAccount valida o acesso da conta ao canal: conta ativa + membership +
// permissao efetiva. platform_admin bypass apos a conta existir.
//
// DIVERGENCIA DELIBERADA de authorizeCalendarAccount (spec F5 §Autorizacao, canonico §10): NAO
// membro => errRealtimeNotFound (404, escopo — enumeration), NUNCA 403. Copiar o calendar cego
// reintroduziria o 403. Permissao FALTANDO (membro sem a key) => errRealtimeForbidden (403 —
// permissao gateia feature).
func (service *Service) authorizeOmnichannelAccount(ctx context.Context, principal auth.Principal, accountID string) error {
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
		return errRealtimeNotFound
	}

	hasPermission, err := service.hasAnyCoreTaskPermission(ctx, accountID, principal.UserID, []string{omnichannelViewPermission})
	if err != nil {
		return err
	}
	if hasPermission {
		return nil
	}

	if principal.PermissionsResolved && hasAnyString(principal.Permissions, omnichannelViewPermission) {
		return nil
	}

	return errRealtimeForbidden
}
