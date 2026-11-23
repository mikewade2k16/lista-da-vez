// Configuração central do runner de observabilidade. Tudo que é limite,
// perfil ou lista de bloqueio mora aqui; variáveis de ambiente sobrescrevem.
import { readFileSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

export const ROOT = dirname(fileURLToPath(import.meta.url));
export const REPO = dirname(ROOT);

// .env.local (gitignored) é lido sem sobrescrever o que já está no ambiente.
export function loadEnvFile(file = join(ROOT, ".env.local")) {
  if (!existsSync(file)) return;
  for (const line of readFileSync(file, "utf8").split("\n")) {
    const m = line.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*?)\s*$/);
    if (!m || line.trim().startsWith("#")) continue;
    if (process.env[m[1]] === undefined)
      process.env[m[1]] = m[2].replace(/^(['"])(.*)\1$/, "$2");
  }
}
loadEnvFile();

const env = process.env;
const num = (v, d) =>
  v === undefined || v === "" || Number.isNaN(Number(v)) ? d : Number(v);

export const config = {
  env: env.OBS_ENV || env.APP_ENV || env.NODE_ENV || "development",
  apiUrl: (env.OBS_API_URL || "http://localhost:8080").replace(/\/$/, ""),
  webUrl: (env.OBS_WEB_URL || "http://localhost:3003").replace(/\/$/, ""),
  databaseUrl:
    env.OBS_DATABASE_URL || "postgres://omni:omni_dev@127.0.0.1:5432/omni",
  inventoryFile: env.OBS_INVENTORY || join(ROOT, "inventory.json"),
  runsDir: join(ROOT, "runs"),

  // Escrita só com OBS_ALLOW_WRITES=1 + OBS_TEST_DATABASE=1 + env != production
  // + baseUrl local/staging (veja safety.mjs).
  stagingHosts: (env.OBS_STAGING_HOSTS || "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean),

  repeat: num(env.OBS_REPEAT, 5),
  concurrency: num(env.OBS_CONCURRENCY, 4),
  // A API limita 1200 req/min por usuário; ficamos abaixo com folga.
  rateLimitPerMin: num(env.OBS_RATE_PER_MIN, 900),
  timeouts: {
    requestMs: 15000,
    pageMs: 45000,
    networkIdleMs: 12000,
    loginMs: 60000,
    dbStatementMs: 5000,
  },

  limits: {
    p95Ms: 500, // padrão por endpoint
    p95MsByModule: {}, // ex.: { reports: 1500 }
    p95MsByEndpoint: {}, // ex.: { 'GET /v1/x': 2000 }
    slowQueryMs: num(env.OBS_SLOW_QUERY_MS, 200),
    vitals: { lcp: 2500, inp: 200, cls: 0.1, ttfb: 800 },
    pageLoadMs: 5000,
    seqScanRows: 10000, // Seq Scan em relação com mais linhas que isso vira aviso
    regression: {
      pct: 30,
      minMs: 50,
      queriesPct: 50,
      queriesMin: 3,
      vitalsPct: 25,
    },
  },

  // Perfis: papel esperado + credenciais OBS_USER_<PERFIL>_EMAIL/PASSWORD.
  // ADMIN cai em OMNI_QA_EMAIL/OMNI_QA_PASSWORD se OBS_USER_ADMIN_* não existir.
  profiles: {
    ADMIN: { role: "platform_admin" },
    OWNER: { role: "owner" },
    MANAGER: { role: "manager" },
    CONSULTANT: { role: "consultant" },
  },
  defaultProfile: "ADMIN",

  // Valores fixos para path params que não se resolvem por listagem.
  paramHints: {},

  // GETs que nunca são chamados, mesmo em leitura. Cada entrada tem motivo.
  neverCall: [
    {
      re: /\/realtime(\/|$)|\/ws(\/|$)|websocket/i,
      reason: "websocket/stream de longa duração",
    },
    {
      re: /\/(stream|events|sse)(\/|$)/i,
      reason: "stream (SSE) de longa duração",
    },
    {
      re: /\/(export|exports)(\/|$)|\/export\b/i,
      reason: "export pode gerar arquivo/carga pesada",
    },
    { re: /\/download|\/backup/i, reason: "download grande" },
    {
      re: /\/(callback|oauth|authorize)(\/|$)/i,
      reason: "callback/OAuth dispara efeito colateral",
    },
    {
      re: /\/(sync|refresh|reprocess|rebuild|run|trigger)(\/|$)/i,
      reason: "GET que dispara sincronização/execução",
    },
    {
      re: /\/webhooks?(\/|$)/i,
      reason: "webhook de terceiros (verificação com efeito)",
    },
    {
      re: /\/(logout|signout)(\/|$)/i,
      reason: "encerra sessão do usuário de teste",
    },
    {
      re: /\/(media|files|attachments|blob)\/.+/i,
      reason: "binário/arquivo grande",
    },
  ],
  // GET cuja dependência externa (deps.external) pode custar ou ter efeito.
  skipExternalDeps: env.OBS_SKIP_EXTERNAL !== "0",

  load: {
    maxEndpoints: 5,
    durationSec: 10,
    connections: 10,
    // Teto de req/s para não estourar o rate limit por usuário (1200/min).
    overallRate: num(env.OBS_LOAD_RATE, 18),
  },

  health: {
    govulncheck: env.OBS_GOVULNCHECK !== "0",
    govulncheckImage: "golang:1.26.5-bookworm",
    externals: [
      {
        name: "whisper",
        url: env.OBS_WHISPER_URL || "http://127.0.0.1:8010/health",
      },
      { name: "n8n", url: env.OBS_N8N_URL || "http://127.0.0.1:5680/healthz" },
      { name: "waha", url: env.OBS_WAHA_URL || "http://127.0.0.1:3010/" },
      {
        name: "evolution",
        url: env.OBS_EVOLUTION_URL || "http://127.0.0.1:8085/",
      },
      {
        name: "meta-ads-assistant",
        url: env.OBS_META_ADS_URL || "http://127.0.0.1:8765/health",
      },
    ],
    auditTimeoutMs: 60000,
  },

  // Pesos máximos de penalidade da nota (soma 100; veja docs/SCORE.md).
  scoreWeights: { errors: 30, latency: 25, vitals: 20, db: 15, coverage: 10 },
};

export function credentialsFor(profile) {
  const p = profile.toUpperCase();
  let email = env[`OBS_USER_${p}_EMAIL`];
  let password = env[`OBS_USER_${p}_PASSWORD`];
  if ((!email || !password) && p === "ADMIN") {
    email = email || env.OMNI_QA_EMAIL;
    password = password || env.OMNI_QA_PASSWORD;
  }
  return email && password ? { email, password } : null;
}
