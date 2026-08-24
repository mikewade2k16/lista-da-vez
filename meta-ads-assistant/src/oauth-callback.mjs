import http from "node:http";

import { config } from "./config.mjs";
import { PendingOAuthRegistry } from "./oauth-pending.mjs";

const CALLBACK_TIMEOUT_MS = 10 * 60 * 1000;
const pendingAuth = new PendingOAuthRegistry();
let callbackServer = null;
let callbackServerStart = null;

// Um listener atende varias accounts porque o state roteia cada callback. Se a
// porta pertencer a outro processo, OAuthCallbackConflictError e mapeado pelo
// HTTP para 409; nunca se tenta capturar/completar o fluxo por outro tenant.

export class OAuthCallbackConflictError extends Error {
  constructor() {
    super("A porta do callback OAuth ja esta em uso.");
    this.name = "OAuthCallbackConflictError";
    this.code = "oauth_callback_conflict";
  }
}

function deferred() {
  const state = {};
  state.promise = new Promise((resolve, reject) => {
    state.resolve = resolve;
    state.reject = reject;
  });
  return state;
}

function callbackHTML() {
  return (
    '<!doctype html><meta charset="utf-8"><title>Omni</title>' +
    '<body style="font-family:sans-serif;padding:2rem">' +
    "<h2>Conexao com a Meta concluida</h2>" +
    "<p>Pode fechar esta aba e voltar ao painel.</p></body>"
  );
}

export async function ensureOAuthCallbackListener() {
  if (callbackServer?.listening) {
    return;
  }
  if (callbackServerStart) {
    return callbackServerStart;
  }
  callbackServerStart = new Promise((resolve, reject) => {
    const server = http.createServer((req, res) => {
      const reqUrl = new URL(
        req.url || "/",
        `http://127.0.0.1:${config.oauthCallbackPort}`,
      );
      if (reqUrl.pathname !== "/oauth/callback") {
        res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
        res.end("not found");
        return;
      }
      const code = reqUrl.searchParams.get("code") || "";
      const state = reqUrl.searchParams.get("state") || "";
      const match = pendingAuth.findByState(state);
      if (!match || code === "") {
        res.writeHead(400, { "Content-Type": "text/plain; charset=utf-8" });
        res.end("invalid oauth callback");
        return;
      }
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(callbackHTML());
      match.flow.callback.resolve({ code, state });
    });
    const onError = () => {
      callbackServer = null;
      reject(new OAuthCallbackConflictError());
    };
    server.once("error", onError);
    server.listen(config.oauthCallbackPort, "127.0.0.1", () => {
      server.off("error", onError);
      callbackServer = server;
      resolve();
    });
  }).finally(() => {
    callbackServerStart = null;
  });
  return callbackServerStart;
}

export function closeOAuthCallbackListener() {
  const server = callbackServer;
  callbackServer = null;
  try {
    server?.close();
  } catch {
    // Fechamento best-effort no shutdown do processo.
  }
}

export function getPendingOAuth(accountId) {
  return pendingAuth.get(accountId);
}

export function registerPendingOAuth(accountId, flowInput) {
  const callback = deferred();
  callback.promise.catch(() => {});
  const flow = { ...flowInput, callback, timer: null };
  flow.timer = setTimeout(() => {
    callback.reject(new Error("callback_timeout"));
    cancelPendingOAuth(accountId, flow);
  }, CALLBACK_TIMEOUT_MS);
  flow.timer.unref?.();
  pendingAuth.set(accountId, flow);
  return flow;
}

export function cancelPendingOAuth(accountId, expectedFlow) {
  const flow = pendingAuth.get(accountId);
  if (!flow || (expectedFlow !== undefined && flow !== expectedFlow)) {
    return false;
  }
  clearTimeout(flow.timer);
  pendingAuth.delete(accountId, flow);
  return true;
}
