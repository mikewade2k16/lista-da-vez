// Nota por módulo (0-100) = 100 - penalidades. Fórmula (espelhada em docs/SCORE.md):
//  erros     (máx 30): min(30, 10*falhas authz + 5*falhas 5xx/rede + 3*outras falhas)
//  latência  (máx 25): 25 * média, sobre GETs medidos do módulo, de clamp((p95/limite - 1)/2, 0, 1)
//                      (no limite = 0; 3x o limite ou mais = penalidade cheia)
//  vitals    (máx 20): 20 * média, sobre páginas medidas, da fração de métricas fora do limite
//                      (LCP/INP/CLS/TTFB: acima do limite = 0.5, acima de 2x = 1.0; média das métricas medidas)
//  banco     (máx 15): min(15, 0.5*FK sem índice + 0.5*índice duplicado + 1*tabela sem PK
//                      + 1*query lenta + 2*N+1 + 1*Seq Scan grande), só itens atribuídos ao módulo
//  cobertura (máx 10): 10 * (1 - média dos % testados de endpoints/páginas/tabelas
//                      das fases efetivamente executadas)
// Módulo sem nenhum resultado medido (tudo skip) fica com score null ("sem dados").
import { config } from "../config.mjs";

const PSEUDO = new Set(["database", "infra", "unknown"]);
const clamp = (x, a = 0, b = 1) => Math.min(b, Math.max(a, x));
const r1 = (n) => Math.round(n * 10) / 10;

export function computeScores({ inv, results, coverage, phases }) {
  const W = config.scoreWeights;
  const mods = new Set([
    ...(inv?.modules || []).map((m) => m.id),
    ...results.map((r) => r.module),
  ]);
  const scores = {};
  for (const mod of mods) {
    if (PSEUDO.has(mod)) continue;
    const rs = results.filter((r) => r.module === mod);
    const measured = rs.filter(
      (r) =>
        ["api", "page", "db", "authz"].includes(r.kind) && r.status !== "skip",
    );
    if (!measured.length) {
      scores[mod] = {
        score: null,
        reason: "sem resultados medidos (tudo skip ou fase não executada)",
        components: {},
      };
      continue;
    }

    const fails = rs.filter(
      (r) =>
        r.status === "fail" &&
        ["api", "page", "authz", "db", "load"].includes(r.kind),
    );
    const authz = fails.filter((r) => r.kind === "authz").length;
    const hard = fails.filter(
      (r) =>
        r.kind !== "authz" &&
        (r.metrics?.status >= 500 ||
          r.metrics?.status === 0 ||
          /5xx|HTTP 5|sem resposta|navegação/.test(r.reason)),
    ).length;
    const errors =
      clamp(
        (10 * authz + 5 * hard + 3 * (fails.length - authz - hard)) / W.errors,
      ) * W.errors;

    const gets = rs.filter(
      (r) =>
        r.kind === "api" &&
        r.status !== "skip" &&
        r.metrics?.p95 != null &&
        r.target.startsWith("GET "),
    );
    const latency = gets.length
      ? W.latency *
        (gets.reduce(
          (a, r) => a + clamp((r.metrics.p95 / r.metrics.limitMs - 1) / 2),
          0,
        ) /
          gets.length)
      : 0;

    const lim = config.limits.vitals;
    const pages = rs.filter(
      (r) => r.kind === "page" && r.status !== "skip" && r.status !== "fail",
    );
    const pageFrac = (m) => {
      const parts = Object.entries(lim)
        .map(([k, l]) =>
          m[k] == null ? null : m[k] > 2 * l ? 1 : m[k] > l ? 0.5 : 0,
        )
        .filter((x) => x !== null);
      return parts.length
        ? parts.reduce((a, b) => a + b, 0) / parts.length
        : null;
    };
    const fr = pages.map((p) => pageFrac(p.metrics)).filter((x) => x !== null);
    const vitals = fr.length
      ? W.vitals * (fr.reduce((a, b) => a + b, 0) / fr.length)
      : 0;

    let dbPts = 0;
    for (const r of rs.filter(
      (x) => x.kind === "db" && x.status !== "skip" && x.status !== "pass",
    )) {
      const t = r.target;
      dbPts += t.startsWith("db:fk-sem-indice")
        ? 0.5
        : t.startsWith("db:indice-duplicado")
          ? 0.5
          : t.startsWith("db:sem-pk")
            ? 1
            : t.startsWith("db:slow")
              ? 1
              : t.startsWith("db:n+1")
                ? 2
                : t.startsWith("db:explain")
                  ? 1
                  : 0;
    }
    const db = Math.min(W.db, dbPts);

    const cov = coverage?.byModule?.[mod];
    const ran = {
      endpoints: phases.has("api"),
      pages: phases.has("pages"),
      tables: phases.has("db"),
    };
    const pcts = cov
      ? Object.entries(cov)
          .filter(([k, v]) => ran[k] && v.total > 0)
          .map(([, v]) => v.pct / 100)
      : [];
    const covPen = pcts.length
      ? W.coverage * (1 - pcts.reduce((a, b) => a + b, 0) / pcts.length)
      : 0;

    const comps = {
      errors: r1(errors),
      latency: r1(latency),
      vitals: r1(vitals),
      db: r1(db),
      coverage: r1(covPen),
    };
    const total = Object.values(comps).reduce((a, b) => a + b, 0);
    scores[mod] = {
      score: Math.max(0, Math.round(100 - total)),
      components: comps,
      counts: {
        fails: fails.length,
        authzFails: authz,
        endpointsMeasured: gets.length,
        pagesMeasured: pages.length,
      },
    };
  }
  return scores;
}
