// Servidor MCP in-process 'omni' (Problema 2).
//
// O MCP oficial da Meta NAO expoe o feed do Instagram. O backend Go expoe um
// BRIDGE interno (/internal/meta-ads/runner/instagram/*) e aqui o runner publica
// esse bridge como ferramentas custom (createSdkMcpServer + tool do Agent SDK),
// para o modelo buscar contas e postagens reais do Instagram.
//
// O accountId NAO e parametro da ferramenta (o modelo nao escolhe a conta): vem
// do contexto do turno, injetado por setAccountContext (chamado pelo execTurn).
// Erros da bridge viram texto amigavel no tool result — nunca exception solta.

import { createSdkMcpServer, tool } from '@anthropic-ai/claude-agent-sdk';
import { z } from 'zod';

import { config } from './config.mjs';

const BRIDGE_TIMEOUT_MS = 15000;

// Registry simples do accountId do turno atual. Setter chamado pelo execTurn
// (turnos sao serializados, entao e seguro guardar um unico valor global).
let currentAccountId = '';

export function setAccountContext(accountId) {
  currentAccountId = typeof accountId === 'string' ? accountId.trim() : '';
}

export function getAccountContext() {
  return currentAccountId;
}

// textResult monta o retorno padrao de tool (lista content com um bloco texto).
function textResult(text) {
  return { content: [{ type: 'text', text }] };
}

// bridgeBase normaliza a base da API Go (sem barra final).
function bridgeBase() {
  return (config.bridgeApiBase || '').replace(/\/+$/, '');
}

