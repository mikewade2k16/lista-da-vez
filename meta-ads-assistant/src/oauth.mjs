// OAuth proprio do runner para o MCP da Meta (Problema 1).
//
// Em vez de depender do login feito PELO MODELO (token vive so na conexao MCP
// viva e some no restart/recreate), o runner faz o fluxo OAuth padrao de MCP
// direto: discovery (RFC 9728 / RFC 8414) -> Dynamic Client Registration
// (RFC 7591) -> Authorization Code + PKCE -> tokens persistidos em disco
// (.auth/tokens.json). O access_token entra como header Authorization na conexao
// MCP http (buildQueryOptions), entao qualquer sessao nova nasce autenticada.
//
// Tudo aqui e SEM modelo (deterministico). Se discovery ou DCR falharem, o caller
// cai no fluxo legado via modelo (auth.mjs). Nenhum token e logado.

import { createHash, randomBytes } from 'node:crypto';
import http from 'node:http';

import { config } from './config.mjs';
import { clearTokens, loadClient, loadTokens, saveClient, saveTokens } from './oauth-store.mjs';

// A Meta so libera Dynamic Client Registration para client_names permitidos
// (testado 2026-06-12: 'omni-meta-ads-runner' -> 400 invalid_client_metadata;
// 'Claude Code' -> 200 com client_id). O runner roda o Claude Code por baixo
// (Agent SDK), entao o nome e fiel ao client real.
const CLIENT_NAME = 'Claude Code';
const DISCOVERY_TIMEOUT_MS = 10000;
const CALLBACK_TIMEOUT_MS = 10 * 60 * 1000; // 10min para o usuario autorizar

// base64url sem padding (PKCE / state).
function base64url(buffer) {
  return buffer.toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function randomToken(bytes = 32) {
  return base64url(randomBytes(bytes));
}

// pkcePair gera code_verifier aleatorio e o challenge S256 correspondente.
function pkcePair() {
  const verifier = randomToken(32);
  const challenge = base64url(createHash('sha256').update(verifier).digest());
  return { verifier, challenge };
}

// redirectUri monta a URL do callback local a partir da porta configurada.
export function redirectUri() {
  return `http://127.0.0.1:${config.oauthCallbackPort}/oauth/callback`;
}

// fetchJson faz GET/POST com timeout e devolve { ok, status, data }. Nunca lanca
// por status != 2xx (o caller decide o fallback); so lanca em erro de rede/abort.
async function fetchJson(url, options = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), DISCOVERY_TIMEOUT_MS);
  try {
    const res = await fetch(url, { ...options, signal: controller.signal });
    let data = null;
    try {
      data = await res.json();
    } catch {
      data = null;
    }
    return { ok: res.ok, status: res.status, data };
  } finally {
    clearTimeout(timer);
  }
}

