// Dados capturados pela instrumentação (obs.*): queries lentas, N+1, top queries + EXPLAIN,
// e enriquecimento de queries/req dos resultados de API.
import { config } from "../config.mjs";
import {
  connectReadOnly,
  tableExists,
  columnsOf,
  tablesInStatement,
} from "./db.mjs";

export async function captured(c, { out, add, routeModule, queried }) {
  const have = {
    stats: await tableExists(c, "obs.db_query_stats"),
    slow: await tableExists(c, "obs.db_slow_queries"),
    n1: await tableExists(c, "obs.db_n_plus_one"),
  };
  if (!have.slow)
    add(
      "skip",
      "db",
      "database",
      "db:queries-lentas",
      "tabela obs.db_slow_queries não existe (migration 0329 não aplicada)",
    );
  else {
    try {
      const rows = (
        await c.query(
          `select route, fingerprint, left(statement, 400) as statement, duration_ms, correlation_id from obs.db_slow_queries order by duration_ms desc limit 20`,
        )
      ).rows;
      out.slowQueries = rows;
      add(
        rows.length ? "warn" : "pass",
        "db",
        "database",
        "db:queries-lentas",
        rows.length ? `${rows.length} consultas lentas capturadas` : "",
        { count: rows.length },
      );
      for (const r of rows) {
        if (r.statement)
          tablesInStatement(r.statement).forEach((t) => queried.add(t));
        add(
          "warn",
          "db",
          routeModule.get(r.route),
          `db:slow:${r.fingerprint}`,
          `consulta de ${Math.round(r.duration_ms)}ms em ${r.route || "?"}`,
          r,
        );
      }
    } catch (e) {
      add(
        "skip",
        "db",
        "database",
        "db:queries-lentas",
        `colunas inesperadas em obs.db_slow_queries: ${e.message}`,
      );
    }
  }
  if (have.n1) {
    try {
      const rows = (
        await c.query(
          `select route, fingerprint, left(statement,300) as statement, max(occurrences) as occurrences from obs.db_n_plus_one group by 1,2,3 order by 4 desc limit 20`,
        )
      ).rows;
      out.nPlusOne = rows;
      for (const r of rows)
        add(
          "warn",
          "db",
          routeModule.get(r.route),
          `db:n+1:${r.fingerprint}`,
          `N+1: ${r.occurrences}x em ${r.route}`,
          r,
        );
    } catch {
      /* colunas podem variar: não crítico */
    }
  }
  if (!have.stats) {
    add(
      "skip",
      "db",
      "database",
      "db:top-queries",
      "tabela obs.db_query_stats não existe (migration 0329 não aplicada)",
    );
    return;
  }
  let top = [];
  try {
    top = (
      await c.query(`select fingerprint, max(statement) as statement, sum(calls)::bigint as calls, sum(total_ms) as total_ms, max(max_ms) as max_ms
      from obs.db_query_stats group by fingerprint order by sum(total_ms) desc limit 50`)
    ).rows;
  } catch (e) {
    add(
      "skip",
      "db",
      "database",
      "db:top-queries",
      `colunas inesperadas em obs.db_query_stats: ${e.message}`,
    );
    return;
  }
  out.topQueries = top.slice(0, 20);
  for (const r of top)
    if (r.statement)
      tablesInStatement(r.statement).forEach((t) => queried.add(t));
  add(
    top.length ? "pass" : "skip",
    "db",
    "database",
    "db:top-queries",
    top.length ? "" : "sem queries capturadas ainda",
    { count: top.length },
  );
  await explainTop(c, top, out, add);
}

