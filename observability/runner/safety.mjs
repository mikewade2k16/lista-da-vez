// Barreiras de segurança: o banco local é cópia de produção. Escrita só passa
// com TODAS as condições; caso contrário o endpoint de escrita vira skip.
import { config } from "../config.mjs";
import { hostOf, isLocalHost } from "./util.mjs";

export const WRITE_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);
export const WRITE_SKIP_REASON =
  "escrita desabilitada: requer banco de teste (OBS_TEST_DATABASE=1)";

export function looksLikeProduction(cfg = config) {
  const reasons = [];
  if (/^prod/i.test(cfg.env)) reasons.push(`env=${cfg.env}`);
  const apiHost = hostOf(cfg.apiUrl);
  if (!isLocalHost(apiHost) && !cfg.stagingHosts.includes(apiHost))
    reasons.push(`baseUrl não-local (${apiHost})`);
  const dbHost = hostOf(cfg.databaseUrl.replace(/^postgres(ql)?:/, "http:"));
  if (!isLocalHost(dbHost))
    reasons.push(`banco com host não-local (${dbHost})`);
  return reasons;
}

// Retorna { allowed, reason }. allowed só com todas as condições.
export function writePolicy(args, cfg = config) {
  const env = process.env;
  if (!args.allowWrites) return { allowed: false, reason: WRITE_SKIP_REASON };
  if (env.OBS_ALLOW_WRITES !== "1")
    return {
      allowed: false,
      reason: "escrita desabilitada: defina OBS_ALLOW_WRITES=1",
    };
  if (env.OBS_TEST_DATABASE !== "1")
    return { allowed: false, reason: WRITE_SKIP_REASON };
  if (looksLikeProduction(cfg).length)
    return {
      allowed: false,
      reason: "escrita bloqueada: ambiente parece produção",
    };
  return { allowed: true, reason: "" };
}

// Chamada no início: --allow-writes em ambiente de produção encerra com exit 2.
export function assertSafeStart(args, cfg = config) {
  if (args.allowWrites) {
    const why = looksLikeProduction(cfg);
    if (why.length)
      return `--allow-writes recusado: ambiente parece produção (${why.join("; ")})`;
  }
  return null;
}
