// Nucleo do agent-runner: roda o Claude headless (Agent SDK) restrito as tools
// do MCP oficial da Meta e mapeia as tool calls para actions[] auditaveis.

import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { config } from "./config.mjs";
import { currentAccessToken } from "./oauth.mjs";
import { createOmniMcpServer } from "./omni-tools.mjs";
import { SYSTEM_PROMPT } from "./system-prompt.mjs";

const MCP_SERVER_NAME = "meta-ads";
const OMNI_SERVER_NAME = "omni";
// Exportado para o fluxo de login (auth.mjs) reconhecer as tool calls do MCP.
export const MCP_TOOL_PREFIX = `mcp__${MCP_SERVER_NAME}__`;
const OMNI_TOOL_PREFIX = `mcp__${OMNI_SERVER_NAME}__`;
// Lista dos prefixos de tool que contam como acao real (Meta + bridge Omni).
export const MCP_TOOL_PREFIXES = [MCP_TOOL_PREFIX, OMNI_TOOL_PREFIX];
const READ_ONLY_META_TOOLS = new Set([
  "ads_get_ad_entities",
  "ads_get_creatives",
  "ads_get_ad_images",
  "ads_get_ad_videos",
]);
const AUTH_META_TOOLS = new Set([
  "ads_get_ad_accounts",
  "authenticate",
  "complete_authentication",
]);
const RUNNER_ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");

// startsWithAllowedPrefix: a tool pertence a um dos MCP permitidos (Meta/Omni)?
function startsWithAllowedPrefix(toolName) {
  return MCP_TOOL_PREFIXES.some((prefix) => toolName.startsWith(prefix));
}

// O endpoint legado /run recebe texto livre e nao carrega uma proposta aprovada,
// idempotency key nem registro de auditoria previo. Portanto ele e estritamente
// READ-ONLY. Escritas da Meta so poderao voltar por um endpoint dedicado que receba
// uma aprovacao persistida e um permit assinado pelo backend Go.
function normalizeAdAccountId(value) {
  return typeof value === "string"
    ? value.trim().replace(/^act_/i, "")
    : "";
}

function scopedMetaReadAllowed(toolName, input, policy = {}) {
  if (!toolName.startsWith(MCP_TOOL_PREFIX)) {
    return false;
  }
  const name = toolName.slice(MCP_TOOL_PREFIX.length);
  if (policy.mode === "auth") {
    return AUTH_META_TOOLS.has(name);
  }
  if (!READ_ONLY_META_TOOLS.has(name)) {
    return false;
  }
  const allowed = normalizeAdAccountId(policy.adAccountId);
  const requested = normalizeAdAccountId(input?.ad_account_id);
  return allowed !== "" && requested === allowed;
}

function isReadOnlyTool(toolName, input, policy) {
  if (toolName.startsWith(OMNI_TOOL_PREFIX)) {
    const name = toolName.slice(OMNI_TOOL_PREFIX.length);
    return (
      name === "instagram_get_accounts" || name === "instagram_get_recent_posts"
    );
  }
  if (!toolName.startsWith(MCP_TOOL_PREFIX)) {
    return false;
  }
  return scopedMetaReadAllowed(toolName, input, policy);
}

// Belt-and-suspenders: alem de tools:[] (zera as built-in), bloqueia por nome
// as ferramentas perigosas caso alguma volte por mudanca de default do SDK.
const DISALLOWED_BUILTIN_TOOLS = [
  "Bash",
  "BashOutput",
  "KillShell",
  "Read",
  "Write",
  "Edit",
  "MultiEdit",
  "NotebookEdit",
  "Glob",
  "Grep",
  "LS",
  "WebFetch",
  "WebSearch",
  "Task",
  "Agent",
  "TodoWrite",
  "Skill",
  "ExitPlanMode",
];

