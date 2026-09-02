package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	omnichannelmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

const (
	// omnichannelViewPermission e a permissao efetiva exigida para assinar o canal do atendimento
	// (espelho de calendarViewPermission). A fonte autoritativa e o RBAC atual no PostgreSQL.
	omnichannelViewPermission = "omnichannel.conversations.view"

	// O topico omnichannel e account-wide. Portanto, ele transporta apenas uma invalidacao opaca;
	// dados da mensagem/conversa sao obtidos novamente pela REST, que aplica o escopo do usuario.
	omnichannelInvalidateEventType = "omnichannel.invalidate"

	omnichannelInvalidateReasonMessageChanged     = "message_changed"
	omnichannelInvalidateReasonHistoryReset       = "history_reset"
	omnichannelInvalidateReasonAccessScopeChanged = "access_scope_changed"
)

// PublishOmnichannelEvent implementa omnichannel.Publisher no boundary do transporte account-wide.
// Produtores legados ainda podem emitir message.*/conversation.*, mas nenhum campo deles atravessa
// o boundary: o evento publicado possui type fixo, ResourceID vazio e payload estritamente limitado
// a eventId/reason/occurredAt. Tipo ou motivo desconhecido e descartado (fail-closed).
func (service *Service) PublishOmnichannelEvent(_ context.Context, evt omnichannelmodule.RealtimeEvent) {
	if service.hub == nil {
		return
	}

	accountID := strings.TrimSpace(evt.AccountID)
	reason, ok := omnichannelInvalidationReason(strings.TrimSpace(evt.Type), evt.Payload)
	if accountID == "" || !ok {
		return
	}

	eventID, ok := omnichannelOpaqueEventID()
	if !ok {
		return
	}
	now := time.Now().UTC()
	occurredAt := omnichannelOccurredAt(evt.Payload, now)

	service.hub.Publish(omnichannelAccountTopic(accountID), Event{
		Type:       omnichannelInvalidateEventType,
		AccountID:  accountID,
		ResourceID: "",
		Payload: map[string]any{
			"eventId":    eventID,
			"reason":     reason,
			"occurredAt": occurredAt.Format(time.RFC3339Nano),
		},
		SavedAt: now,
	})
}

// omnichannelInvalidationReason converte os tres tipos ricos legados em invalidacao e aceita o
// contrato novo somente com um motivo fechado. Qualquer outro valor nao publica nada.
func omnichannelInvalidationReason(eventType string, payload map[string]any) (string, bool) {
	switch eventType {
	case omnichannelmodule.RealtimeEventMessageCreated,
		omnichannelmodule.RealtimeEventMessageUpdated,
		omnichannelmodule.RealtimeEventConversationUpdated:
		return omnichannelInvalidateReasonMessageChanged, true
	case omnichannelInvalidateEventType:
		reason, ok := payload["reason"].(string)
		reason = strings.TrimSpace(reason)
		if !ok || !isAllowedOmnichannelInvalidationReason(reason) {
			return "", false
		}
		return reason, true
	default:
		return "", false
	}
}

func isAllowedOmnichannelInvalidationReason(reason string) bool {
	switch reason {
	case omnichannelInvalidateReasonMessageChanged,
		omnichannelInvalidateReasonHistoryReset,
		omnichannelInvalidateReasonAccessScopeChanged:
		return true
	default:
		return false
	}
}

// omnichannelOpaqueEventID cria uma chave aleatoria por publicacao. Ela e compartilhada por todos
// os assinantes que recebem aquele mesmo Event, mas nao deriva de telefone, conteudo ou IDs do
// dominio — nem mesmo por hash, evitando correlacao/dicionario sobre dados sensiveis conhecidos.
func omnichannelOpaqueEventID() (string, bool) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", false
	}
	return "omi_" + hex.EncodeToString(random[:]), true
}

// omnichannelOccurredAt reaproveita somente timestamps validos e os normaliza; qualquer outro
// valor e ignorado para impedir que texto arbitrario atravesse o payload opaco.
func omnichannelOccurredAt(payload map[string]any, fallback time.Time) time.Time {
	for _, key := range []string{"occurredAt", "updatedAt", "createdAt"} {
		raw, exists := payload[key]
		if !exists {
			continue
		}
		switch value := raw.(type) {
		case time.Time:
			if !value.IsZero() {
				return value.UTC()
			}
		case string:
			parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
			if err == nil {
				return parsed.UTC()
			}
		}
	}

	return fallback.UTC()
}

// HandleOmnichannelSocket serve o canal do atendimento (GET /v1/realtime/omnichannel
// ?scope=account&accountId=...). Autoriza a conta antes do upgrade (conta ativa + membership +
// modulo + omnichannel.conversations.view) e revalida antes de cada escrita no socket.
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
	}, nil, nil, service.readPumpWithRateLimit, func(ctx context.Context) error {
		return service.authorizeOmnichannelAccount(ctx, principal, accountID)
	})
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
// permissao efetiva. Nenhum papel cria bypass de membership, modulo ou permissao.
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

	var exists, member, moduleEnabled bool
	if err := service.pool.QueryRow(ctx, `
		select a.is_active,
			exists(select 1 from core.account_users au
				where au.account_id=a.id and au.user_id=$2::uuid and au.is_active),
			exists(select 1 from core.account_modules am
				where am.account_id=a.id and am.module_id='omnichannel' and am.enabled)
		from core.accounts a where a.id=$1::uuid
	`, accountID, principal.UserID).Scan(&exists, &member, &moduleEnabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errRealtimeNotFound
		}
		return err
	}
	if !exists {
		return errRealtimeNotFound
	}
	if !member {
		return errRealtimeNotFound
	}
	if !moduleEnabled {
		return errRealtimeForbidden
	}

	hasPermission, err := service.hasAnyCoreTaskPermission(ctx, accountID, principal.UserID, []string{omnichannelViewPermission})
	if err != nil {
		return err
	}
	if hasPermission {
		return nil
	}
	return errRealtimeForbidden
}
