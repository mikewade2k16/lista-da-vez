// Servico HTTP interno do agent-runner do modulo meta_ads.
// Rotas: GET /healthz (status) e POST /run (executa o assistente).
// Nunca expor publicamente: e rede interna, com Bearer token de servico.

import { createHash, timingSafeEqual } from 'node:crypto';
import http from 'node:http';

import { AssistantTimeoutError } from './agent.mjs';
import { authComplete, authStart } from './auth.mjs';
import { claudeAuthStatus, config, metaAuthStatus } from './config.mjs';
import { runAssistant, session } from './session.mjs';

const MAX_BODY_BYTES = 1 << 20; // 1 MiB

// Espelha o bearerEquals de back/internal/modules/automation/http.go:
// prefixo "Bearer " + comparacao em tempo constante. Hash de ambos os lados
// para igualar tamanho antes do timingSafeEqual.
function bearerEquals(header, token) {
  const prefix = 'Bearer ';
  if (typeof header !== 'string' || !header.startsWith(prefix)) {
    return false;
  }
  const got = header.slice(prefix.length).trim();
  const gotDigest = createHash('sha256').update(got).digest();
  const wantDigest = createHash('sha256').update(token).digest();
  return timingSafeEqual(gotDigest, wantDigest);
}

function writeJSON(res, statusCode, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(statusCode, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(body),
  });
  res.end(body);
}

