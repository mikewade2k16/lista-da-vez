// Configuracao do agent-runner do modulo meta_ads.
// Tudo vem de variaveis de ambiente; nenhum segredo e logado.

import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

import { hasValidAccessToken } from "./oauth-store.mjs";

function parsePositiveInt(rawValue, fallback) {
  const parsed = Number.parseInt(String(rawValue ?? ""), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

export const config = {
  // Porta do servico HTTP interno (host: npm start; container: profile meta-ads-assistant).
  port: parsePositiveInt(process.env.META_ADS_ASSISTANT_PORT, 8765),
  // Token de servico exigido no Authorization: Bearer do POST /run.
  // Vazio => runner desligado (503 runner_not_configured), nunca aberto sem auth.
  token: (process.env.META_ADS_ASSISTANT_TOKEN || "").trim(),
  // Tempo maximo de uma execucao do agente antes do abort (504).
  timeoutMs: parsePositiveInt(
    process.env.META_ADS_ASSISTANT_TIMEOUT_MS,
    120000,
  ),
  // Limite de turnos por execucao (protecao contra loop de tool calls).
  maxTurns: parsePositiveInt(process.env.META_ADS_ASSISTANT_MAX_TURNS, 25),
  // Maximo de conexoes Claude/MCP tenant-scoped mantidas pelo processo.
  maxAccountSessions: parsePositiveInt(
    process.env.META_ADS_ASSISTANT_MAX_SESSIONS,
    24,
  ),
  // Sessao sem turnos por esta janela e fechada na proxima varredura do pool.
  sessionIdleMs: parsePositiveInt(
    process.env.META_ADS_ASSISTANT_SESSION_IDLE_MS,
    15 * 60 * 1000,
  ),
  // URL do MCP oficial da Meta (mesma do .mcp.json da raiz do repo).
  mcpUrl: (
    process.env.META_ADS_MCP_URL || "https://mcp.facebook.com/ads"
  ).trim(),
  // Modelo opcional; vazio usa o default da assinatura/CLI.
  model: (process.env.META_ADS_ASSISTANT_MODEL || "").trim(),
  // Porta do callback HTTP local do OAuth proprio do runner (Problema 1).
  oauthCallbackPort: parsePositiveInt(
    process.env.META_ADS_OAUTH_CALLBACK_PORT,
    8766,
  ),
  // Base da API Go que expoe o bridge interno do Instagram (Problema 2).
  bridgeApiBase: (
    process.env.META_ADS_API_BASE || "http://localhost:9091"
  ).trim(),
  // Token de servico do bridge Go. Vazio => ferramentas omni respondem
  // "bridge nao configurada" (nunca chamam a API sem auth).
  bridgeToken: (process.env.META_ADS_RUNNER_BRIDGE_TOKEN || "").trim(),
};

// Checagem best-effort de credenciais do Claude para o GET /healthz.
// Nao faz chamada de rede (barato e sem consumir franquia): considera autenticado
// quando ha CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_API_KEY no ambiente ou o arquivo
// de credenciais do Claude Code (~/.claude/.credentials.json) existe no host.
// Limitacao conhecida: em macOS o login pode viver no Keychain (falso negativo).
export function claudeAuthStatus() {
  if ((process.env.CLAUDE_CODE_OAUTH_TOKEN || "").trim() !== "") {
    return {
      ok: true,
      detail: "CLAUDE_CODE_OAUTH_TOKEN presente no ambiente.",
    };
  }
  if ((process.env.ANTHROPIC_API_KEY || "").trim() !== "") {
    return { ok: true, detail: "ANTHROPIC_API_KEY presente no ambiente." };
  }
  const credentialsPath = join(homedir(), ".claude", ".credentials.json");
  if (existsSync(credentialsPath)) {
    return {
      ok: true,
      detail: "Credenciais do Claude Code encontradas em ~/.claude.",
    };
  }
  return {
    ok: false,
    detail:
      "Nenhuma credencial Claude detectada (CLAUDE_CODE_OAUTH_TOKEN, ANTHROPIC_API_KEY " +
      "ou ~/.claude/.credentials.json). Rode `claude setup-token` ou faca login no Claude Code.",
  };
}

// metaAuthStatus resolve o estado do login da Meta para o /healthz:
//   'oauth'   -> token valido da account em .auth/accounts/<uuid>/tokens.json;
//   'session' -> sem token em disco, mas a sessao MCP viva tem tools (fallback
//                in-session via modelo — nao persiste no restart);
//   'none'    -> deslogado.
// sessionHasMetaTools vem da sessao (session.mjs) por injecao, evitando ciclo.
export function metaAuthStatus(accountId, sessionHasMetaTools) {
  if (hasValidAccessToken(accountId)) {
    return "oauth";
  }
  if (sessionHasMetaTools === true) {
    return "session";
  }
  return "none";
}
