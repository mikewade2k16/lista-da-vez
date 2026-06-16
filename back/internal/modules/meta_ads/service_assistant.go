package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
)

// Limites do chat do assistente.
const (
	assistantHistoryMaxLimit = 50   // default e teto do GET /assistant/messages
	assistantContextTurns    = 12   // ultimas mensagens enviadas como historico ao runner
	assistantMaxMessageChars = 4000 // tamanho maximo da mensagem do usuario
)

// Erros de validacao da mensagem (mapeados para 400 nos handlers).
var (
	ErrAssistantMessageEmpty   = errors.New("meta_ads: mensagem vazia")
	ErrAssistantMessageTooLong = errors.New("meta_ads: mensagem longa demais")
)

// AssistantHistory retorna as ultimas mensagens do chat em ordem cronologica.
// limit <= 0 ou acima do teto vira assistantHistoryMaxLimit.
func (s *Service) AssistantHistory(ctx context.Context, accountID string, limit int) ([]AssistantMessageView, error) {
	if limit <= 0 || limit > assistantHistoryMaxLimit {
		limit = assistantHistoryMaxLimit
	}
	rows, err := s.store.ListAssistantMessages(ctx, accountID, limit)
	if err != nil {
		return nil, err
	}
	views := make([]AssistantMessageView, len(rows))
	for i, m := range rows {
		views[i] = toAssistantMessageView(m)
	}
	return views, nil
}

// AssistantSend persiste a mensagem do usuario, roda o runner (Claude headless
// + MCP meta-ads) com as ultimas mensagens como contexto e persiste a resposta
// com as acoes executadas. Se o runner executou acoes e ha adAccountID, dispara
// um sync best-effort para o cache refletir o resultado real na hora — falha do
// sync NAO falha a requisicao (so warn no log). A mensagem do usuario fica
// persistida mesmo quando o runner falha (o retry nao perde o que foi digitado).
func (s *Service) AssistantSend(ctx context.Context, accountID, message, adAccountID string) (AssistantSendResult, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return AssistantSendResult{}, ErrAssistantMessageEmpty
	}
	if len(message) > assistantMaxMessageChars {
		return AssistantSendResult{}, ErrAssistantMessageTooLong
	}
	adAccountID = strings.TrimSpace(adAccountID)

	userMsg, err := s.store.InsertAssistantMessage(ctx, accountID, assistantRoleUser, message, nil)
	if err != nil {
		return AssistantSendResult{}, err
	}

	history, err := s.assistantContext(ctx, accountID, userMsg.ID)
	if err != nil {
		return AssistantSendResult{}, err
	}

	// O runner/MCP precisam do ID da conta NA META (act_<numero>), nao do nosso
	// ID interno. Traduz aqui; o adAccountID interno continua valendo para o sync.
	runnerAdAccount := metaAdAccountForRunner(ctx, s, accountID, adAccountID)
	result, err := s.runner.Run(ctx, message, history, runnerAdAccount, accountID, s.assistantRunnerOpts(ctx, accountID))
	if err != nil {
		return AssistantSendResult{}, err
	}

	var actionsJSON []byte
	if len(result.Actions) > 0 {
		if actionsJSON, err = json.Marshal(result.Actions); err != nil {
			return AssistantSendResult{}, err
		}
	}
	assistantMsg, err := s.store.InsertAssistantMessage(ctx, accountID, assistantRoleAssistant, result.Reply, actionsJSON)
	if err != nil {
		return AssistantSendResult{}, err
	}

	syncTriggered := false
	if len(result.Actions) > 0 && adAccountID != "" {
		syncTriggered = true
		if _, syncErr := s.Sync(ctx, accountID, adAccountID); syncErr != nil {
			slog.WarnContext(ctx, "meta_ads: sync pos-acao do assistente falhou",
				"account_id", accountID, "ad_account_id", adAccountID, "err", syncErr)
		}
	}

	return AssistantSendResult{
		Messages: []AssistantMessageView{
			toAssistantMessageView(userMsg),
			toAssistantMessageView(assistantMsg),
		},
		SyncTriggered: syncTriggered,
	}, nil
}

// AssistantClear apaga o historico do chat da account.
func (s *Service) AssistantClear(ctx context.Context, accountID string) error {
	return s.store.DeleteAssistantMessages(ctx, accountID)
}

