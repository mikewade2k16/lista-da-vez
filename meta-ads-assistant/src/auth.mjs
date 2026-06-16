// Login do MCP oficial da Meta.
//
// Dois caminhos, nesta ordem:
//   1. OAuth proprio do runner (oauth.mjs) — deterministico, sem modelo. O token
//      e persistido em .auth/tokens.json e injetado como header na conexao MCP,
//      entao sobrevive a restart/recreate. Caminho preferencial.
//   2. Fallback in-session via modelo (legado): se discovery/DCR falharem, mantem
//      o fluxo antigo (tools authenticate/complete_authentication do MCP). O token
//      vive so na conexao viva e NAO persiste — o detail avisa isso.

import { authCompleteOauth, authStartOauth } from './oauth.mjs';
import { recreateSession, session } from './session.mjs';

const AUTH_START_PROMPT =
  'Chame IMEDIATAMENTE a ferramenta authenticate e devolva SOMENTE a URL de ' +
  'autorizacao retornada (comeca com https://), sem mais nenhuma palavra. Se voce ' +
  'JA estiver autenticado (as ferramentas ads_get_* ja existem), responda apenas ' +
  '"JA_AUTENTICADO".';

function buildCompletePrompt(callbackUrl) {
  const urlLine = callbackUrl ? `\nURL de callback: ${callbackUrl}` : '';
  return (
    'O usuario autorizou no navegador; a autenticacao pode ja ter concluido sozinha. ' +
    'Verifique seu acesso chamando ads_get_ad_accounts. Se conseguir, responda "OK: conectado". ' +
    'Se NAO tiver acesso e existir a ferramenta complete_authentication, chame-a com o callback_url.' +
    urlLine +
    '\nSe complete_authentication NAO existir (erro "No such tool"), o login JA foi concluido — ' +
    'responda "OK: conectado". Responda em UMA frase comecando com "OK:" ou "ERRO:".'
  );
}

function extractUrl(text) {
  const match = typeof text === 'string' ? text.match(/https?:\/\/[^\s)'"]+/) : null;
  return match ? match[0] : '';
}

// authStart inicia o login. Tenta primeiro o OAuth proprio (token persistente);
// se o servidor MCP nao suportar (discovery/DCR falham), cai no fluxo via modelo.
// Devolve { url, mode } — mode='oauth' | 'session'. url vazia + alreadyAuthed se
// ja estiver autenticado pelo modelo.
export async function authStart(opts = {}) {
  let oauthStart = null;
  try {
    oauthStart = await authStartOauth();
  } catch {
    oauthStart = null;
  }
  if (oauthStart?.url) {
    return { url: oauthStart.url, mode: 'oauth' };
  }
  // Fallback legado via modelo (token nao persiste).
  const { reply } = await session.run(AUTH_START_PROMPT, opts, { guard: false });
  const url = extractUrl(reply);
  if (url) {
    return { url, mode: 'session' };
  }
  if (/ja[_\s]?autenticad|ja (esta|estou) (conectad|autenticad)/i.test(reply)) {
    return { url: '', alreadyAuthed: true, mode: 'session' };
  }
  throw new Error('authenticate nao devolveu uma URL de autorizacao');
}

// authComplete conclui o login. Se ha fluxo OAuth pendente, troca code por tokens
// (deterministico) e recria a sessao MCP para nascer com o header. Senao, usa o
// fluxo via modelo (token nao persiste).
export async function authComplete(callbackUrl, opts = {}) {
  let oauthDone = null;
  try {
    oauthDone = await authCompleteOauth(callbackUrl);
  } catch {
    oauthDone = null;
  }
  if (oauthDone?.ok) {
    // Token novo em disco: descarta a sessao atual para a proxima nascer com o
    // header Authorization (autenticada sem o modelo).
    recreateSession();
    return { ok: true, detail: 'OK: conectado (token persistido em disco).' };
  }
  // Fallback legado via modelo (token vive so na conexao viva).
  const { reply } = await session.run(buildCompletePrompt(callbackUrl), opts, { guard: false });
  const ok =
    /(^|\s)ok:|conclu|conectad|autenticad|sucesso/i.test(reply) &&
    !/(^|\s)erro:|falh|expir|nao consegui|reconectar/i.test(reply);
  return { ok, detail: `${reply} (fallback in-session — token nao persiste)`.trim() };
}
