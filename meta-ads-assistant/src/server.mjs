// HTTP interno do runner Meta Ads. Todas as rotas de operacao, inclusive
// health, exigem Bearer de servico e accountId UUID validado.

import { createHash, timingSafeEqual } from "node:crypto";
import http from "node:http";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { InvalidAccountIdError, requireAccountId } from "./account-id.mjs";
import { AssistantTimeoutError } from "./agent.mjs";
import { AuthSessionGoneError, authComplete, authStart } from "./auth.mjs";
import { claudeAuthStatus, config, metaAuthStatus } from "./config.mjs";
import {
  closeOAuthCallbackListener,
  OAuthCallbackConflictError,
} from "./oauth-callback.mjs";
import { runAssistant, sessionPool } from "./session.mjs";
import { SessionCapacityError } from "./session-pool.mjs";

const MAX_BODY_BYTES = 1 << 20;
const ROUTES_WITH_METHODS = [
  "/healthz",
  "/run",
  "/auth/start",
  "/auth/complete",
];

function bearerEquals(header, token) {
  const prefix = "Bearer ";
  if (typeof header !== "string" || !header.startsWith(prefix)) {
    return false;
  }
  const gotDigest = createHash("sha256")
    .update(header.slice(prefix.length).trim())
    .digest();
  const wantDigest = createHash("sha256").update(token).digest();
  return timingSafeEqual(gotDigest, wantDigest);
}

function writeJSON(res, statusCode, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(statusCode, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(body),
  });
  res.end(body);
}

function writeError(res, statusCode, errorCode, message) {
  writeJSON(res, statusCode, { error: errorCode, message });
}

function safeLogFailure(operation, err) {
  const name = typeof err?.name === "string" ? err.name : "Error";
  const code = typeof err?.code === "string" ? err.code : "";
  // Nunca inclui message, request/body, URL de callback, headers ou tokens.
  console.error(`[meta-ads-assistant] ${operation} falhou`, { name, code });
}

function readBody(req) {
  return new Promise((resolveBody, reject) => {
    const chunks = [];
    let total = 0;
    let exceeded = false;
    req.on("data", (chunk) => {
      if (exceeded) {
        return;
      }
      total += chunk.length;
      if (total > MAX_BODY_BYTES) {
        exceeded = true;
        chunks.length = 0;
        reject(new Error("body_too_large"));
        return;
      }
      chunks.push(chunk);
    });
    req.on("end", () => {
      if (!exceeded) {
        resolveBody(Buffer.concat(chunks).toString("utf8"));
      }
    });
    req.on("error", reject);
  });
}

function parseJSON(rawBody) {
  let parsed;
  try {
    parsed = JSON.parse(rawBody || "{}");
  } catch {
    return { error: "JSON invalido." };
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return { error: "Body deve ser um objeto JSON." };
  }
  return { parsed };
}

function parseAccountId(value) {
  try {
    return { accountId: requireAccountId(value) };
  } catch {
    return { error: 'Campo "accountId" deve ser um UUID valido.' };
  }
}

function parseRunPayload(rawBody) {
  const json = parseJSON(rawBody);
  if (json.error) {
    return json;
  }
  const parsed = json.parsed;
  const account = parseAccountId(parsed.accountId);
  if (account.error) {
    return { error: account.error, errorCode: "invalid_account_id" };
  }
  const prompt = typeof parsed.prompt === "string" ? parsed.prompt.trim() : "";
  if (prompt === "") {
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
      if (
        (role !== "user" && role !== "assistant") ||
        typeof content !== "string"
      ) {
        return {
          error:
            'Itens de "history" devem ter role user|assistant e content string.',
        };
      }
      history.push({ role, content });
    }
  }
  return {
    payload: {
      prompt,
      history,
      adAccountId:
        typeof parsed.adAccountId === "string" ? parsed.adAccountId.trim() : "",
      accountId: account.accountId,
      model: typeof parsed.model === "string" ? parsed.model : "",
      systemPrompt:
        typeof parsed.systemPrompt === "string" ? parsed.systemPrompt : "",
    },
  };
}

function optsFromParsed(parsed) {
  return {
    model: typeof parsed?.model === "string" ? parsed.model : "",
    systemPrompt:
      typeof parsed?.systemPrompt === "string" ? parsed.systemPrompt : "",
  };
}

function requireRunnerAuth(req, res) {
  if (config.token === "") {
    writeError(
      res,
      503,
      "runner_not_configured",
      "META_ADS_ASSISTANT_TOKEN nao configurado.",
    );
    return false;
  }
  if (!bearerEquals(req.headers.authorization, config.token)) {
    writeError(res, 401, "unauthorized", "Token de servico invalido.");
    return false;
  }
  return true;
}

function writeRuntimeError(res, err) {
  if (err instanceof SessionCapacityError) {
    writeError(
      res,
      503,
      "session_capacity",
      "Runner ocupado. Tente novamente em instantes.",
    );
    return true;
  }
  if (err instanceof OAuthCallbackConflictError) {
    writeError(
      res,
      409,
      "oauth_callback_conflict",
      "A porta do callback OAuth esta em uso. Conclua o login em andamento e tente novamente.",
    );
    return true;
  }
  if (err instanceof AuthSessionGoneError) {
    writeError(
      res,
      409,
      "auth_session_gone",
      "A sessao de login expirou. Gere o link novamente.",
    );
    return true;
  }
  if (err instanceof InvalidAccountIdError) {
    writeError(
      res,
      400,
      "invalid_account_id",
      'Campo "accountId" deve ser um UUID valido.',
    );
    return true;
  }
  return false;
}

