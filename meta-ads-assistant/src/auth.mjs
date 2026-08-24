// Login do MCP oficial da Meta, sempre escopado pela account do painel.
// OAuth proprio persiste em disco; o fallback via modelo permanece apenas como
// compatibilidade e vive na sessao Claude/MCP daquela mesma account.

import { requireAccountId } from "./account-id.mjs";
import { authCompleteOauth, authStartOauth } from "./oauth.mjs";
import { OAuthCallbackConflictError } from "./oauth-callback.mjs";
import { hasValidAccessToken } from "./oauth-store.mjs";
import { recreateSession, sessionPool } from "./session.mjs";

const AUTH_START_PROMPT =
  "Chame IMEDIATAMENTE a ferramenta authenticate e devolva SOMENTE a URL de " +
  "autorizacao retornada (comeca com https://), sem mais nenhuma palavra. Se voce " +
  "JA estiver autenticado (as ferramentas ads_get_* ja existem), responda apenas " +
  '"JA_AUTENTICADO".';

const AUTH_MODE_TTL_MS = 10 * 60 * 1000;
const authModes = new Map();

function setAuthMode(accountId, mode) {
  const previous = authModes.get(accountId);
  clearTimeout(previous?.timer);
  const entry = { mode, timer: null };
  entry.timer = setTimeout(() => {
    if (authModes.get(accountId) === entry) {
      authModes.delete(accountId);
    }
  }, AUTH_MODE_TTL_MS);
  entry.timer.unref?.();
  authModes.set(accountId, entry);
}

function clearAuthMode(accountId) {
  const entry = authModes.get(accountId);
  clearTimeout(entry?.timer);
  authModes.delete(accountId);
}

export class AuthSessionGoneError extends Error {
  constructor() {
    super("Nenhuma sessao de autenticacao pendente para esta account.");
    this.name = "AuthSessionGoneError";
    this.code = "auth_session_gone";
  }
}

function buildCompletePrompt(callbackUrl) {
  const urlLine = callbackUrl ? `\nURL de callback: ${callbackUrl}` : "";
  return (
    "O usuario autorizou no navegador; a autenticacao pode ja ter concluido sozinha. " +
    'Verifique seu acesso chamando ads_get_ad_accounts. Se conseguir, responda "OK: conectado". ' +
    "Se NAO tiver acesso e existir a ferramenta complete_authentication, chame-a com o callback_url." +
    urlLine +
    '\nSe complete_authentication NAO existir (erro "No such tool"), o login JA foi concluido - ' +
    'responda "OK: conectado". Responda em UMA frase comecando com "OK:" ou "ERRO:".'
  );
}

function extractUrl(text) {
  const match =
    typeof text === "string" ? text.match(/https?:\/\/[^\s)'\"]+/) : null;
  return match ? match[0] : "";
}

export async function authStart(rawAccountId, opts = {}) {
  const accountId = requireAccountId(rawAccountId);
  if (hasValidAccessToken(accountId)) {
    return { url: "", mode: "oauth", alreadyAuthed: true };
  }

  let oauthStart = null;
  try {
    oauthStart = await authStartOauth(accountId);
  } catch (err) {
    if (err instanceof OAuthCallbackConflictError) {
      throw err;
    }
  }
  if (oauthStart?.url) {
    setAuthMode(accountId, "oauth");
    return { url: oauthStart.url, mode: "oauth" };
  }

  const { reply } = await sessionPool.withSession(accountId, (session) =>
    session.run(AUTH_START_PROMPT, opts, { guard: false, auth: true }),
  );
  const url = extractUrl(reply);
  if (url) {
    setAuthMode(accountId, "session");
    return { url, mode: "session" };
  }
  if (/ja[_\s]?autenticad|ja (esta|estou) (conectad|autenticad)/i.test(reply)) {
    return { url: "", alreadyAuthed: true, mode: "session" };
  }
  throw new Error("authenticate nao devolveu uma URL de autorizacao");
}

export async function authComplete(rawAccountId, callbackUrl, opts = {}) {
  const accountId = requireAccountId(rawAccountId);
  const mode = authModes.get(accountId)?.mode;
  if (!mode) {
    throw new AuthSessionGoneError();
  }

  if (mode === "oauth") {
    const oauthDone = await authCompleteOauth(accountId, callbackUrl);
    clearAuthMode(accountId);
    if (!oauthDone.ok) {
      throw new AuthSessionGoneError();
    }
    recreateSession(accountId);
    return {
      ok: true,
      detail: "OK: conectado (token persistido em disco por account).",
    };
  }

  const { reply } = await sessionPool.withSession(accountId, (session) =>
    session.run(buildCompletePrompt(callbackUrl), opts, {
      guard: false,
      auth: true,
    }),
  );
  const ok =
    /(^|\s)ok:|conclu|conectad|autenticad|sucesso/i.test(reply) &&
    !/(^|\s)erro:|falh|expir|nao consegui|reconectar/i.test(reply);
  if (ok) {
    clearAuthMode(accountId);
  }
  return {
    ok,
    detail: `${reply} (fallback in-session - token nao persiste)`.trim(),
  };
}
