// Fase carga (--load): autocannon em até 5 GETs principais, só leitura.
import { config } from "../config.mjs";
import { round } from "./util.mjs";
import { connectReadOnly } from "./db.mjs";

async function topRoutes() {
  let c;
  try {
    c = await connectReadOnly();
    if (
      !(await c.query(`select to_regclass('obs.http_route_hourly') t`)).rows[0]
        .t
    )
      return [];
    return (
      await c.query(
        `select route, sum(calls)::bigint as calls from obs.http_route_hourly where route like 'GET %' group by route order by calls desc limit 50`,
      )
    ).rows.map((r) => r.route);
  } catch {
    return [];
  } finally {
    await c?.end().catch(() => {});
  }
}

export async function runLoad(ctx) {
  const { api, sessions, args } = ctx;
  const mk = (target, module, status, reason, metrics = {}) => ({
    kind: "load",
    module,
    target,
    status,
    reason,
    metrics,
  });
  let autocannon;
  try {
    autocannon = (await import("autocannon")).default;
  } catch {
    return [
      mk(
        "load:autocannon",
        "database",
        "skip",
        "autocannon não instalado (optionalDependency)",
      ),
    ];
  }
  const ok = api.filter(
    (r) =>
      r.kind === "api" &&
      r.status !== "skip" &&
      r.status !== "fail" &&
      r.metrics?.url &&
      r.target.startsWith("GET ") &&
      r.metrics.status >= 200 &&
      r.metrics.status < 300,
  );
  const popular = await topRoutes();
  const rank = (r) => {
    const i = popular.indexOf(r.target);
    return i < 0 ? 999 : i;
  };
  const chosen = [...ok]
    .sort((a, b) => rank(a) - rank(b) || a.metrics.p50 - b.metrics.p50)
    .slice(0, config.load.maxEndpoints);
  if (!chosen.length)
    return [
      mk(
        "load:endpoints",
        "database",
        "skip",
        "nenhum GET 2xx medido na fase API para usar na carga",
      ),
    ];
  const sess = sessions[args.profile];
  const out = [];
  for (const r of chosen) {
    const headers = {
      "X-Obs-Source": "runner",
      "X-Request-ID": `obsrun-${ctx.runId}-load`,
    };
    if (sess?.token) headers.Authorization = `Bearer ${sess.token}`;
    if (sess?.accountId) headers["X-Account-Id"] = sess.accountId;
    const res = await autocannon({
      url: config.apiUrl + r.metrics.url,
      connections: config.load.connections,
      duration: config.load.durationSec,
      headers,
      overallRate: config.load.overallRate || undefined,
    });
    const m = {
      rps: round(res.requests.average),
      p50: res.latency.p50,
      p99: res.latency.p99,
      avg: round(res.latency.average),
      errors: res.errors,
      timeouts: res.timeouts,
      non2xx: res.non2xx,
      total: res.requests.total,
      isolatedP50: r.metrics.p50,
      isolatedP95: r.metrics.p95,
      rateCapRps: config.load.overallRate || null,
      connections: config.load.connections,
      durationSec: config.load.durationSec,
    };
    let status = "pass",
      reason = "";
    if (res.errors || res.timeouts) {
      status = "fail";
      reason = `${res.errors} erros / ${res.timeouts} timeouts`;
    } else if (res.non2xx) {
      status = "warn";
      reason = `${res.non2xx} respostas não-2xx (429 = rate limit)`;
    } else if (
      res.latency.p99 > Math.max(r.metrics.limitMs * 2, r.metrics.p95 * 4)
    ) {
      status = "warn";
      reason = `p99 ${res.latency.p99}ms sob carga vs p95 isolado ${r.metrics.p95}ms`;
    }
    out.push(mk(r.target, r.module, status, reason, m));
  }
  return out;
}
