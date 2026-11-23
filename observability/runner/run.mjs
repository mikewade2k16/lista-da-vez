#!/usr/bin/env node
// CLI do runner de observabilidade. Exit: 0 ok, 1 regressão/falha de autorização, 2 recusa de segurança/config.
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { config, credentialsFor, REPO } from "../config.mjs";
import { assertSafeStart, writePolicy } from "./safety.mjs";
import {
  parseArgs,
  list,
  runId as makeRunId,
  sh,
  sleep,
  pad,
} from "./util.mjs";
import { createClient, loginProfiles } from "./http.mjs";
import { runApi } from "./api.mjs";
import { runPages } from "./pages.mjs";
import { runDb } from "./db.mjs";
import { enrichQueryCounts } from "./db-captured.mjs";
import { runLoad } from "./load.mjs";
import { runHealth } from "./health.mjs";
import { computeCoverage } from "./coverage.mjs";
import { computeScores } from "./score.mjs";
import { findBaseline, compareRuns } from "./compare.mjs";
import { renderReport } from "./report.mjs";
import { storeRun } from "./persist.mjs";

const PHASES = ["health", "db", "api", "pages", "load"];
const log = (m) => console.log(m);

function bool(v, d) {
  if (v === undefined) return d;
  return !["false", "0", "no"].includes(String(v));
}

export function buildArgs(argv) {
  const { flags } = parseArgs(argv);
  let only = list(flags.only);
  const bad = only.filter((o) => !PHASES.includes(o));
  if (bad.length)
    return {
      error: `--only inválido: ${bad.join(",")} (use ${PHASES.join("|")})`,
    };
  const load = !!flags.load || only.includes("load");
  if (!only.length)
    only = flags.page
      ? ["pages"]
      : flags.endpoint
        ? ["api"]
        : PHASES.filter((p) => p !== "load");
  if (load && !only.includes("load")) only.push("load");
  return {
    modules: list(flags.module),
    only,
    endpoint: flags.endpoint || null,
    page: flags.page || null,
    load,
    repeat: Math.max(1, Number(flags.repeat) || config.repeat),
    profile: String(flags.profile || config.defaultProfile).toUpperCase(),
    noDbStore: !!flags["no-db-store"],
    baseline: flags.baseline || null,
    failOnRegression: flags["no-fail-on-regression"]
      ? false
      : bool(flags["fail-on-regression"], true),
    allowWrites: !!flags["allow-writes"],
    govulncheck: !flags["no-govulncheck"],
  };
}

async function gitInfo() {
  const sha = (
    await sh("git", ["rev-parse", "HEAD"], { cwd: REPO, timeout: 5000 })
  ).stdout.trim();
  const branch = (
    await sh("git", ["rev-parse", "--abbrev-ref", "HEAD"], {
      cwd: REPO,
      timeout: 5000,
    })
  ).stdout.trim();
  return { sha, branch };
}