// Resumo humano (PT-BR) por tool conhecida do MCP da Meta; fallback generico.
const TOOL_SUMMARIES = {
  ads_create_campaign: "Criou campanha (nasce pausada)",
  ads_create_ad_set: "Criou conjunto de anuncios",
  ads_create_ad: "Criou anuncio",
  ads_update_entity: "Atualizou entidade de anuncios",
  ads_activate_entity: "Ativou entidade de anuncios",
  ads_pause_entity: "Pausou entidade de anuncios",
  instagram_get_recent_posts: "Buscou postagens recentes do Instagram",
  instagram_get_accounts: "Listou contas do Instagram",
};

export class AssistantTimeoutError extends Error {
  constructor(timeoutMs) {
    super(`Execucao excedeu ${timeoutMs}ms e foi abortada.`);
    this.name = "AssistantTimeoutError";
    this.code = "assistant_timeout";
  }
}

function summarizeToolCall(toolName, input) {
  const base =
    TOOL_SUMMARIES[toolName] || `Chamou a tool ${toolName} do MCP da Meta`;
  const hint = pickInputHint(input);
  return hint ? `${base} (${hint})` : base;
}

function pickInputHint(input) {
  if (!input || typeof input !== "object") {
    return "";
  }
  for (const key of [
    "name",
    "campaign_name",
    "entity_id",
    "campaign_id",
    "ad_account_id",
    "account_id",
  ]) {
    const value = input[key];
    if (typeof value === "string" && value.trim() !== "") {
      return `${key}=${value.trim().slice(0, 80)}`;
    }
  }
  return "";
}

// Multi-turn: o historico vem do Go como [{role, content}] e e prefixado como
// transcript no prompt do usuario (abordagem mais simples e deterministica;
// o runner e stateless, sem resume/sessao do SDK entre requests).
export function buildPrompt({ prompt, history, adAccountId }) {
  const sections = [];
  if (Array.isArray(history) && history.length > 0) {
    const transcript = history
      .map(
        (entry) =>
          `${entry.role === "assistant" ? "Assistente" : "Usuario"}: ${entry.content}`,
      )
      .join("\n\n");
    sections.push(
      `<historico_da_conversa>\n${transcript}\n</historico_da_conversa>`,
    );
  }
  if (typeof adAccountId === "string" && adAccountId.trim() !== "") {
    sections.push(
      `Contexto: a conta de anuncios ATIVA e o id NUMERICO ${adAccountId.trim()}. ` +
        `Passe esse ad_account_id (numerico) em TODA chamada de ferramenta; nunca use outra conta nem a default.`,
    );
  }
  sections.push(prompt);
  return sections.join("\n\n");
}

