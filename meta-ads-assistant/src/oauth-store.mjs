// Persistencia do OAuth do runner.
//
// O registro DCR e global porque representa este processo/redirect URI. Tokens
// da Meta sao tenant-scoped e vivem exclusivamente em
// .auth/accounts/<accountId>/tokens.json. O UUID validado impede traversal e a
// checagem de confinamento abaixo permanece como defesa em profundidade.

import {
  chmodSync,
  existsSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, isAbsolute, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { requireAccountId } from "./account-id.mjs";

const RUNNER_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const DEFAULT_AUTH_DIR = resolve(RUNNER_ROOT, ".auth");
const DIR_MODE = 0o700;
const FILE_MODE = 0o600;

function ensureSecureDir(path) {
  if (!existsSync(path)) {
    mkdirSync(path, { mode: DIR_MODE, recursive: true });
  }
  try {
    chmodSync(path, DIR_MODE);
  } catch {
    // Alguns filesystems (especialmente Windows) herdam ACL do diretorio pai.
  }
}

function writeJsonSecure(filePath, value) {
  ensureSecureDir(dirname(filePath));
  writeFileSync(filePath, JSON.stringify(value, null, 2), { mode: FILE_MODE });
  try {
    chmodSync(filePath, FILE_MODE);
  } catch {
    // Alguns filesystems nao implementam chmod; a ACL herdada continua valendo.
  }
}

function readJson(filePath) {
  if (!existsSync(filePath)) {
    return null;
  }
  try {
    const parsed = JSON.parse(readFileSync(filePath, "utf8"));
    return parsed && typeof parsed === "object" ? parsed : null;
  } catch {
    return null;
  }
}

function confinedPath(root, ...parts) {
  const resolvedRoot = resolve(root);
  const candidate = resolve(resolvedRoot, ...parts);
  const rel = relative(resolvedRoot, candidate);
  if (rel === "" || rel.startsWith("..") || isAbsolute(rel)) {
    throw new Error("oauth_path_outside_auth_dir");
  }
  return candidate;
}

export function createOAuthStore(authDir = DEFAULT_AUTH_DIR) {
  const root = resolve(authDir);
  const accountsDir = confinedPath(root, "accounts");
  const clientFile = confinedPath(root, "client.json");

  function accountDir(accountId) {
    return confinedPath(accountsDir, requireAccountId(accountId));
  }

  function tokensFile(accountId) {
    return confinedPath(accountDir(accountId), "tokens.json");
  }

  function loadAccountTokens(accountId) {
    const data = readJson(tokensFile(accountId));
    if (
      !data ||
      typeof data.access_token !== "string" ||
      data.access_token.trim() === ""
    ) {
      return null;
    }
    return {
      access_token: data.access_token,
      refresh_token:
        typeof data.refresh_token === "string" ? data.refresh_token : "",
      expires_at: Number.isFinite(data.expires_at) ? data.expires_at : 0,
      client_id: typeof data.client_id === "string" ? data.client_id : "",
      token_endpoint:
        typeof data.token_endpoint === "string" ? data.token_endpoint : "",
    };
  }

  return {
    paths: {
      authDir: root,
      accountsDir,
      clientFile,
      tokensFile,
    },

    loadClient() {
      const data = readJson(clientFile);
      if (
        !data ||
        typeof data.clientId !== "string" ||
        data.clientId.trim() === ""
      ) {
        return null;
      }
      return data;
    },

    saveClient(client) {
      writeJsonSecure(clientFile, client);
    },

    loadTokens: loadAccountTokens,

    saveTokens(
      accountId,
      { access_token, refresh_token, expiresInSec, clientId, tokenEndpoint },
    ) {
      if (typeof access_token !== "string" || access_token.trim() === "") {
        throw new TypeError("access_token ausente");
      }
      const expiresAt =
        Number.isFinite(expiresInSec) && expiresInSec > 0
          ? Date.now() + Math.floor(expiresInSec * 1000)
          : 0;
      writeJsonSecure(tokensFile(accountId), {
        access_token,
        refresh_token: typeof refresh_token === "string" ? refresh_token : "",
        expires_at: expiresAt,
        client_id: typeof clientId === "string" ? clientId : "",
        token_endpoint:
          typeof tokenEndpoint === "string" ? tokenEndpoint : "",
      });
    },

    clearTokens(accountId) {
      try {
        rmSync(tokensFile(accountId), { force: true });
      } catch {
        // Limpeza best-effort. Nunca remove o diretorio compartilhado .auth.
      }
    },

    hasValidAccessToken(accountId, skewMs = 60000) {
      const tokens = loadAccountTokens(accountId);
      if (!tokens) {
        return false;
      }
      return tokens.expires_at === 0 || tokens.expires_at > Date.now() + skewMs;
    },
  };
}

const defaultStore = createOAuthStore();

export const loadClient = () => defaultStore.loadClient();
export const saveClient = (client) => defaultStore.saveClient(client);
export const loadTokens = (accountId) => defaultStore.loadTokens(accountId);
export const saveTokens = (accountId, tokens) =>
  defaultStore.saveTokens(accountId, tokens);
export const clearTokens = (accountId) => defaultStore.clearTokens(accountId);
export const hasValidAccessToken = (accountId, skewMs) =>
  defaultStore.hasValidAccessToken(accountId, skewMs);

export const AUTH_PATHS = {
  AUTH_DIR: defaultStore.paths.authDir,
  ACCOUNTS_DIR: defaultStore.paths.accountsDir,
  CLIENT_FILE: defaultStore.paths.clientFile,
  tokensFile: defaultStore.paths.tokensFile,
};
