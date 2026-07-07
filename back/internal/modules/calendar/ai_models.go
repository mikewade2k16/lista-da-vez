package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Listagem de modelos por provedor (Opcao C do painel). O front seleciona o provedor
// e o campo Modelo vira um SELECT populado pela lista real do provedor — nunca texto
// livre — evitando a armadilha provider=OpenAI + model=gemini-*. A chamada e SEMPRE
// server-side: a API key CRUA vive so aqui (resolveAIKey), o front nunca a ve. Por
// isso o Go bate direto no `{baseURL}/models` do provedor (camada OpenAI-compatible)
// com a key como Bearer e devolve so os IDs. Isto NAO passa pelo n8n (e config, nao
// dispatch) e NAO consome tokens de geracao.

// ErrModelsUnavailable: o provedor respondeu erro (chave invalida, endpoint sem
// /models, rede) — mapeado em 502 models_unavailable. Distinto de ErrAIKeyMissing
// (409, chave nao configurada) para o front dar a mensagem certa.
var ErrModelsUnavailable = errors.New("calendar: listagem de modelos indisponivel")

// modelsTimeout limita a ida ao provedor. Listagem e leve; endpoint travado nao pode
// segurar o handler.
const modelsTimeout = 10 * time.Second

// modelsUserAgent identifica o cliente HTTP ao provedor (evita bloqueio por WAF que
// barra o User-Agent default do Go — ver registro de falhas nº7).
const modelsUserAgent = "Omni-Calendar/1.0 (+https://omni.crowvisuals.com.br)"

// providerDefaultBaseURL espelha AI_PROVIDER_BASE_URL do front (utils/calendar-config.ts)
// para os provedores com slot de chave. SEGURANCA: a listagem usa SO este endpoint
// canonico server-side; uma Base URL vinda do cliente NAO e usada aqui (seria SSRF —
// servidor buscando host arbitrario). A Base URL customizada da config segue valendo
// no dispatch (n8n), so nao na listagem.
var providerDefaultBaseURL = map[string]string{
	"openai": "https://api.openai.com/v1",
	"gemini": "https://generativelanguage.googleapis.com/v1beta/openai",
	"glm":    "https://api.z.ai/api/paas/v4",
}

// modelsResponse e o shape OpenAI-compatible de GET /models (openai/gemini/glm todos
// respondem `{data:[{id}]}`). So o id importa para o select.
type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListAIModels devolve os IDs de modelos de CHAT do provedor, para o SELECT do painel.
// provider fora do enum de secret (gemini|glm|openai) => ErrInvalidProvider (400).
// Chave nao gravada (conta/global, conforme useGlobalKeys) => ErrAIKeyMissing (409).
// Falha do provedor (chave invalida/endpoint/rede) => ErrModelsUnavailable (502). NAO
// aplica o kill switch (ai.enabled): listar modelos e uma acao de CONFIGURACAO, tem de
// funcionar mesmo com a IA desligada. A key NUNCA e logada nem devolvida.
func (s *Service) ListAIModels(ctx context.Context, accountID, provider string) ([]string, error) {
	prov := normalizeSecretProvider(provider)
	if prov == "" {
		return nil, ErrInvalidProvider
	}
	baseURL := providerDefaultBaseURL[prov]
	if baseURL == "" {
		return nil, ErrInvalidProvider
	}
	apiKey, err := s.resolveAIKey(ctx, strings.TrimSpace(accountID), prov)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, ErrAIKeyMissing
	}
	ids, err := s.fetchProviderModels(ctx, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return filterChatModels(prov, ids), nil
}

// fetchProviderModels faz GET {baseURL}/models com a key como Bearer e devolve os IDs
// crus. Qualquer erro de transporte / status != 2xx / JSON invalido vira
// ErrModelsUnavailable (o handler mapeia 502). A key vai so no header, nunca em log.
func (s *Service) fetchProviderModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	callCtx, cancel := context.WithTimeout(ctx, modelsTimeout)
	defer cancel()
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/models"
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrModelsUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", modelsUserAgent)

	resp, err := s.chatClient().Do(req)
	if err != nil {
		return nil, ErrModelsUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrModelsUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, chatMaxResponseBytes))
	if err != nil {
		return nil, ErrModelsUnavailable
	}
	var out modelsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, ErrModelsUnavailable
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// filterChatModels normaliza (tira o prefixo `models/` do Gemini, dedup) e mantem so
// os modelos de CHAT do provedor (fora embeddings/audio/imagem/tts etc.), ordenados.
// Se o filtro nao casar NENHUM (heuristica desatualizada), devolve todos os IDs em vez
// de uma lista vazia — melhor sobrar do que travar o usuario sem opcao.
func filterChatModels(provider string, ids []string) []string {
	seen := make(map[string]bool, len(ids))
	all := make([]string, 0, len(ids))
	kept := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimPrefix(strings.TrimSpace(raw), "models/")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		all = append(all, id)
		if isChatModel(provider, strings.ToLower(id)) {
			kept = append(kept, id)
		}
	}
	if len(kept) == 0 {
		kept = all
	}
	sort.Strings(kept)
	return kept
}

// isChatModel aplica a heuristica por provedor (id ja em minusculas). Conservadora:
// inclui a familia de chat e exclui os nao-conversacionais conhecidos.
func isChatModel(provider, id string) bool {
	switch provider {
	case "openai":
		if !hasAnyPrefix(id, "gpt", "o1", "o3", "o4", "chatgpt") {
			return false
		}
		// So os modelos que rodam em /chat/completions (o endpoint do workflow n8n). A
		// /v1/models NAO informa qual endpoint cada modelo suporta, entao filtramos por
		// marcadores conhecidos: NAO-conversacionais (embedding/audio/image/tts/moderation/
		// search), COMPLETIONS-legada (instruct) e RESPONSES-ONLY (codex, *-pro,
		// *-deep-research, computer-use). Um modelo incompativel listado falharia com "not
		// a chat model" / "not supported in v1/chat/completions" (ex.: gpt-3.5-turbo-instruct,
		// gpt-5.3-codex, o1-pro). Heuristica validada contra a /v1/models real (120 modelos);
		// familia incompativel nova precisa entrar aqui.
		return !containsAny(id, "embedding", "whisper", "tts", "audio", "realtime",
			"transcribe", "image", "dall-e", "moderation", "search", "instruct",
			"codex", "-pro", "deep-research", "computer-use")
	case "gemini":
		if !strings.Contains(id, "gemini") {
			return false
		}
		return !containsAny(id, "embedding", "aqa", "imagen")
	case "glm":
		if !strings.HasPrefix(id, "glm") {
			return false
		}
		return !containsAny(id, "embedding", "voice", "video", "image", "cogview", "rerank")
	default:
		return true
	}
}

// hasAnyPrefix diz se s comeca por algum dos prefixos.
func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// containsAny diz se s contem alguma das substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
