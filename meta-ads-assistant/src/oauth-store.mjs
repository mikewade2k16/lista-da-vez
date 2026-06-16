// Persistencia em disco do OAuth proprio do runner (Problema 1).
//
// Guarda o registro do client (DCR) e os tokens (access/refresh/expiry) na pasta
// meta-ads-assistant/.auth/ com permissoes restritas (dir 0700, arquivos 0600).
// Nunca loga conteudo de token. Tudo best-effort: leitura de arquivo ausente ou
// corrompido devolve null (estado deslogado), nunca lanca para o caller.

import {
  chmodSync,
  existsSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const RUNNER_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const AUTH_DIR = join(RUNNER_ROOT, '.auth');
const CLIENT_FILE = join(AUTH_DIR, 'client.json');
const TOKENS_FILE = join(AUTH_DIR, 'tokens.json');

const DIR_MODE = 0o700;
const FILE_MODE = 0o600;

// ensureAuthDir garante a pasta .auth/ com mode 0700. Idempotente.
function ensureAuthDir() {
  if (!existsSync(AUTH_DIR)) {
    mkdirSync(AUTH_DIR, { mode: DIR_MODE, recursive: true });
    return;
  }
  // Pasta ja existe: reforca o mode (no-op em FS que ignora chmod, ex.: Windows).
  try {
    chmodSync(AUTH_DIR, DIR_MODE);
  } catch {
    /* FS sem suporte a chmod; permissao herda do diretorio pai */
  }
}

// writeJsonSecure grava JSON com mode 0600 (best-effort no chmod).
function writeJsonSecure(filePath, value) {
  ensureAuthDir();
  const body = JSON.stringify(value, null, 2);
  writeFileSync(filePath, body, { mode: FILE_MODE });
  try {
    chmodSync(filePath, FILE_MODE);
  } catch {
    /* FS sem suporte a chmod */
  }
}

// readJson le e parseia; arquivo ausente/corrompido => null (sem lancar).
function readJson(filePath) {
  if (!existsSync(filePath)) {
    return null;
  }
  try {
    const raw = readFileSync(filePath, 'utf8');
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : null;
  } catch {
    return null;
  }
}

// loadClient devolve o registro DCR salvo { clientId, registrationEndpoint, ... }
// ou null se ainda nao houve registro.
export function loadClient() {
  const data = readJson(CLIENT_FILE);
  if (!data || typeof data.clientId !== 'string' || data.clientId.trim() === '') {
    return null;
  }
  return data;
}

// saveClient persiste o registro DCR. Reusa o mesmo client_id entre boots.
export function saveClient(client) {
  writeJsonSecure(CLIENT_FILE, client);
}

// loadTokens devolve { access_token, refresh_token, expires_at } salvos, ou null.
// expires_at e epoch em ms.
export function loadTokens() {
  const data = readJson(TOKENS_FILE);
  if (!data || typeof data.access_token !== 'string' || data.access_token.trim() === '') {
    return null;
  }
  return {
    access_token: data.access_token,
    refresh_token: typeof data.refresh_token === 'string' ? data.refresh_token : '',
    expires_at: Number.isFinite(data.expires_at) ? data.expires_at : 0,
  };
}

// saveTokens persiste os tokens (0600). expiresInSec vira expires_at absoluto.
export function saveTokens({ access_token, refresh_token, expiresInSec }) {
  const expiresAt =
    Number.isFinite(expiresInSec) && expiresInSec > 0
      ? Date.now() + Math.floor(expiresInSec * 1000)
      : 0;
  writeJsonSecure(TOKENS_FILE, {
    access_token,
    refresh_token: typeof refresh_token === 'string' ? refresh_token : '',
    expires_at: expiresAt,
  });
}

// clearTokens apaga o arquivo de tokens (volta ao estado deslogado). Mantem o
// registro do client (DCR) para reuso. Best-effort.
export function clearTokens() {
  try {
    if (existsSync(TOKENS_FILE)) {
      rmSync(TOKENS_FILE, { force: true });
    }
  } catch {
    /* noop */
  }
}

// hasValidAccessToken: ha token em disco que ainda nao expirou (com folga)?
export function hasValidAccessToken(skewMs = 60000) {
  const tokens = loadTokens();
  if (!tokens) {
    return false;
  }
  if (tokens.expires_at === 0) {
    // Sem expiry conhecido: considera valido (servidores que nao mandam expires_in).
    return true;
  }
  return tokens.expires_at > Date.now() + skewMs;
}

export const AUTH_PATHS = { AUTH_DIR, CLIENT_FILE, TOKENS_FILE };