// Exportado: o fluxo de login (auth.mjs) reusa as MESMAS opcoes (MCP meta-ads,
// tools restritas, isolamento) numa sessao persistente em streaming.
export function buildQueryOptions(abortController, overrides = {}) {
  // model + systemPrompt vem das configuracoes da account (painel); vazio = default.
  const systemPrompt =
    typeof overrides.systemPrompt === "string" &&
    overrides.systemPrompt.trim() !== ""
      ? overrides.systemPrompt
      : SYSTEM_PROMPT;
  const overrideModel =
    typeof overrides.model === "string" && overrides.model.trim() !== ""
      ? overrides.model.trim()
      : config.model;
  const accountId = overrides.accountId;
  const getToolPolicyContext =
    typeof overrides.getToolPolicyContext === "function"
      ? overrides.getToolPolicyContext
      : () => ({});
  // Header OAuth do runner: se ha access_token valido em disco (refresh ja
  // rodou em session.ensure antes daqui), a conexao MCP da Meta nasce
  // autenticada pelo header e nao depende do login via modelo. Vazio = fallback.
  const metaServer = { type: "http", url: config.mcpUrl };
  const accessToken = currentAccessToken(accountId);
  if (accessToken !== "") {
    metaServer.headers = { Authorization: `Bearer ${accessToken}` };
  }
  const options = {
    abortController,
    cwd: RUNNER_ROOT,
    systemPrompt,
    // So os MCP declarados aqui existem; ignora .mcp.json/settings/plugins do host.
    // 'meta-ads' = MCP oficial da Meta (http); 'omni' = bridge in-process (feed IG).
    mcpServers: {
      [MCP_SERVER_NAME]: metaServer,
      [OMNI_SERVER_NAME]: createOmniMcpServer(accountId),
    },
    strictMcpConfig: true,
    // Isolamento do SDK: nao carrega settings/CLAUDE.md do filesystem. A persistencia
    // do token OAuth e garantida pela sessao tenant-scoped (session.mjs): uma
    // conexao MCP viva atende auth + /run somente daquela account.
    settingSources: [],
    // Zera o conjunto de tools built-in (Bash/Read/Web/etc nao existem na sessao).
    tools: [],
    disallowedTools: DISALLOWED_BUILTIN_TOOLS,
    // As tools aparecem para o modelo, mas o gate final abaixo libera somente
    // leituras/autenticacao. Wildcards aqui nao substituem canUseTool.
    allowedTools: [
      `mcp__${MCP_SERVER_NAME}`,
      `${MCP_TOOL_PREFIX}*`,
      `mcp__${OMNI_SERVER_NAME}`,
      `${OMNI_TOOL_PREFIX}*`,
    ],
    permissionMode: "default",
    // Gate final headless: qualquer pedido de permissao fora dos MCP meta-ads/omni
    // e negado sem prompt interativo.
    canUseTool: async (toolName, input) => {
      const policy = getToolPolicyContext() || {};
      if (
        startsWithAllowedPrefix(toolName) &&
        isReadOnlyTool(toolName, input, policy)
      ) {
        return { behavior: "allow" };
      }
      if (startsWithAllowedPrefix(toolName)) {
        return {
          behavior: "deny",
          message:
            "Escritas na Meta exigem uma proposta visual aprovada e idempotente no painel; este turno de chat e somente leitura.",
        };
      }
      return {
        behavior: "deny",
        message:
          "Apenas tools dos MCP meta-ads e omni sao permitidas neste runner.",
      };
    },
    maxTurns: config.maxTurns,
  };
  if (overrideModel !== "") {
    options.model = overrideModel;
  }
  return options;
}

export function collectAssistantMessage(message, state) {
  const blocks = Array.isArray(message.message?.content)
    ? message.message.content
    : [];
  const texts = [];
  for (const block of blocks) {
    if (block.type === "text" && typeof block.text === "string") {
      texts.push(block.text);
    }
    if (
      block.type === "tool_use" &&
      typeof block.name === "string" &&
      startsWithAllowedPrefix(block.name) &&
      isReadOnlyTool(block.name, block.input, state.toolPolicy || {})
    ) {
      // Registra apenas uma tool permitida pela policy deste turno. Ela ainda nao
      // conta como dado real: o resultado precisa chegar sem `is_error` primeiro.
      // Isso impede que uma tentativa de write negada ou uma leitura que falhou
      // desative a trava anti-invencao.
      const prefix =
        MCP_TOOL_PREFIXES.find((p) => block.name.startsWith(p)) || "";
      const toolName = block.name.slice(prefix.length);
      state.actions.push({
        tool: toolName,
        summary: summarizeToolCall(toolName, block.input),
        status: "pending",
      });
      if (typeof block.id === "string") {
        state.actionIndexByToolUseId.set(block.id, state.actions.length - 1);
      }
    }
  }
  if (texts.length > 0) {
    state.lastAssistantText = texts.join("\n").trim();
  }
}

export function collectToolResults(message, state) {
  const blocks = Array.isArray(message.message?.content)
    ? message.message.content
    : [];
  for (const block of blocks) {
    if (block.type !== "tool_result" || typeof block.tool_use_id !== "string") {
      continue;
    }
    const index = state.actionIndexByToolUseId.get(block.tool_use_id);
    if (index !== undefined) {
      state.actions[index].status = block.is_error === true ? "error" : "ok";
    }
  }
}

