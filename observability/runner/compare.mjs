// Comparação com baseline (run anterior em runs/ ou id informado): regressões e melhorias.
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { config } from "../config.mjs";

export function loadRun(id) {
  const f = join(config.runsDir, id, "results.json");
  return existsSync(f) ? JSON.parse(readFileSync(f, "utf8")) : null;
}

// Baseline = id pedido, senão o run mais recente (por nome) diferente do atual.
export function findBaseline(currentId, wanted) {
  let id = wanted;
  if (!id) {
    if (!existsSync(config.runsDir)) return null;
    id = readdirSync(config.runsDir)
      .filter(
        (d) =>
          d !== currentId &&
          /^\d{8}-\d{6}/.test(d) &&
          existsSync(join(config.runsDir, d, "results.json")),
      )
      .sort()
      .pop();
  }
  if (!id) return null;
  const run = loadRun(id);
  if (!run) return null;
  run.byTarget = new Map(run.results.map((r) => [`${r.kind}|${r.target}`, r]));
  return run;
}

const pct = (a, b) => (a ? Math.round(((b - a) / a) * 1000) / 10 : null);

export function compareRuns(current, baseline) {
  const L = config.limits,
    R = L.regression;
  const regressions = [],
    improvements = [];
  if (!baseline) return { regressions, improvements, baselineId: null };
  const push = (arr, kind, r, metric, before, after, msg) =>
    arr.push({
      kind,
      module: r.module,
      target: r.target,
      metric,
      before,
      after,
      deltaPct: pct(before, after),
      message: msg,
    });
  for (const r of current.results) {
    const old = baseline.byTarget.get(`${r.kind}|${r.target}`);
    if (!old) continue;
    if (old.status === "pass" && r.status === "fail")
      push(
        regressions,
        r.kind,
        r,
        "status",
        "pass",
        "fail",
        `${r.target} passou de pass para fail: ${r.reason}`,
      );
    else if (old.status === "fail" && r.status === "pass")
      push(
        improvements,
        r.kind,
        r,
        "status",
        "fail",
        "pass",
        `${r.target} voltou de fail para pass`,
      );
    const a = old.metrics || {},
      b = r.metrics || {};
    if (
      r.kind === "api" &&
      a.p95 != null &&
      b.p95 != null &&
      r.status !== "skip" &&
      old.status !== "skip"
    ) {
      const d = b.p95 - a.p95;
      if (d > R.minMs && pct(a.p95, b.p95) > R.pct)
        push(
          regressions,
          "api",
          r,
          "p95",
          a.p95,
          b.p95,
          `${r.target} p95 subiu de ${a.p95}ms para ${b.p95}ms`,
        );
      else if (-d > R.minMs && -pct(a.p95, b.p95) > R.pct)
        push(
          improvements,
          "api",
          r,
          "p95",
          a.p95,
          b.p95,
          `${r.target} p95 caiu de ${a.p95}ms para ${b.p95}ms`,
        );
      if (
        a.queriesPerReq != null &&
        b.queriesPerReq != null &&
        b.queriesPerReq - a.queriesPerReq >= R.queriesMin &&
        pct(a.queriesPerReq, b.queriesPerReq) > R.queriesPct
      )
        push(
          regressions,
          "api",
          r,
          "queries",
          a.queriesPerReq,
          b.queriesPerReq,
          `${r.target} queries/req subiu de ${a.queriesPerReq} para ${b.queriesPerReq}`,
        );
    }
    if (r.kind === "page") {
      for (const [k, lim] of Object.entries(L.vitals)) {
        if (a[k] == null || b[k] == null) continue;
        if (b[k] > lim && pct(a[k], b[k]) > R.vitalsPct && b[k] > a[k])
          push(
            regressions,
            "page",
            r,
            k,
            a[k],
            b[k],
            `${r.target} ${k.toUpperCase()} piorou de ${a[k]} para ${b[k]} (limite ${lim})`,
          );
        else if (a[k] > lim && b[k] <= lim)
          push(
            improvements,
            "page",
            r,
            k,
            a[k],
            b[k],
            `${r.target} ${k.toUpperCase()} voltou ao limite: ${a[k]} -> ${b[k]}`,
          );
      }
    }
    if (
      r.kind === "load" &&
      a.p99 != null &&
      b.p99 != null &&
      b.p99 - a.p99 > R.minMs &&
      pct(a.p99, b.p99) > R.pct
    )
      push(
        regressions,
        "load",
        r,
        "p99",
        a.p99,
        b.p99,
        `${r.target} p99 sob carga subiu de ${a.p99}ms para ${b.p99}ms`,
      );
  }
  return { regressions, improvements, baselineId: baseline.runId };
}

// CLI: node runner/compare.mjs [<runIdA> [<runIdB>]] — A=baseline (default: penúltimo), B=atual (default: último).
if (import.meta.url === `file://${process.argv[1]}`) {
  const ids = existsSync(config.runsDir)
    ? readdirSync(config.runsDir)
        .filter((d) => existsSync(join(config.runsDir, d, "results.json")))
        .sort()
    : [];
  const [a = ids.at(-2), b = ids.at(-1)] = process.argv
    .slice(2)
    .filter((x) => !x.startsWith("--"));
  const base = a && findBaseline(b, a),
    cur = b && loadRun(b);
  if (!base || !cur) {
    console.error("precisa de dois runs em observability/runs/");
    process.exit(2);
  }
  const { regressions, improvements } = compareRuns(cur, base);
  console.log(`baseline ${a} -> atual ${b}`);
  for (const r of regressions) console.log(`  REGRESSÃO ${r.message}`);
  for (const r of improvements) console.log(`  melhoria  ${r.message}`);
  console.log(
    `${regressions.length} regressões, ${improvements.length} melhorias`,
  );
  process.exit(regressions.length ? 1 : 0);
}