// discover descobre os endpoints OAuth do MCP da Meta. Tenta a metadata do
// protected resource (RFC 9728) para achar o authorization server e depois a
// metadata do AS (RFC 8414); cai direto na metadata do AS se a primeira faltar.
// Retorna { authorization_endpoint, token_endpoint, registration_endpoint } ou
// null (sem suporte -> fallback via modelo).
export async function discover() {
  const mcpUrl = new URL(config.mcpUrl);
  const origin = mcpUrl.origin;
  const resourcePath = mcpUrl.pathname.replace(/^\/+|\/+$/g, '');

  let authServer = '';
  // RFC 9728: /.well-known/oauth-protected-resource/<path>
  const prmUrl = resourcePath
    ? `${origin}/.well-known/oauth-protected-resource/${resourcePath}`
    : `${origin}/.well-known/oauth-protected-resource`;
  const prm = await safeFetchJson(prmUrl);
  if (prm?.data && Array.isArray(prm.data.authorization_servers) && prm.data.authorization_servers[0]) {
    authServer = String(prm.data.authorization_servers[0]).replace(/\/+$/, '');
  }

  // Metadata do authorization server (RFC 8414). Issuer COM path (caso da Meta:
  // https://mcp.facebook.com/ads) usa PATH-INSERTION — o well-known entra ANTES
  // do path: https://mcp.facebook.com/.well-known/oauth-authorization-server/ads.
  // Tenta tambem a forma sufixo e a raiz do origin como fallback.
  const candidates = [];
  if (authServer) {
    const asUrl = new URL(authServer);
    const asPath = asUrl.pathname.replace(/^\/+|\/+$/g, '');
    if (asPath) {
      candidates.push(`${asUrl.origin}/.well-known/oauth-authorization-server/${asPath}`);
    }
    candidates.push(`${authServer}/.well-known/oauth-authorization-server`);
  }
  candidates.push(`${origin}/.well-known/oauth-authorization-server`);

  let meta = null;
  for (const candidate of candidates) {
    const res = await safeFetchJson(candidate);
    if (
      res?.ok &&
      res.data &&
      typeof res.data.authorization_endpoint === 'string' &&
      typeof res.data.token_endpoint === 'string'
    ) {
      meta = res.data;
      break;
    }
  }
  if (!meta) {
    return null;
  }
  return {
    authorization_endpoint: meta.authorization_endpoint,
    token_endpoint: meta.token_endpoint,
    registration_endpoint:
      typeof meta.registration_endpoint === 'string' ? meta.registration_endpoint : '',
    // Permissoes anunciadas pelo AS (ads_management, ads_read, ...). O dialog da
    // Meta EXIGE scope explicito ("app precisa de pelo menos uma supported
    // permission" sem ele).
    scopes: Array.isArray(meta.scopes_supported)
      ? meta.scopes_supported.filter((s) => typeof s === 'string' && s !== '')
      : [],
  };
}

// safeFetchJson nunca lanca: erro de rede/timeout vira null (fallback decide).
async function safeFetchJson(url, options) {
  try {
    return await fetchJson(url, options);
  } catch {
    return null;
  }
}

// registerClient faz Dynamic Client Registration (RFC 7591). Reusa o client salvo
// se o registration_endpoint nao mudou. Retorna { clientId } ou null.
export async function registerClient(metadata) {
  // RFC 7591 usa scope separado por espaco. Cliente registrado SEM scope gera
  // dialog "app nao esta disponivel" na Meta — por isso o scope entra no registro
  // e participa do criterio de reuso (mudou scope => re-registra).
  const scope = Array.isArray(metadata.scopes) ? metadata.scopes.join(' ') : '';
  const existing = loadClient();
  if (
    existing &&
    existing.clientId &&
    existing.registrationEndpoint === metadata.registration_endpoint &&
    existing.scope === scope
  ) {
    return { clientId: existing.clientId };
  }
  if (!metadata.registration_endpoint) {
    return null;
  }
  const body = {
    client_name: CLIENT_NAME,
    redirect_uris: [redirectUri()],
    grant_types: ['authorization_code', 'refresh_token'],
    response_types: ['code'],
    token_endpoint_auth_method: 'none',
  };
  if (scope !== '') {
    body.scope = scope;
  }
  const res = await safeFetchJson(metadata.registration_endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res || !res.ok || !res.data || typeof res.data.client_id !== 'string') {
    return null;
  }
  const client = {
    clientId: res.data.client_id,
    registrationEndpoint: metadata.registration_endpoint,
    scope,
  };
  saveClient(client);
  return { clientId: client.clientId };
}

// startCallbackListener sobe um HTTP temporario no callback port que captura o
// primeiro code+state. Resolve { code, state } quando o redirect chega; timeout
// (10min) rejeita. Devolve { promise, close } para o caller controlar o ciclo.
function startCallbackListener() {
  let settle;
  const promise = new Promise((resolve, reject) => {
    settle = { resolve, reject };
  });
  const server = http.createServer((req, res) => {
    const reqUrl = new URL(req.url || '/', `http://127.0.0.1:${config.oauthCallbackPort}`);
    if (reqUrl.pathname !== '/oauth/callback') {
      res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
      res.end('not found');
      return;
    }
    const code = reqUrl.searchParams.get('code') || '';
    const state = reqUrl.searchParams.get('state') || '';
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(
      '<!doctype html><meta charset="utf-8"><title>Omni</title>' +
        '<body style="font-family:sans-serif;padding:2rem">' +
        '<h2>Conexao com a Meta concluida</h2>' +
        '<p>Pode fechar esta aba e voltar ao painel.</p></body>',
    );
    settle.resolve({ code, state });
  });
  const timer = setTimeout(() => {
    settle.reject(new Error('callback_timeout'));
  }, CALLBACK_TIMEOUT_MS);
  const close = () => {
    clearTimeout(timer);
    try {
      server.close();
    } catch {
      /* noop */
    }
  };
  server.listen(config.oauthCallbackPort, '127.0.0.1');
  server.on('error', (err) => settle.reject(err));
  return { promise, close };
}