// sanitizeReply limpa o texto final mostrado no chat: remove blocos de ferramenta
// serializados (varias formas), JSON cru de resultado e tags soltas.
export function sanitizeReply(text) {
  if (typeof text !== "string") return "";
  let out = text;
  // Blocos <tag>...</tag> de ferramenta/raciocinio (function_calls/result/invoke/thinking/etc.).
  out = out.replace(
    /<(function_calls|function_results|function_result|result|invoke|thinking)\b[\s\S]*?<\/\1>/gi,
    "",
  );
  // Tags soltas remanescentes (inclui fechamentos orfaos de thinking/result).
  out = out.replace(
    /<\/?(function_calls|function_results|function_result|result|invoke|parameter|tool_use_error|thinking)\b[^>]*>/gi,
    "",
  );
  // Linhas que sao so um JSON/dict de resultado de tool. Aceita aspas simples
  // (dict do Python, ex.: {'campaigns': [...]}) ou duplas, objeto ou array.
  out = out
    .split("\n")
    .filter((line) => {
      const t = line.trim();
      const looksObj =
        (t.startsWith("{") && t.endsWith("}")) ||
        (t.startsWith("[") && t.endsWith("]"));
      return !(
        looksObj &&
        /['"](success|campaigns?|adsets?|ads|creatives?|data|id|name|objective|status|insights?|results?)['"]\s*:/.test(
          t,
        )
      );
    })
    .join("\n");
  return out.replace(/\n{3,}/g, "\n\n").trim();
}

// Resposta determinística quando o turno NAO produz dado real (sessao sem login
// na Meta ou ferramenta indisponivel). Impede o modelo de "inventar" campanhas.
export const NO_REAL_DATA_REPLY =
  "Nao consegui consultar a Meta agora — nenhuma ferramenta retornou dados reais. " +
  "Va na aba Conexoes, refaca o login para reconectar a Meta e tente de novo.";

// Padroes FORTES de afirmacao de dado real da Meta — coisas que o modelo so saberia
// chamando uma ferramenta. De proposito NAO inclui mencoes genericas ("posso listar
// suas campanhas"), que sao oferta/pergunta e devem passar. Roda so na trilha do chat
// (o login usa guard:false), entao ids/numeros longos aqui sao seguros.
const STRONG_DATA_PATTERNS = [
  /R\$\s*\d/, // valor monetario (orcamento/gasto)
  /\b\d{8,}\b/, // id numerico de objeto da Meta (campanha/conta/anuncio)
  /\b\d+([.,]\d+)?\s*%/, // percentual de metrica (CTR etc.)
  /\b(CTR|CPC|CPM|ROAS)\b\s*[:=]?\s*\d/i, // metrica nomeada com numero
  /\b\d[\d.,]*\s*(impress|clique|convers|visualiz|alcance)/i, // metrica contada
  /\b\d+\s+(campanhas?|conjuntos? de an[uú]ncio|ad\s?sets?|an[uú]ncios?|criativos?)\b/i, // "4 campanhas"
  /[-•*]\s+.+\b(ativ[ao]s?|pausad[ao]s?|ACTIVE|PAUSED)\b/i, // item de lista com status
];

// replyAssertsData: a resposta afirma dado/numero concreto da Meta (vs. oferta casual)?
export function replyAssertsData(text) {
  return (
    typeof text === "string" && STRONG_DATA_PATTERNS.some((re) => re.test(text))
  );
}

// guardReply: TRAVA determinística anti-invencao. Se NENHUMA ferramenta real do MCP
// rodou no turno (actions vazio) e a resposta afirma dados da Meta, o modelo nao
// teve como saber — troca pela mensagem de reconectar. Retorna { reply, suppressed }.
export function guardReply(reply, actions) {
  const usedRealTool =
    Array.isArray(actions) &&
    actions.some((action) => action && action.status === "ok");
  if (!usedRealTool && replyAssertsData(reply)) {
    return { reply: NO_REAL_DATA_REPLY, suppressed: true };
  }
  return { reply, suppressed: false };
}

// metaToolCountFromInit: quantas tools do MCP meta-ads o SDK expos ao iniciar a
// sessao. Diagnostico de conexao no log (0 = o MCP da Meta nao conectou).
export function metaToolCountFromInit(message) {
  const tools = Array.isArray(message?.tools) ? message.tools : [];
  return tools.filter(
    (name) => typeof name === "string" && name.startsWith(MCP_TOOL_PREFIX),
  ).length;
}

// O turno roda na sessao persistente da account (session.mjs), sem compartilhar
// conexao MCP, token ou bridge com outra account.
