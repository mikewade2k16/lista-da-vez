// Persistência no Postgres (obs.runs / obs.run_results). Tolerante a ausência das tabelas.
import { config } from "../config.mjs";
import { hostOf, isLocalHost } from "./util.mjs";

export async function openWriteConn() {
  const host = hostOf(config.databaseUrl.replace(/^postgres(ql)?:/, "http:"));
  if (!isLocalHost(host) && process.env.OBS_ALLOW_REMOTE_DB_STORE !== "1")
    throw new Error(
      `banco não-local (${host}): defina OBS_ALLOW_REMOTE_DB_STORE=1 para gravar`,
    );
  const { default: pg } = await import("pg");
  const c = new pg.Client({
    connectionString: config.databaseUrl,
    statement_timeout: 30000,
  });
  await c.connect();
  return c;
}

export async function columns(c, schema, table) {
  const r = await c.query(
    "select column_name from information_schema.columns where table_schema=$1 and table_name=$2",
    [schema, table],
  );
  return new Set(r.rows.map((x) => x.column_name));
}

// Insere só as colunas que existem (o schema pode evoluir sem quebrar o runner).
export async function insertRow(c, table, row, cols) {
  const have = cols || (await columns(c, "obs", table));
  const keys = Object.keys(row).filter((k) => have.has(k));
  const sql = `insert into obs.${table} (${keys.join(",")}) values (${keys.map((_, i) => `$${i + 1}`).join(",")})`;
  await c.query(
    sql,
    keys.map((k) =>
      row[k] !== null && typeof row[k] === "object"
        ? JSON.stringify(row[k])
        : row[k],
    ),
  );
}

export async function storeRun(run) {
  let c;
  try {
    c = await openWriteConn();
  } catch (e) {
    return { ok: false, why: e.message };
  }
  try {
    const ok = await c.query(
      `select to_regclass('obs.runs') a, to_regclass('obs.run_results') b`,
    );
    if (!ok.rows[0].a || !ok.rows[0].b)
      return {
        ok: false,
        why: "tabelas obs.runs/obs.run_results não existem (migration 0329 não aplicada)",
      };
    const runCols = await columns(c, "obs", "runs");
    const resCols = await columns(c, "obs", "run_results");
    await c.query("begin");
    await insertRow(
      c,
      "runs",
      {
        id: run.runId,
        started_at: run.startedAt,
        finished_at: run.finishedAt,
        git_sha: run.git?.sha,
        branch: run.git?.branch,
        env: run.env,
        base_url: run.baseUrl,
        filters: run.filters,
        summary: run.summary,
        coverage: run.coverage,
        scores: run.scores,
        regressions: run.regressions,
        exit_code: run.exitCode,
      },
      runCols,
    );
    for (const r of run.results) {
      await insertRow(
        c,
        "run_results",
        {
          run_id: run.runId,
          kind: r.kind,
          module: r.module,
          target: r.target,
          status: r.status,
          reason: r.reason || null,
          metrics: r.metrics || {},
        },
        resCols,
      );
    }
    await c.query("commit");
    return { ok: true, rows: run.results.length };
  } catch (e) {
    await c.query("rollback").catch(() => {});
    return { ok: false, why: e.message };
  } finally {
    await c.end().catch(() => {});
  }
}