// pendingAuth guarda o estado do fluxo em andamento (1 por vez; turnos serializados).
let pendingAuth = null;

// authStartOauth (deterministico, sem modelo): discovery -> DCR -> monta a URL de
// autorizacao com PKCE + state e sobe o listener do callback. Devolve { url } ou
// null se o servidor nao suportar OAuth (fallback via modelo).
export async function authStartOauth() {
  const metadata = await discover();
  if (!metadata) {
    return null;
  }
  const registered = await registerClient(metadata);
  if (!registered) {
    return null;
  }
  const { verifier, challenge } = pkcePair();
  const state = randomToken(16);

  // Encerra um fluxo anterior pendente (listener antigo) antes de abrir outro.
  cancelPending();
  let listener;
  try {
    listener = startCallbackListener();
  } catch {
    listener = null;
  }
  pendingAuth = {
    metadata,
    clientId: registered.clientId,
    verifier,
    state,
    listener,
  };
  // Ignora rejeicao nao consumida do listener (timeout) para nao virar unhandled.
  listener?.promise?.catch(() => {});

  const authUrl = new URL(metadata.authorization_endpoint);
  authUrl.searchParams.set('response_type', 'code');
  authUrl.searchParams.set('client_id', registered.clientId);
  authUrl.searchParams.set('redirect_uri', redirectUri());
  authUrl.searchParams.set('code_challenge', challenge);
  authUrl.searchParams.set('code_challenge_method', 'S256');
  authUrl.searchParams.set('state', state);
  // resource (RFC 8707): liga o token ao MCP da Meta.
  authUrl.searchParams.set('resource', config.mcpUrl);
  // scope: o dialog da Meta recusa autorizacao sem permissao pedida. O endpoint
  // e o dialog/oauth classico do Facebook, que usa lista separada por VIRGULA.
  if (Array.isArray(metadata.scopes) && metadata.scopes.length > 0) {
    authUrl.searchParams.set('scope', metadata.scopes.join(','));
  }
  return { url: authUrl.toString() };
}

// buildAuthUrl: helper puro (sem rede) usado em teste — monta a URL de
// autorizacao a partir de metadata + parametros ja conhecidos.
export function buildAuthUrl({ authorization_endpoint, clientId, challenge, state, resource }) {
  const authUrl = new URL(authorization_endpoint);
  authUrl.searchParams.set('response_type', 'code');
  authUrl.searchParams.set('client_id', clientId);
  authUrl.searchParams.set('redirect_uri', redirectUri());
  authUrl.searchParams.set('code_challenge', challenge);
  authUrl.searchParams.set('code_challenge_method', 'S256');
  authUrl.searchParams.set('state', state);
  if (resource) {
    authUrl.searchParams.set('resource', resource);
  }
  return authUrl.toString();
}

function cancelPending() {
  if (pendingAuth?.listener?.close) {
    pendingAuth.listener.close();
  }
  pendingAuth = null;
}

// parseCallback extrai code+state de uma callbackUrl colada no painel.
function parseCallback(callbackUrl) {
  try {
    const url = new URL(callbackUrl);
    return {
      code: url.searchParams.get('code') || '',
      state: url.searchParams.get('state') || '',
    };
  } catch {
    return { code: '', state: '' };
  }
}

