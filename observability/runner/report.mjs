// report.html estático: CSS inline, sem JS/libs externas.
import { esc } from "./util.mjs";

const CSS = `body{font:14px system-ui,sans-serif;margin:24px;color:#1b1f24;background:#fafafa}h1{font-size:20px}h2{font-size:16px;margin-top:28px;border-bottom:1px solid #ddd;padding-bottom:4px}
table{border-collapse:collapse;width:100%;background:#fff;font-size:12.5px}th,td{border:1px solid #e2e2e2;padding:4px 8px;text-align:left;vertical-align:top}th{background:#f0f2f5}
.pass{color:#0a7d33}.fail{color:#c0152a;font-weight:600}.warn{color:#a15c00}.skip{color:#777}.kv span{display:inline-block;margin-right:18px}
code{font-size:12px}.bar{display:inline-block;height:8px;background:#2f7de1;vertical-align:middle}.muted{color:#777}details{margin:6px 0}`;

const table = (head, rows, empty = "nada a mostrar") =>
  rows.length
    ? `<table><thead><tr>${head.map((h) => `<th>${esc(h)}</th>`).join("")}</tr></thead><tbody>${rows.map((r) => `<tr>${r.map((c) => `<td>${c}</td>`).join("")}</tr>`).join("")}</tbody></table>`
    : `<p class="muted">${empty}</p>`;
const st = (s) => `<span class="${s}">${s}</span>`;
const code = (s) => `<code>${esc(s)}</code>`;

