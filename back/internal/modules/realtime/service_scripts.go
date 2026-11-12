package realtime

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	toolsmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/tools"
)

// scriptsViewPermissions: quem pode ler roteiros pode assinar o canal (manage inclui view).
var scriptsViewPermissions = []string{"tools.scripts.view", "tools.scripts.manage"}

func scriptsAccountTopic(accountID string) string {
	return "scripts:account:" + accountID
}

// PublishScriptEvent implementa tools.ScriptPublisher: publica a invalidação no canal da
// conta dona e no das contas-cliente que viam ou passam a ver o roteiro. Sem conteúdo.
func (service *Service) PublishScriptEvent(_ context.Context, evt toolsmodule.ScriptEvent) {
	if service.hub == nil {
		return
	}
	eventType := strings.TrimSpace(evt.Type)
	if eventType == "" {
		return
	}
	base := Event{
		Type:       eventType,
		ResourceID: strings.TrimSpace(evt.ResourceID),
		UserID:     strings.TrimSpace(evt.UserID),
		SavedAt:    time.Now().UTC(),
	}
	if updated := strings.TrimSpace(evt.UpdatedAt); updated != "" {
		base.Payload = map[string]any{"updatedAt": updated}
	}
	for _, accountID := range scriptPublishAccountIDs(evt) {
		event := base
		event.AccountID = accountID
		service.hub.Publish(scriptsAccountTopic(accountID), event)
	}
}

// scriptPublishAccountIDs: conta dona + clientes, sem vazios nem repetidos. A ordem por
// usuário (reordered) fica só na conta dona.
func scriptPublishAccountIDs(evt toolsmodule.ScriptEvent) []string {
	candidates := []string{evt.AccountID}
	if evt.Type != toolsmodule.ScriptEventReordered {
		candidates = append(candidates, evt.ClientAccountIDs...)
	}
	ids := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, raw := range candidates {
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

// HandleScriptsSocket serve GET /v1/realtime/scripts?scope=account&accountId=... com a mesma
// autorização do canal do calendário, trocando a permissão por tools.scripts.view/manage.
func (service *Service) HandleScriptsSocket(w http.ResponseWriter, r *http.Request) {
	principal, ok := service.authenticateRealtimeRequest(w, r)
	if !ok {
		return
	}
	accountID, err := service.resolveRealtimeAccountID(principal, strings.TrimSpace(r.URL.Query().Get("accountId")))
	if err == nil && accountID == "" {
		err = errRealtimeValidation
	}
	if err == nil {
		err = service.authorizeAccountPermission(r.Context(), principal, accountID, scriptsViewPermissions)
	}
	if err != nil {
		service.writeRealtimeAccessError(w, r, err, "Canal de roteiros invalido.", "Canal de roteiros nao encontrado.")
		return
	}
	service.serveSubscriptionSocket(w, r, scriptsAccountTopic(accountID), tasksSubscriptionBuffer, Event{
		Type:      EventTypeConnected,
		AccountID: accountID,
		SavedAt:   time.Now().UTC(),
	}, nil, nil, service.readPumpWithRateLimit)
}

// authorizeAccountPermission: conta ativa + membership ativa + alguma das permissões
// efetivas. platform_admin tem bypass após a conta existir.
func (service *Service) authorizeAccountPermission(ctx context.Context, principal auth.Principal, accountID string, permissions []string) error {
	if service.pool == nil {
		return errRealtimeUnavailable
	}
	var exists bool
	if err := service.pool.QueryRow(ctx, `select exists(select 1 from core.accounts where id=$1::uuid and is_active)`, accountID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errRealtimeNotFound
	}
	if principal.Role == auth.RolePlatformAdmin {
		return nil
	}
	var member bool
	if err := service.pool.QueryRow(ctx, `select exists(select 1 from core.account_users
 where account_id=$1::uuid and user_id=$2::uuid and is_active)`, accountID, principal.UserID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return errRealtimeForbidden
	}
	allowed, err := service.hasAnyCoreTaskPermission(ctx, accountID, principal.UserID, permissions)
	if err != nil {
		return err
	}
	if allowed || (principal.PermissionsResolved && hasAnyString(principal.Permissions, permissions...)) {
		return nil
	}
	return errRealtimeForbidden
}