function writeError(res, statusCode, errorCode, message) {
  writeJSON(res, statusCode, { error: errorCode, message });
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let total = 0;
    req.on('data', (chunk) => {
      total += chunk.length;
      if (total > MAX_BODY_BYTES) {
        reject(new Error('body_too_large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

function parseRunPayload(rawBody) {
  let parsed;
  try {
    parsed = JSON.parse(rawBody);
  } catch {
    return { error: 'JSON invalido.' };
  }
  if (!parsed || typeof parsed !== 'object') {
    return { error: 'Body deve ser um objeto JSON.' };
  }
  const prompt = typeof parsed.prompt === 'string' ? parsed.prompt.trim() : '';
  if (prompt === '') {
    return { error: 'Campo "prompt" e obrigatorio.' };
  }
  const history = [];
  if (parsed.history !== undefined) {
    if (!Array.isArray(parsed.history)) {
      return { error: 'Campo "history" deve ser uma lista.' };
    }
    for (const entry of parsed.history) {
      const role = entry?.role;
      const content = entry?.content;
      if ((role !== 'user' && role !== 'assistant') || typeof content !== 'string') {
        return { error: 'Itens de "history" devem ter role user|assistant e content string.' };
      }
      history.push({ role, content });
    }
  }
  const adAccountId = typeof parsed.adAccountId === 'string' ? parsed.adAccountId.trim() : '';
  // accountId = id da conta do painel (core.accounts), usado pelas tools do bridge
  // omni (feed do Instagram). NAO e a conta de anuncios da Meta (adAccountId).
  const accountId = typeof parsed.accountId === 'string' ? parsed.accountId.trim() : '';
  const model = typeof parsed.model === 'string' ? parsed.model : '';
  const systemPrompt = typeof parsed.systemPrompt === 'string' ? parsed.systemPrompt : '';
  return { payload: { prompt, history, adAccountId, accountId, model, systemPrompt } };
}

// optsFromParsed extrai model + systemPrompt do body (configuracoes da account).
function optsFromParsed(parsed) {
  return {
    model: typeof parsed?.model === 'string' ? parsed.model : '',
    systemPrompt: typeof parsed?.systemPrompt === 'string' ? parsed.systemPrompt : '',
  };
}

function handleHealthz(res) {
  const auth = claudeAuthStatus();
  // metaAuth: 'oauth' (token proprio em disco) | 'session' (fallback in-memory) | 'none'.
  const metaAuth = metaAuthStatus(session.hasMetaTools());
  writeJSON(res, 200, { ok: true, claudeAuth: auth.ok, detail: auth.detail, metaAuth });
}

async function handleRun(req, res) {
  if (config.token === '') {
    writeError(res, 503, 'runner_not_configured', 'META_ADS_ASSISTANT_TOKEN nao configurado.');
    return;
  }
  if (!bearerEquals(req.headers.authorization, config.token)) {
    writeError(res, 401, 'unauthorized', 'Token de servico invalido.');
    return;
  }

  let rawBody;
  try {
    rawBody = await readBody(req);
  } catch (err) {
    if (err?.message === 'body_too_large') {
      writeError(res, 413, 'body_too_large', 'Body acima de 1MB.');
      return;
    }
    writeError(res, 400, 'invalid_body', 'Falha ao ler o body.');
    return;
  }

  const { payload, error } = parseRunPayload(rawBody);
  if (error) {
    writeError(res, 400, 'invalid_body', error);
    return;
  }

  try {
    const result = await runAssistant(payload);
    writeJSON(res, 200, result);
  } catch (err) {
    if (err instanceof AssistantTimeoutError) {
      writeError(res, 504, 'assistant_timeout', err.message);
      return;
    }
    // Mensagem generica para fora; detalhe (sem tokens) no log do processo.
    console.error('[meta-ads-assistant] run falhou:', err?.name || 'Error', err?.message || '');
    writeError(res, 502, 'assistant_error', 'O assistente falhou ao executar o comando.');
  }
}

// requireRunnerAuth valida token configurado + Bearer. Retorna true se respondeu
// um erro (o caller deve abortar).
function requireRunnerAuth(req, res) {
  if (config.token === '') {
    writeError(res, 503, 'runner_not_configured', 'META_ADS_ASSISTANT_TOKEN nao configurado.');
    return true;
  }
  if (!bearerEquals(req.headers.authorization, config.token)) {
    writeError(res, 401, 'unauthorized', 'Token de servico invalido.');
    return true;
  }
  return false;
}

// handleAuthStart inicia o login do MCP da Meta (chama authenticate -> URL).
async function handleAuthStart(req, res) {
  if (requireRunnerAuth(req, res)) {
    return;
  }
  let opts = {};
  try {
    const raw = await readBody(req);
    opts = optsFromParsed(JSON.parse(raw || '{}'));
  } catch {
    opts = {};
  }
  try {
    const { url, mode, alreadyAuthed } = await authStart(opts);
    writeJSON(res, 200, { url, mode, alreadyAuthed: alreadyAuthed === true });
  } catch (err) {
    console.error('[meta-ads-assistant] auth/start falhou:', err?.name || 'Error', err?.message || '');
    writeError(res, 502, 'auth_error', 'Falha ao iniciar a autenticacao com a Meta.');
  }
}

// handleAuthComplete conclui o login com a URL de callback colada no painel.
async function handleAuthComplete(req, res) {
  if (requireRunnerAuth(req, res)) {
    return;
  }
  let rawBody;
  try {
    rawBody = await readBody(req);
  } catch {
    writeError(res, 400, 'invalid_body', 'Falha ao ler o body.');
    return;
  }
  let callbackUrl = '';
  let opts = {};
  try {
    const parsed = JSON.parse(rawBody || '{}');
    callbackUrl = typeof parsed?.callbackUrl === 'string' ? parsed.callbackUrl.trim() : '';
    opts = optsFromParsed(parsed);
  } catch {
    writeError(res, 400, 'invalid_body', 'JSON invalido.');
    return;
  }
  // callbackUrl pode ser vazio: com a sessao persistente, o redirect localhost
  // pode ter sido capturado sozinho e o login ja estar concluido.
  try {
    const { ok, detail } = await authComplete(callbackUrl, opts);
    writeJSON(res, 200, { ok, detail });
  } catch (err) {
    console.error('[meta-ads-assistant] auth/complete falhou:', err?.name || 'Error', err?.message || '');
    if (/nenhuma sessao|expir/i.test(String(err?.message || ''))) {
      writeError(res, 409, 'auth_session_gone', 'A sessao de login expirou. Gere o link novamente.');
      return;
    }
    writeError(res, 502, 'auth_error', 'Falha ao concluir a autenticacao com a Meta.');
  }
}

const ROUTES_WITH_METHODS = ['/healthz', '/run', '/auth/start', '/auth/complete'];

const server = http.createServer((req, res) => {
  const path = (req.url || '/').split('?')[0];
  if (path === '/healthz' && req.method === 'GET') {
    handleHealthz(res);
    return;
  }
  if (path === '/run' && req.method === 'POST') {
    handleRun(req, res).catch(() => {
      writeError(res, 500, 'internal_error', 'Erro interno do runner.');
    });
    return;
  }
  if (path === '/auth/start' && req.method === 'POST') {
    handleAuthStart(req, res).catch(() => {
      writeError(res, 500, 'internal_error', 'Erro interno do runner.');
    });
    return;
  }
  if (path === '/auth/complete' && req.method === 'POST') {
    handleAuthComplete(req, res).catch(() => {
      writeError(res, 500, 'internal_error', 'Erro interno do runner.');
    });
    return;
  }
  if (ROUTES_WITH_METHODS.includes(path)) {
    writeError(res, 405, 'method_not_allowed', 'Metodo nao suportado nesta rota.');
    return;
  }
  writeError(res, 404, 'not_found', 'Rota inexistente.');
});

server.listen(config.port, '0.0.0.0', () => {
  const auth = claudeAuthStatus();
  console.warn(
    `[meta-ads-assistant] ouvindo em 0.0.0.0:${config.port} | ` +
      `token configurado: ${config.token !== '' ? 'sim' : 'NAO (503 em /run)'} | ` +
      `claudeAuth: ${auth.ok ? 'ok' : 'ausente'}`,
  );
});