export async function main(argv) {
  const args = buildArgs(argv);
  if (args.error) {
    console.error(args.error);
    return 2;
  }
  const refuse = assertSafeStart(args);
  if (refuse) {
    console.error(`RECUSADO: ${refuse}`);
    return 2;
  }
  if (!args.govulncheck) config.health.govulncheck = false;
  if (!existsSync(config.inventoryFile)) {
    console.error(
      `inventário não encontrado: ${config.inventoryFile}\nGere com: npm run obs:inventory`,
    );
    return 2;
  }
  const inv = JSON.parse(readFileSync(config.inventoryFile, "utf8"));
  const id = makeRunId();
  const outDir = join(config.runsDir, id);
  mkdirSync(outDir, { recursive: true });
  const startedAt = new Date().toISOString();
  const policy = writePolicy(args);
  const baseline = findBaseline(id, args.baseline);
  if (args.baseline && !baseline) {
    console.error(`baseline ${args.baseline} não encontrada em runs/`);
    return 2;
  }
  const git = await gitInfo();

  log(`Run: ${id}`);
  log(
    `env=${config.env} api=${config.apiUrl} fases=${args.only.join(",")} módulos=${args.modules.join(",") || "todos"} repeat=${args.repeat} perfil=${args.profile}`,
  );
  log(
    `escrita: ${policy.allowed ? "HABILITADA (banco de teste)" : "desabilitada"} | baseline: ${baseline?.runId || "nenhum"}`,
  );

  const results = [];
  const ctx = {
    inv,
    args,
    runId: id,
    outDir,
    writePolicy: policy,
    baseline,
    log,
    results,
    sessions: {},
    client: createClient({ runId: id }),
  };
  let db = {},
    structural = new Set(),
    queried = new Set();
  const phaseTime = {};
  const timed = async (name, fn) => {
    const t0 = Date.now();
    log(`[${name}] ...`);
    const r = await fn();
    phaseTime[name] = Date.now() - t0;
    log(
      `[${name}] ${r.length ?? ""} resultados em ${(phaseTime[name] / 1000).toFixed(1)}s`,
    );
    return r;
  };

  if (args.only.some((p) => ["api", "pages", "load"].includes(p))) {
    const wanted = new Set([args.profile, ...Object.keys(config.profiles)]);
    ctx.sessions = await loginProfiles(
      ctx.client,
      [...wanted].filter((p) => p === args.profile || credentialsFor(p)),
      log,
    );
    const ok = Object.values(ctx.sessions)
      .filter((s) => s.token)
      .map((s) => s.profile);
    log(
      `sessões: ${ok.join(", ") || "nenhuma"}${ctx.sessions[args.profile]?.error ? ` (${ctx.sessions[args.profile].error})` : ""}`,
    );
  }

  if (args.only.includes("health"))
    results.push(...(await timed("health", runHealth)));
  if (args.only.includes("db")) {
    const t0 = Date.now();
    log("[db] ...");
    const r = await runDb(ctx);
    results.push(...r.results);
    db = r.db;
    structural = r.structural;
    queried = r.queried;
    log(
      `[db] ${r.results.length} resultados em ${((Date.now() - t0) / 1000).toFixed(1)}s`,
    );
  }
  let apiRes = [];
  if (args.only.includes("api")) {
    apiRes = await timed("api", () => runApi(ctx));
    results.push(...apiRes);
    await sleep(3500); // coletor assíncrono grava obs.http_requests
    const e = await enrichQueryCounts(
      apiRes.filter((r) => r.kind === "api"),
      id,
    );
    log(
      `  queries/req: ${e.ok ? `${e.matched} endpoints casados em obs.http_requests` : `indisponível (${e.why})`}`,
    );
  }
  if (args.only.includes("pages"))
    results.push(...(await timed("pages", () => runPages(ctx))));
  if (args.only.includes("load"))
    results.push(
      ...(await timed("load", () => runLoad({ ...ctx, api: apiRes }))),
    );

  const phases = new Set(args.only);
  const coverage = computeCoverage({
    inv,
    results,
    structural,
    queried,
    filters: args,
  });
  const scores = computeScores({ inv, results, coverage, phases });
  const finishedAt = new Date().toISOString();
  const count = (s) => results.filter((r) => r.status === s).length;
  const run = {
    runId: id,
    startedAt,
    finishedAt,
    git,
    env: config.env,
    baseUrl: config.apiUrl,
    filters: {
      modules: args.modules,
      only: args.only,
      endpoint: args.endpoint,
      page: args.page,
      repeat: args.repeat,
      profile: args.profile,
      load: args.load,
      writes: policy.allowed,
    },
    summary: {
      pass: count("pass"),
      warn: count("warn"),
      fail: count("fail"),
      skip: count("skip"),
      total: results.length,
      phaseMs: phaseTime,
    },
    results,
    db,
    coverage,
    scores,
    regressions: [],
    improvements: [],
    baselineId: baseline?.runId || null,
    exitCode: 0,
  };
  const cmp = compareRuns(run, baseline);
  run.regressions = cmp.regressions;
  run.improvements = cmp.improvements;
  const authzFails = results.filter(
    (r) => r.kind === "authz" && r.status === "fail",
  );
  if (args.failOnRegression && (run.regressions.length || authzFails.length))
    run.exitCode = 1;

  writeFileSync(join(outDir, "results.json"), JSON.stringify(run, null, 1));
  writeFileSync(join(outDir, "report.html"), renderReport(run));
  let stored = "não gravado (--no-db-store)";
  if (!args.noDbStore) {
    const s = await storeRun(run);
    stored = s.ok
      ? `gravado em obs.runs (${s.rows} resultados)`
      : `AVISO: não gravado no banco: ${s.why}`;
  }
  printSummary(run, authzFails, stored);
  return run.exitCode;
}

function printSummary(run, authzFails, stored) {
  const s = run.summary;
  log("\n=== Resumo ===");
  log(
    `pass ${s.pass} | warn ${s.warn} | fail ${s.fail} | skip ${s.skip}   (${s.total} resultados)`,
  );
  const c = run.coverage;
  log(
    `cobertura: endpoints ${c.endpoints.tested}/${c.endpoints.total} (${c.endpoints.pct}%) | páginas ${c.pages.tested}/${c.pages.total} (${c.pages.pct}%) | tabelas ${c.tables.tested}/${c.tables.total} (${c.tables.pct}%)`,
  );
  const sc = Object.entries(run.scores)
    .filter(([, v]) => v.score != null)
    .sort((a, b) => a[1].score - b[1].score)
    .slice(0, 8);
  if (sc.length)
    log("notas (piores): " + sc.map(([m, v]) => `${m}=${v.score}`).join(" "));
  for (const r of authzFails.slice(0, 10))
    log(`  !! AUTHZ ${r.target}: ${r.reason}`);
  for (const r of run.results
    .filter((x) => x.status === "fail" && x.kind !== "authz")
    .slice(0, 10))
    log(`  FAIL ${pad(r.kind, 6)} ${r.target}: ${r.reason}`);
  for (const r of run.regressions.slice(0, 15)) log(`  REGRESSÃO ${r.message}`);
  for (const r of run.improvements.slice(0, 5)) log(`  melhoria ${r.message}`);
  log(
    `regressões: ${run.regressions.length} | melhorias: ${run.improvements.length} | baseline: ${run.baselineId || "nenhum"}`,
  );
  log(`banco: ${stored}`);
  log(`relatório: observability/runs/${run.runId}/report.html`);
  log(`exit ${run.exitCode}`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main(process.argv.slice(2)).then(
    (code) => process.exit(code),
    (e) => {
      console.error(e);
      process.exit(2);
    },
  );
}