// callBridge faz GET autenticado no bridge Go e devolve { ok, status, data } ou
// { ok:false, networkError } em falha de rede/timeout. Nunca lanca.
async function callBridge(pathWithQuery) {
  if (!config.bridgeToken) {
    return { ok: false, notConfigured: true };
  }
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), BRIDGE_TIMEOUT_MS);
  try {
    const res = await fetch(`${bridgeBase()}${pathWithQuery}`, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${config.bridgeToken}`,
        Accept: 'application/json',
      },
      signal: controller.signal,
    });
    let data = null;
    try {
      data = await res.json();
    } catch {
      data = null;
    }
    return { ok: res.ok, status: res.status, data };
  } catch {
    return { ok: false, networkError: true };
  } finally {
    clearTimeout(timer);
  }
}

// friendlyError traduz a resposta de erro da bridge em texto pro modelo. Cobre os
// codigos do contrato congelado (401/404 not_connected/503/400) e a falha de rede.
function friendlyError(result) {
  if (result.notConfigured) {
    return 'Bridge nao configurada no runner (META_ADS_RUNNER_BRIDGE_TOKEN ausente). Avise o administrador.';
  }
  if (result.networkError) {
    return 'Nao consegui falar com o painel Omni (bridge fora do ar). Tente de novo em instantes.';
  }
  const code = result.data?.error || '';
  switch (result.status) {
    case 401:
      return 'Bridge recusou a autenticacao (token interno invalido). Avise o administrador.';
    case 503:
      return 'A integracao com a Meta nao esta configurada no painel (bridge desligada no servidor).';
    case 400:
      return 'Faltou identificar a conta do contexto para consultar o Instagram.';
    case 404:
      if (code === 'not_connected') {
        return 'Esta conta nao esta conectada a Meta no painel. Conecte na aba Conexoes e tente de novo.';
      }
      return 'Recurso do Instagram nao encontrado para esta conta.';
    default:
      return result.data?.message || 'A consulta ao Instagram falhou no painel.';
  }
}

// requireAccount garante accountId do contexto antes de chamar a bridge.
function requireAccount() {
  if (!currentAccountId) {
    return 'Nao ha conta no contexto desta conversa para consultar o Instagram.';
  }
  return '';
}

// formatAccounts resume as contas de Instagram Business retornadas.
function formatAccounts(accounts) {
  if (!Array.isArray(accounts) || accounts.length === 0) {
    return 'Nenhuma conta de Instagram Business conectada a esta conta no painel.';
  }
  const lines = accounts.map((acc) => {
    const username = acc.username ? `@${acc.username}` : '(sem username)';
    const page = acc.pageName ? ` — pagina ${acc.pageName}` : '';
    return `- ${username} (igUserId: ${acc.igUserId || '?'}${page})`;
  });
  return ['Contas de Instagram Business conectadas:', ...lines].join('\n');
}

// formatMedia resume as postagens; inclui mediaUrl/thumbnailUrl em linha propria
// (o painel renderiza a imagem). Sem inventar nada: so o que a bridge retornou.
function formatMedia(media) {
  if (!Array.isArray(media) || media.length === 0) {
    return 'Nenhuma postagem recente encontrada para esta conta de Instagram.';
  }
  const blocks = media.map((post, idx) => {
    const caption = (post.caption || '').trim();
    const shortCaption = caption.length > 160 ? `${caption.slice(0, 157)}...` : caption;
    const image = post.mediaUrl || post.thumbnailUrl || '';
    const parts = [`Post ${idx + 1} — ${post.mediaType || 'POST'} (${formatDate(post.timestamp)})`];
    if (image) {
      parts.push(image);
    }
    if (shortCaption) {
      parts.push(`Legenda: ${shortCaption}`);
    }
    if (post.permalink) {
      parts.push(`Link: ${post.permalink}`);
    }
    if (post.id) {
      parts.push(`mediaId: ${post.id}`);
    }
    return parts.join('\n');
  });
  return ['Postagens recentes do Instagram:', '', blocks.join('\n\n')].join('\n');
}

function formatDate(iso) {
  if (typeof iso !== 'string' || iso === '') {
    return 'sem data';
  }
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString('pt-BR');
}

// Ferramenta: lista as contas de Instagram Business conectadas no painel.
const instagramGetAccounts = tool(
  'instagram_get_accounts',
  'Lista as contas de Instagram Business conectadas a esta conta do painel (igUserId, username, pageId, pageName). Use antes de buscar postagens quando houver mais de uma conta.',
  {},
  async () => {
    const missing = requireAccount();
    if (missing) {
      return textResult(missing);
    }
    const result = await callBridge(
      `/internal/meta-ads/runner/instagram/accounts?accountId=${encodeURIComponent(currentAccountId)}`,
    );
    if (!result.ok) {
      return textResult(friendlyError(result));
    }
    return textResult(formatAccounts(result.data?.accounts));
  },
);

// Ferramenta: ultimas postagens do Instagram (id, legenda, tipo, imagem, link, data).
const instagramGetRecentPosts = tool(
  'instagram_get_recent_posts',
  'Lista as ultimas postagens do Instagram desta conta (id da midia, legenda, tipo, URL da imagem/thumbnail, permalink e data). As URLs mediaUrl/thumbnailUrl podem ser exibidas no chat. Use para o usuario escolher um post antes de criar anuncio.',
  {
    limit: z
      .number()
      .int()
      .min(1)
      .max(20)
      .optional()
      .describe('Quantas postagens trazer (1 a 20, padrao 5).'),
    igUserId: z
      .string()
      .optional()
      .describe('igUserId de uma conta especifica (opcional; padrao usa a primeira conectada).'),
  },
  async (args) => {
    const missing = requireAccount();
    if (missing) {
      return textResult(missing);
    }
    const limit = Number.isFinite(args?.limit) ? Math.min(Math.max(args.limit, 1), 20) : 5;
    const params = new URLSearchParams({ accountId: currentAccountId, limit: String(limit) });
    if (typeof args?.igUserId === 'string' && args.igUserId.trim() !== '') {
      params.set('igUserId', args.igUserId.trim());
    }
    const result = await callBridge(`/internal/meta-ads/runner/instagram/media?${params.toString()}`);
    if (!result.ok) {
      return textResult(friendlyError(result));
    }
    return textResult(formatMedia(result.data?.media));
  },
);

// createOmniMcpServer instancia o servidor MCP in-process 'omni' com as 2 tools.
export function createOmniMcpServer() {
  return createSdkMcpServer({
    name: 'omni',
    version: '0.1.0',
    tools: [instagramGetAccounts, instagramGetRecentPosts],
  });
}
