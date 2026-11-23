// Fase API: smoke + contrato + autorização por endpoint do inventário.
import { config } from "../config.mjs";
import { WRITE_METHODS } from "./safety.mjs";
import { createResolver } from "./params.mjs";
import { mapPool, shapeOf, shapeDiff, stats, round } from "./util.mjs";

const result = (kind, ep, status, reason, metrics = {}) => ({
  kind,
  module: ep.module || "unknown",
  target: ep.id,
  status,
  reason,
  metrics,
});

export function p95Limit(ep) {
  const l = config.limits;
  return l.p95MsByEndpoint[ep.id] ?? l.p95MsByModule[ep.module] ?? l.p95Ms;
}

export function neverCallReason(ep) {
  if (ep.method !== "GET") return null;
  for (const n of config.neverCall)
    if (n.re.test(ep.path)) return `nunca chamado: ${n.reason}`;
  if (config.skipExternalDeps && ep.deps?.external?.length)
    return `nunca chamado: dependência externa (${ep.deps.external.join(", ")})`;
  return null;
}

export function selectEndpoints(inv, args) {
  let eps = inv.endpoints;
  if (args.modules.length)
    eps = eps.filter((e) => args.modules.includes(e.module));
  if (args.endpoint)
    eps = eps.filter(
      (e) => e.id.includes(args.endpoint) || e.path.includes(args.endpoint),
    );
  return eps;
}