async function parseAuthenticatedJSON(req, res) {
  if (!requireRunnerAuth(req, res)) {
    return null;
  }
  let rawBody;
  try {
    rawBody = await readBody(req);
  } catch (err) {
    if (err?.message === "body_too_large") {
      writeError(res, 413, "body_too_large", "Body acima de 1MB.");
    } else {
      writeError(res, 400, "invalid_body", "Falha ao ler o body.");
    }
    return null;
  }
  const result = parseJSON(rawBody);
  if (result.error) {
    writeError(res, 400, "invalid_body", result.error);
    return null;
  }
  const account = parseAccountId(result.parsed.accountId);
  if (account.error) {
    writeError(res, 400, "invalid_account_id", account.error);
    return null;
  }
  return { parsed: result.parsed, accountId: account.accountId };
}

function handleHealthz(req, res) {
  if (!requireRunnerAuth(req, res)) {
    return;
  }
  const reqUrl = new URL(req.url || "/healthz", "http://runner.internal");
  const account = parseAccountId(reqUrl.searchParams.get("accountId"));
  if (account.error) {
    writeError(res, 400, "invalid_account_id", account.error);
    return;
  }
  const auth = claudeAuthStatus();
  const metaAuth = metaAuthStatus(
    account.accountId,
    sessionPool.hasMetaTools(account.accountId),
  );
  writeJSON(res, 200, {
    ok: true,
    claudeAuth: auth.ok,
    detail: auth.detail,
    metaAuth,
  });
}

async function handleRun(req, res) {
  if (!requireRunnerAuth(req, res)) {
    return;
  }
  let rawBody;
  try {
    rawBody = await readBody(req);
  } catch (err) {
    if (err?.message === "body_too_large") {
      writeError(res, 413, "body_too_large", "Body acima de 1MB.");
    } else {
      writeError(res, 400, "invalid_body", "Falha ao ler o body.");
    }
    return;
  }
  const { payload, error, errorCode } = parseRunPayload(rawBody);
  if (error) {
    writeError(res, 400, errorCode || "invalid_body", error);
    return;
  }
  try {
    writeJSON(res, 200, await runAssistant(payload));
  } catch (err) {
    if (err instanceof AssistantTimeoutError) {
      writeError(res, 504, "assistant_timeout", err.message);
      return;
    }
    if (writeRuntimeError(res, err)) {
      return;
    }
    safeLogFailure("run", err);
    writeError(
      res,
      502,
      "assistant_error",
      "O assistente falhou ao executar o comando.",
    );
  }
}

async function handleAuthStart(req, res) {
  const input = await parseAuthenticatedJSON(req, res);
  if (!input) {
    return;
  }
  try {
    const result = await authStart(
      input.accountId,
      optsFromParsed(input.parsed),
    );
    writeJSON(res, 200, {
      url: result.url,
      mode: result.mode,
      alreadyAuthed: result.alreadyAuthed === true,
    });
  } catch (err) {
    if (writeRuntimeError(res, err)) {
      return;
    }
    safeLogFailure("auth/start", err);
    writeError(
      res,
      502,
      "auth_error",
      "Falha ao iniciar a autenticacao com a Meta.",
    );
  }
}

async function handleAuthComplete(req, res) {
  const input = await parseAuthenticatedJSON(req, res);
  if (!input) {
    return;
  }
  const callbackUrl =
    typeof input.parsed.callbackUrl === "string"
      ? input.parsed.callbackUrl.trim()
      : "";
  try {
    const result = await authComplete(
      input.accountId,
      callbackUrl,
      optsFromParsed(input.parsed),
    );
    writeJSON(res, 200, { ok: result.ok, detail: result.detail });
  } catch (err) {
    if (writeRuntimeError(res, err)) {
      return;
    }
    safeLogFailure("auth/complete", err);
    writeError(
      res,
      502,
      "auth_error",
      "Falha ao concluir a autenticacao com a Meta.",
    );
  }
}

export function createRunnerServer() {
  return http.createServer((req, res) => {
    const path = (req.url || "/").split("?")[0];
    if (path === "/healthz" && req.method === "GET") {
      handleHealthz(req, res);
      return;
    }
    let handler = null;
    switch (path) {
      case "/run":
        handler = handleRun;
        break;
      case "/auth/start":
        handler = handleAuthStart;
        break;
      case "/auth/complete":
        handler = handleAuthComplete;
        break;
    }
    if (req.method === "POST" && handler) {
      handler(req, res).catch((err) => {
        safeLogFailure("request", err);
        if (!res.headersSent) {
          writeError(res, 500, "internal_error", "Erro interno do runner.");
        }
      });
      return;
    }
    if (ROUTES_WITH_METHODS.includes(path)) {
      writeError(
        res,
        405,
        "method_not_allowed",
        "Metodo nao suportado nesta rota.",
      );
      return;
    }
    writeError(res, 404, "not_found", "Rota inexistente.");
  });
}

export function startRunnerServer() {
  const server = createRunnerServer();
  server.listen(config.port, "0.0.0.0", () => {
    const auth = claudeAuthStatus();
    console.warn(
      `[meta-ads-assistant] ouvindo em 0.0.0.0:${config.port} | ` +
        `token configurado: ${config.token !== "" ? "sim" : "NAO"} | ` +
        `claudeAuth: ${auth.ok ? "ok" : "ausente"}`,
    );
  });
  const shutdown = () => {
    sessionPool.closeAll();
    closeOAuthCallbackListener();
    server.close();
  };
  process.once("SIGTERM", shutdown);
  process.once("SIGINT", shutdown);
  return server;
}

const isEntrypoint =
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isEntrypoint) {
  startRunnerServer();
}