export function renderReport(run) {
  const { results: rs, coverage: cov, scores } = run;
  const s = run.summary;
  const html = [];
  html.push(
    `<h1>Observabilidade — run ${esc(run.runId)}</h1><p class="kv"><span>env: <b>${esc(run.env)}</b></span><span>API: ${esc(run.baseUrl)}</span><span>git: ${esc(run.git?.branch)}@${esc(String(run.git?.sha || "").slice(0, 8))}</span><span>filtros: ${code(JSON.stringify(run.filters))}</span><span>exit: <b>${run.exitCode}</b></span></p>`,
  );
  html.push(
    `<h2>Resumo</h2>` +
      table(
        ["pass", "warn", "fail", "skip", "regressões", "melhorias", "baseline"],
        [
          [
            s.pass,
            s.warn,
            s.fail,
            s.skip,
            run.regressions.length,
            run.improvements.length,
            esc(run.baselineId || "—"),
          ],
        ],
      ),
  );
  const sc = Object.entries(scores).sort(
    (a, b) => (a[1].score ?? 101) - (b[1].score ?? 101),
  );
  html.push(
    `<h2>Notas por módulo</h2>` +
      table(
        ["módulo", "nota", "erros", "latência", "vitals", "banco", "cobertura"],
        sc.map(([m, v]) => [
          esc(m),
          v.score == null
            ? `<span class="muted">sem dados</span>`
            : `<b>${v.score}</b> <span class="bar" style="width:${v.score}px"></span>`,
          ...["errors", "latency", "vitals", "db", "coverage"].map((k) =>
            v.components?.[k] != null ? `-${v.components[k]}` : "",
          ),
        ]),
        "sem módulos",
      ),
  );
  const reg = (list, empty) =>
    table(
      ["alvo", "métrica", "antes", "depois", "mensagem"],
      list.map((r) => [
        code(r.target),
        esc(r.metric),
        esc(r.before),
        esc(r.after),
        esc(r.message),
      ]),
      empty,
    );
  html.push(`<h2>Regressões</h2>` + reg(run.regressions, "nenhuma regressão"));
  html.push(`<h2>Melhorias</h2>` + reg(run.improvements, "nenhuma melhoria"));
  const api = rs
    .filter((r) => r.kind === "api" && r.metrics?.p95 != null)
    .sort((a, b) => b.metrics.p95 - a.metrics.p95)
    .slice(0, 25);
  html.push(
    `<h2>Endpoints mais lentos (p95)</h2>` +
      table(
        [
          "endpoint",
          "módulo",
          "p50",
          "p95",
          "p99",
          "bytes",
          "queries/req",
          "status",
        ],
        api.map((r) => [
          code(r.target),
          esc(r.module),
          r.metrics.p50,
          `<b>${r.metrics.p95}</b>`,
          r.metrics.p99,
          r.metrics.bytes,
          r.metrics.queriesPerReq ?? "",
          st(r.status),
        ]),
        "nenhum endpoint medido",
      ),
  );
  const fails = rs.filter((r) => r.status === "fail" && r.kind !== "authz");
  html.push(
    `<h2>Falhas</h2>` +
      table(
        ["tipo", "alvo", "módulo", "motivo"],
        fails.map((r) => [
          r.kind,
          code(r.target),
          esc(r.module),
          esc(r.reason),
        ]),
        "nenhuma falha",
      ),
  );
  const az = rs.filter((r) => r.kind === "authz" && r.status !== "pass");
  const azPass = rs.filter(
    (r) => r.kind === "authz" && r.status === "pass",
  ).length;
  html.push(
    `<h2>Autorização</h2><p class="muted">${azPass} verificações passaram.</p>` +
      table(
        ["alvo", "resultado", "motivo"],
        az.map((r) => [code(r.target), st(r.status), esc(r.reason)]),
        "sem falhas de autorização",
      ),
  );
  const pages = rs.filter((r) => r.kind === "page");
  html.push(
    `<h2>Páginas e Web Vitals</h2>` +
      table(
        [
          "página",
          "status",
          "carga ms",
          "LCP",
          "INP",
          "CLS",
          "TTFB",
          "erros console/JS",
          "req. falhas",
          "motivo",
        ],
        pages.map((r) => [
          code(r.target),
          st(r.status),
          r.metrics.loadMs ?? "",
          r.metrics.lcp ?? "",
          r.metrics.inp ?? "",
          r.metrics.cls ?? "",
          r.metrics.ttfb ?? "",
          r.metrics.consoleErrors
            ? `${r.metrics.consoleErrors.length}/${r.metrics.pageErrors.length}`
            : "",
          r.metrics.failedRequests?.length ?? "",
          esc(r.reason),
        ]),
        "páginas não executadas",
      ),
  );
  const d = run.db || {};
  const dbRows = rs.filter(
    (r) =>
      r.kind === "db" &&
      !/^db:(fk-sem|sem-pk|indice-dup|slow|n\+1|explain)/.test(r.target),
  );
  html.push(
    `<h2>Banco</h2>` +
      table(
        ["checagem", "status", "detalhe"],
        dbRows.map((r) => [code(r.target), st(r.status), esc(r.reason)]),
        "fase db não executada",
      ),
  );
  for (const [k, rows] of Object.entries(d.checks || {})) {
    if (rows.length)
      html.push(
        `<details><summary>${esc(k)} (${rows.length})</summary><pre>${esc(
          rows
            .slice(0, 60)
            .map((x) => JSON.stringify(x))
            .join("\n"),
        )}</pre></details>`,
      );
  }
  if (d.bigTables?.length) {
    html.push(
      `<details><summary>Maiores tabelas</summary>${table(
        ["tabela", "MB", "linhas~"],
        d.bigTables.map((t) => [
          code(`${t.schema}.${t.table}`),
          Math.round(Number(t.bytes) / 1048576),
          t.rows,
        ]),
      )}</details>`,
    );
  }
  for (const e of d.explains || []) {
    html.push(
      `<details><summary>EXPLAIN ${esc(e.fingerprint)}${e.seqScans.length ? " (Seq Scan: " + esc(e.seqScans.join(",")) + ")" : ""}</summary><pre>${esc(e.statement + "\n\n" + e.plan.join("\n"))}</pre></details>`,
    );
  }
  const load = rs.filter((r) => r.kind === "load");
  if (load.length) {
    html.push(
      `<h2>Carga</h2>` +
        table(
          [
            "endpoint",
            "status",
            "rps",
            "p50",
            "p99",
            "p50 isolado",
            "erros",
            "não-2xx",
            "motivo",
          ],
          load.map((r) => [
            code(r.target),
            st(r.status),
            r.metrics.rps ?? "",
            r.metrics.p50 ?? "",
            r.metrics.p99 ?? "",
            r.metrics.isolatedP50 ?? "",
            r.metrics.errors ?? "",
            r.metrics.non2xx ?? "",
            esc(r.reason),
          ]),
        ),
    );
  }
  const health = rs.filter((r) => r.kind === "health");
  if (health.length)
    html.push(
      `<h2>Saúde</h2>` +
        table(
          ["alvo", "status", "motivo"],
          health.map((r) => [code(r.target), st(r.status), esc(r.reason)]),
        ),
    );
  html.push(
    `<h2>Cobertura</h2>` +
      table(
        ["tipo", "testados", "total", "%"],
        ["endpoints", "pages", "tables"].map((k) => [
          k,
          cov[k].tested,
          cov[k].total,
          cov[k].pct,
        ]),
      ) +
      `<p class="muted">Tabelas citadas em query capturada: ${cov.tables.queried} (${cov.tables.queriedPct}%). Critério: veja runner/coverage.mjs.</p>`,
  );
  for (const k of ["endpoints", "pages", "tables"]) {
    const u = cov[k].untested;
    html.push(
      `<details><summary>Não testados — ${k} (${u.length})</summary>${table(
        ["item", "módulo", "motivo"],
        u.slice(0, 600).map((x) => [code(x.key), esc(x.module), esc(x.reason)]),
      )}${u.length > 600 ? `<p class="muted">… e mais ${u.length - 600} (veja results.json)</p>` : ""}</details>`,
    );
  }
  return `<!doctype html><html lang="pt-BR"><meta charset="utf-8"><title>Observabilidade ${esc(run.runId)}</title><style>${CSS}</style><body>${html.join("\n")}</body></html>`;
}