// AssistantHealth consulta o estado do runner para o card de conexoes. Nunca
// propaga erro: runner nao configurado/fora do ar viram OK=false com Detail
// explicativo (o handler devolve 200 sempre).
func (s *Service) AssistantHealth(ctx context.Context) (AssistantHealthView, error) {
	view, err := s.runner.Health(ctx)
	switch {
	case errors.Is(err, ErrRunnerNotConfigured):
		return AssistantHealthView{OK: false, Detail: "runner_not_configured"}, nil
	case err != nil:
		return AssistantHealthView{OK: false, Detail: "runner_unreachable"}, nil
	default:
		return view, nil
	}
}

// AssistantAuthStart inicia o login do MCP oficial da Meta no runner e devolve a
// URL de autorizacao do Facebook para o painel exibir.
func (s *Service) AssistantAuthStart(ctx context.Context, accountID string) (string, error) {
	out, err := s.runner.AuthStart(ctx, s.assistantRunnerOpts(ctx, accountID))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

// AssistantAuthComplete conclui o login do MCP da Meta com a URL de callback que
// o usuario copiou do navegador (localhost/callback?code=...).
func (s *Service) AssistantAuthComplete(ctx context.Context, accountID, callbackURL string) (bool, string, error) {
	// callbackURL pode ser vazio: com a sessao persistente, o login pode ja ter
	// sido concluido sozinho (redirect localhost capturado com a conexao viva).
	out, err := s.runner.AuthComplete(ctx, strings.TrimSpace(callbackURL), s.assistantRunnerOpts(ctx, accountID))
	if err != nil {
		return false, "", err
	}
	return out.OK, out.Detail, nil
}

// AssistantSettings retorna o modelo + system prompt da account (default quando
// ainda nao customizou).
func (s *Service) AssistantSettings(ctx context.Context, accountID string) (AssistantSettingsView, error) {
	model, systemPrompt, found, err := s.store.GetAssistantSettings(ctx, accountID)
	if err != nil {
		return AssistantSettingsView{}, err
	}
	if !found || strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultAssistantSystemPrompt
	}
	return AssistantSettingsView{Model: model, SystemPrompt: systemPrompt}, nil
}

// SaveAssistantSettings grava o modelo + system prompt editados no painel.
func (s *Service) SaveAssistantSettings(ctx context.Context, accountID, model, systemPrompt string) error {
	return s.store.UpsertAssistantSettings(ctx, accountID, strings.TrimSpace(model), systemPrompt)
}

// assistantRunnerOpts carrega as configuracoes da account para o runner (vazias
// quando nao customizou -> runner usa os defaults dele).
func (s *Service) assistantRunnerOpts(ctx context.Context, accountID string) RunnerOpts {
	model, systemPrompt, found, err := s.store.GetAssistantSettings(ctx, accountID)
	if err != nil || !found {
		return RunnerOpts{}
	}
	return RunnerOpts{Model: model, SystemPrompt: systemPrompt}
}

// metaAdAccountForRunner traduz o ID interno da conta de anuncio para o ID dela
// na Meta (act_<numero>), que e o que o runner/MCP usam. Vazio se nao resolver
// (o assistente cai na conta default da Meta).
func metaAdAccountForRunner(ctx context.Context, s *Service, accountID, adAccountID string) string {
	if adAccountID == "" {
		return ""
	}
	ad, err := s.requireAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return ""
	}
	// A MCP da Meta usa o id NUMERICO da conta (ex.: 1547966673703703), nao o
	// formato act_<id>. Remove o prefixo act_ se houver.
	return strings.TrimPrefix(strings.TrimSpace(ad.MetaAdAccountID), "act_")
}

// assistantContext monta o historico enviado ao runner: as ultimas mensagens da
// account em ordem cronologica, excluindo a mensagem recem-inserida (ela vai
// como prompt, nao como turno repetido).
func (s *Service) assistantContext(ctx context.Context, accountID, excludeID string) ([]RunnerTurn, error) {
	rows, err := s.store.ListAssistantMessages(ctx, accountID, assistantContextTurns)
	if err != nil {
		return nil, err
	}
	turns := make([]RunnerTurn, 0, len(rows))
	for _, m := range rows {
		if m.ID == excludeID {
			continue
		}
		turns = append(turns, RunnerTurn{Role: m.Role, Content: m.Content})
	}
	return turns, nil
}