export async function runApi(ctx) {
  const { inv, args, client, sessions, writePolicy, baseline, log } = ctx;
  const targets = selectEndpoints(inv, args);
  const resolver = createResolver({
    client,
    endpoints: inv.endpoints,
    hints: config.paramHints,
  });
  ctx.resolver = resolver;
  const session = sessions[args.profile];
  const baseMap = baseline?.byTarget || new Map();
  const results = [];
  const push = (r) => results.push(r);
  let done = 0;

  async function negatives(ep) {
    if (ep.auth === "public") return;
    if (!["auth", "auth_account", "role"].includes(ep.auth)) {
      push(
        result(
          "authz",
          ep,
          "skip",
          `auth "${ep.auth}" desconhecido no inventário: sem expectativa de 401`,
        ),
      );
      return;
    }
    const probe = resolver.probePath(ep.path);
    const res = await client.request(ep.method, probe, {
      body: WRITE_METHODS.has(ep.method) ? {} : undefined,
      keepBody: false,
    });
    const m = { status: res.status, ms: round(res.ms) };
    // Qualquer 4xx sem token é rejeição (ex.: 400 missing_account_id do guard de
    // módulo, que roda antes da auth). Redirect para login (forward-auth) também.
    // Falha de autorização real = 2xx/5xx ou redirect para outro lugar.
    const location = res.location || "";
    const rejected =
      res.status === 401 ||
      (res.status >= 400 && res.status < 500 && res.status !== 403) ||
      (res.status >= 300 && res.status < 400 && /login|auth/i.test(location));
    if (rejected)
      push(
        result(
          "authz",
          { ...ep, id: `${ep.id} [sem token]` },
          "pass",
          res.status === 401 ? "" : `rejeitado com ${res.status} (não 401)`,
          m,
        ),
      );
    else if (res.status === 403)
      push(
        result(
          "authz",
          { ...ep, id: `${ep.id} [sem token]` },
          "warn",
          "sem token retornou 403 (esperado 401)",
          m,
        ),
      );
    else if (res.status === 0)
      push(
        result(
          "authz",
          { ...ep, id: `${ep.id} [sem token]` },
          "warn",
          `sem resposta: ${res.error}`,
          m,
        ),
      );
    else
      push(
        result(
          "authz",
          { ...ep, id: `${ep.id} [sem token]` },
          "fail",
          `AUTHZ: sem token retornou ${res.status} (esperado 401)`,
          { ...m, authz: true },
        ),
      );

    // Perfil sem permissão: só leitura (ou escrita quando liberada).
    if (
      ep.auth === "role" &&
      ep.roles?.length &&
      (ep.method === "GET" || writePolicy.allowed)
    ) {
      const other = Object.values(sessions).find(
        (s) =>
          s.token && s.role && !ep.roles.map(String).includes(String(s.role)),
      );
      if (!other) return;
      const p = await resolver.resolvePath(ep.path, other);
      const res2 = await client.request(ep.method, p.path || probe, {
        token: other.token,
        accountId: other.accountId,
        body: ep.method === "GET" ? undefined : {},
        keepBody: false,
      });
      const tgt = { ...ep, id: `${ep.id} [perfil ${other.profile}]` };
      const m2 = {
        status: res2.status,
        ms: round(res2.ms),
        profile: other.profile,
        role: other.role,
      };
      if (res2.status === 403 || res2.status === 404)
        push(result("authz", tgt, "pass", "", m2));
      else
        push(
          result(
            "authz",
            tgt,
            "fail",
            `AUTHZ: ${other.profile} (${other.role}) recebeu ${res2.status} em rota restrita a ${ep.roles.join("/")}`,
            { ...m2, authz: true },
          ),
        );
    }
  }

  // GET que responde 400 sem filtros: tenta uma vez com os parâmetros de query
  // declarados no inventário que a sessão sabe preencher (tenant/loja/período).
  async function probeQuery(ep, path, sess) {
    const defaults = sess?.queryDefaults;
    if (ep.method !== "GET" || !defaults || !ep.queryParams?.length) return "";
    const params = ep.queryParams.filter((q) => defaults[q] !== undefined);
    if (!params.length) return "";
    const plain = await client.request("GET", path, {
      token: sess.token,
      accountId: sess.accountId,
      keepBody: false,
    });
    if (plain.status !== 400) return "";
    return new URLSearchParams(
      params.map((q) => [q, String(defaults[q])]),
    ).toString();
  }

  async function main(ep) {
    const base = { module: ep.module };
    if (WRITE_METHODS.has(ep.method) && !writePolicy.allowed)
      return push(result("api", ep, "skip", writePolicy.reason));
    const nc = neverCallReason(ep);
    if (nc) return push(result("api", ep, "skip", nc));
    let sess = null;
    if (ep.auth !== "public") {
      sess = session?.token ? session : null;
      if (!sess)
        return push(
          result(
            "api",
            ep,
            "skip",
            session?.error ||
              `sem credencial do perfil ${args.profile} (defina OBS_USER_${args.profile}_EMAIL)`,
          ),
        );
    }
    const r = await resolver.resolvePath(ep.path, sess);
    if (r.missing)
      return push(result("api", ep, "skip", `sem valor para {${r.missing}}`));
    const query = await probeQuery(ep, r.path, sess);
    if (query) r.path += `?${query}`;
    const n = ep.method === "GET" ? args.repeat : 1;
    const times = [],
      sizes = [],
      reqIds = [];
    let last = null,
      firstOk = null;
    for (let i = 0; i < n; i++) {
      const res = await client.request(ep.method, r.path, {
        token: sess?.token,
        accountId: sess?.accountId,
        body: ep.method === "GET" ? undefined : {},
      });
      times.push(res.ms);
      sizes.push(res.bytes);
      reqIds.push(res.requestId);
      last = res;
      if (
        !firstOk &&
        res.status >= 200 &&
        res.status < 300 &&
        res.json !== undefined
      )
        firstOk = res;
      if (res.status === 0 || res.status >= 500) break; // não insiste em endpoint quebrado
    }
    const st = stats(times);
    const limit = p95Limit(ep);
    const m = {
      ...base,
      status: last.status,
      ...st,
      bytes: sizes.sort((a, b) => a - b)[Math.floor(sizes.length / 2)],
      limitMs: limit,
      url: r.path,
      requestIds: reqIds.slice(0, 5),
      profile: sess?.profile || null,
    };
    if (firstOk) m.shape = shapeOf(firstOk.json);
    let status = "pass",
      reason = "";
    if (last.status === 0) {
      status = "fail";
      reason = `sem resposta: ${last.error}`;
    } else if (last.status >= 500) {
      status = "fail";
      reason = `HTTP ${last.status}`;
    } else if (last.status >= 400) {
      status = "warn";
      reason = `HTTP ${last.status} com dados reais/perfil ${args.profile}`;
    } else if (ep.method === "GET" && st.p95 > limit) {
      status = "warn";
      reason = `p95 ${st.p95}ms acima do limite ${limit}ms`;
    }
    const old = baseMap.get(`api|${ep.id}`);
    const diff =
      old?.metrics?.shape && m.shape
        ? shapeDiff(old.metrics.shape, m.shape)
        : null;
    if (diff) {
      m.shapeDiff = diff;
      if (status === "pass") status = "warn";
      const parts = [
        diff.added.length && `+${diff.added.slice(0, 4).join(",")}`,
        diff.removed.length && `-${diff.removed.slice(0, 4).join(",")}`,
        diff.changed.length && `~${diff.changed.slice(0, 3).join(",")}`,
      ].filter(Boolean);
      reason = [reason, `contrato mudou: ${parts.join(" ")}`]
        .filter(Boolean)
        .join("; ");
    }
    push(result("api", ep, status, reason, m));
  }

  await mapPool(targets, config.concurrency, async (ep) => {
    try {
      await negatives(ep);
      await main(ep);
    } catch (e) {
      push(result("api", ep, "fail", `erro interno do runner: ${e.message}`));
    }
    if (++done % 25 === 0) log(`  api ${done}/${targets.length}`);
  });
  return results;
}