async function explainTop(c, top, out, add) {
  const ver = Number(
    (await c.query("show server_version_num")).rows[0].server_version_num,
  );
  let done = 0,
    skippedWrites = 0;
  for (const r of top) {
    if (done >= 10) break;
    const s = String(r.statement || "")
      .trim()
      .replace(/;$/, "");
    const tgt = `db:explain:${r.fingerprint}`;
    if (!/^(select|with)\b/i.test(s)) continue;
    if (
      /\b(insert|delete|truncate|drop|alter|create|grant|copy|call)\b/i.test(
        s,
      ) ||
      /(?<!for )\bupdate\b/i.test(s)
    ) {
      skippedWrites++;
      continue;
    }
    // Statements normalizados usam "?" no lugar de literais: viram $n e usam GENERIC_PLAN (PG16+).
    let n = 0;
    const q = s.includes("?") ? s.replace(/\?/g, () => `$${++n}`) : s;
    const hasParam = /\$\d+/.test(q);
    if (hasParam && ver < 160000) {
      done++;
      add(
        "skip",
        "db",
        "database",
        tgt,
        `statement parametrizado e PG ${ver} sem GENERIC_PLAN`,
      );
      continue;
    }
    done++;
    try {
      await c.query("BEGIN READ ONLY");
      await c.query(
        `SET LOCAL statement_timeout = ${config.timeouts.dbStatementMs}`,
      );
      const plan = (
        await c.query(`EXPLAIN ${hasParam ? "(GENERIC_PLAN) " : ""}${q}`)
      ).rows.map((x) => x["QUERY PLAN"]);
      await c.query("ROLLBACK");
      const seq = plan
        .map((l) => l.match(/Seq Scan on (\S+).*rows=(\d+)/))
        .filter((m) => m && Number(m[2]) >= config.limits.seqScanRows);
      out.explains.push({
        fingerprint: r.fingerprint,
        statement: s.slice(0, 300),
        plan: plan.slice(0, 40),
        seqScans: seq.map((m) => m[1]),
      });
      add(
        seq.length ? "warn" : "pass",
        "db",
        "database",
        tgt,
        seq.length
          ? `Seq Scan em ${seq.map((m) => m[1]).join(", ")} (>= ${config.limits.seqScanRows} linhas)`
          : "",
        { totalMs: r.total_ms, calls: Number(r.calls) },
      );
    } catch (e) {
      await c.query("ROLLBACK").catch(() => {});
      add(
        "skip",
        "db",
        "database",
        tgt,
        `EXPLAIN falhou: ${e.message.slice(0, 120)}`,
      );
    }
  }
  if (skippedWrites)
    add(
      "skip",
      "db",
      "database",
      "db:explain:escritas",
      `${skippedWrites} statements com escrita entre as top queries: não explicados`,
    );
}

// Enriquece resultados de API com query_count/query_ms de obs.http_requests
// (correlation_id obsrun-<runId>-N). Silencioso se a tabela não existir.
export async function enrichQueryCounts(apiResults, runId) {
  let c;
  try {
    c = await connectReadOnly();
  } catch {
    return { ok: false, why: "banco inacessível" };
  }
  try {
    if (!(await tableExists(c, "obs.http_requests")))
      return { ok: false, why: "obs.http_requests não existe" };
    const cols = await columnsOf(c, "obs", "http_requests");
    if (!cols.has("query_count") || !cols.has("correlation_id"))
      return { ok: false, why: "colunas ausentes" };
    const ids = apiResults.flatMap((r) => r.metrics?.requestIds || []);
    if (!ids.length) return { ok: true, matched: 0 };
    const rows = (
      await c.query(
        `select correlation_id, query_count, query_ms from obs.http_requests where correlation_id = any($1)`,
        [ids],
      )
    ).rows;
    const by = new Map(rows.map((r) => [r.correlation_id, r]));
    let matched = 0;
    for (const r of apiResults) {
      const hit = (r.metrics?.requestIds || [])
        .map((i) => by.get(i))
        .filter(Boolean);
      if (!hit.length) continue;
      matched++;
      r.metrics.queriesPerReq =
        Math.round(
          (hit.reduce((a, x) => a + Number(x.query_count || 0), 0) /
            hit.length) *
            10,
        ) / 10;
      r.metrics.queryMsPerReq =
        Math.round(
          (hit.reduce((a, x) => a + Number(x.query_ms || 0), 0) / hit.length) *
            10,
        ) / 10;
    }
    void runId;
    return { ok: true, matched };
  } catch (e) {
    return { ok: false, why: e.message };
  } finally {
    await c.end().catch(() => {});
  }
}