// exchangeCode troca authorization_code por tokens no token_endpoint (PKCE).
async function exchangeCode({ metadata, clientId, code, verifier }) {
  const params = new URLSearchParams({
    grant_type: 'authorization_code',
    code,
    redirect_uri: redirectUri(),
    client_id: clientId,
    code_verifier: verifier,
    resource: config.mcpUrl,
  });
  const res = await safeFetchJson(metadata.token_endpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
      Accept: 'application/json',
    },
    body: params.toString(),
  });
  if (!res || !res.ok || !res.data || typeof res.data.access_token !== 'string') {
    return false;
  }
  saveTokens({
    access_token: res.data.access_token,
    refresh_token: res.data.refresh_token,
    expiresInSec: res.data.expires_in,
  });
  return true;
}

// authCompleteOauth conclui o fluxo: usa o code colado (callbackUrl) OU o que o
// listener capturou sozinho; valida o state; troca por tokens. Retorna
// { ok, detail }. Sem fluxo pendente => ok:false (caller cai no fallback).
export async function authCompleteOauth(callbackUrl) {
  if (!pendingAuth) {
    return { ok: false, detail: 'Nenhum fluxo OAuth pendente.' };
  }
  const flow = pendingAuth;
  let code = '';
  let state = '';
  const pasted = parseCallback(callbackUrl || '');
  if (pasted.code) {
    code = pasted.code;
    state = pasted.state;
  } else if (flow.listener?.promise) {
    try {
      const captured = await flow.listener.promise;
      code = captured.code;
      state = captured.state;
    } catch {
      cancelPending();
      return { ok: false, detail: 'O tempo de autorizacao expirou. Gere o link de novo.' };
    }
  }
  if (!code) {
    return { ok: false, detail: 'Codigo de autorizacao ausente. Cole a URL de callback completa.' };
  }
  if (state && flow.state && state !== flow.state) {
    cancelPending();
    return { ok: false, detail: 'Falha de validacao (state divergente). Gere o link de novo.' };
  }
  const ok = await exchangeCode({
    metadata: flow.metadata,
    clientId: flow.clientId,
    code,
    verifier: flow.verifier,
  });
  cancelPending();
  if (!ok) {
    return { ok: false, detail: 'A Meta recusou a troca do codigo por token.' };
  }
  return { ok: true, detail: 'OK: conectado (token persistido).' };
}

// refreshIfNeeded: chamado ANTES de criar a sessao MCP. Se o access_token expira
// em < 60s e ha refresh_token, troca por um novo no token_endpoint. Se o refresh
// falhar, apaga os tokens (volta ao estado deslogado). Best-effort: erro de rede
// nao apaga nada (mantem o token atual). Retorna o access_token valido ou ''.
export async function refreshIfNeeded() {
  const tokens = loadTokens();
  if (!tokens) {
    return '';
  }
  const skewMs = 60000;
  const expired = tokens.expires_at !== 0 && tokens.expires_at < Date.now() + skewMs;
  if (!expired) {
    return tokens.access_token;
  }
  if (!tokens.refresh_token) {
    clearTokens();
    return '';
  }
  const metadata = await discover();
  const client = loadClient();
  if (!metadata || !client?.clientId) {
    // Sem como descobrir o token_endpoint agora: mantem o token (pode ainda valer).
    return tokens.access_token;
  }
  const params = new URLSearchParams({
    grant_type: 'refresh_token',
    refresh_token: tokens.refresh_token,
    client_id: client.clientId,
    resource: config.mcpUrl,
  });
  let res;
  try {
    res = await fetchJson(metadata.token_endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        Accept: 'application/json',
      },
      body: params.toString(),
    });
  } catch {
    // Erro de rede: nao apaga o token; tenta de novo no proximo turno.
    return tokens.access_token;
  }
  if (!res.ok || !res.data || typeof res.data.access_token !== 'string') {
    clearTokens();
    return '';
  }
  saveTokens({
    access_token: res.data.access_token,
    // Alguns AS nao reemitem refresh_token; mantem o anterior nesse caso.
    refresh_token: res.data.refresh_token || tokens.refresh_token,
    expiresInSec: res.data.expires_in,
  });
  return res.data.access_token;
}

// currentAccessToken: usado pelo buildQueryOptions para montar o header. Lê do
// disco (apos refreshIfNeeded). Vazio = sem OAuth (cai no fluxo legado).
export function currentAccessToken() {
  const tokens = loadTokens();
  return tokens?.access_token || '';
}
