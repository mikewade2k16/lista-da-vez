// Resolve path params ({id}) com valores reais obtidos de GETs de listagem.
// Escopo estrito de leitura: só chama GET.
const camel = (s) => s.replace(/[_-](\w)/g, (_, c) => c.toUpperCase());
const PLACEHOLDER = "obs-probe-0000";

export function firstItem(json) {
  const body = json?.data ?? json;
  if (Array.isArray(body))
    return body.find((x) => x && typeof x === "object") ?? null;
  if (body && typeof body === "object") {
    for (const v of Object.values(body)) {
      if (Array.isArray(v)) {
        const it = v.find((x) => x && typeof x === "object");
        if (it) return it;
      }
    }
    for (const v of Object.values(body)) {
      if (v && typeof v === "object" && !Array.isArray(v)) {
        for (const w of Object.values(v))
          if (Array.isArray(w)) {
            const it = w.find((x) => x && typeof x === "object");
            if (it) return it;
          }
      }
    }
  }
  return null;
}

export function createResolver({ client, endpoints, hints = {} }) {
  const getsNoMethodKey = new Map();
  for (const e of endpoints)
    if (e.method === "GET") getsNoMethodKey.set(e.path, e);
  const cache = new Map(); // `${profile}|${listPath}` -> Promise<item|null>
  const listCalls = [];

  async function fetchItem(listPath, session) {
    const key = `${session?.profile || "-"}|${listPath}`;
    if (!cache.has(key)) {
      cache.set(
        key,
        (async () => {
          const r = await resolvePath(listPath, session);
          if (r.missing) return null;
          const res = await client.request("GET", r.path, {
            token: session?.token,
            accountId: session?.accountId,
          });
          listCalls.push({ path: r.path, status: res.status });
          return res.status >= 200 && res.status < 300
            ? firstItem(res.json)
            : null;
        })(),
      );
    }
    return cache.get(key);
  }

  function pickValue(item, param) {
    for (const k of [
      param,
      camel(param),
      "id",
      `${param.replace(/Id$/, "")}Id`,
    ]) {
      const v = item?.[k];
      if (typeof v === "string" || typeof v === "number") return String(v);
    }
    return null;
  }

  async function resolveParam(template, i, param, session) {
    if (hints[param] !== undefined) return String(hints[param]);
    if (/^(accountId|account_id)$/i.test(param) && session?.accountId)
      return session.accountId;
    const segs = template.split("/");
    const prefix = segs.slice(0, i).join("/");
    const direct = getsNoMethodKey.get(prefix);
    const candidates = [];
    if (direct) candidates.push(direct.path);
    const base = param.replace(/(Id|_id)$/, "").toLowerCase();
    for (const e of endpoints) {
      if (
        e.method !== "GET" ||
        e.path === template ||
        candidates.includes(e.path)
      )
        continue;
      const last = e.path.split("/").pop().toLowerCase();
      if (
        !/^\{/.test(last) &&
        [base, `${base}s`, `${base}es`].includes(last) &&
        !e.path.includes(`{${param}}`)
      )
        candidates.push(e.path);
    }
    for (const c of candidates.slice(0, 3)) {
      const item = await fetchItem(c, session);
      const v =
        item && pickValue(item, direct && c === direct.path ? param : "id");
      if (v) return v;
    }
    return null;
  }

  async function resolvePath(template, session) {
    const segs = template.split("/");
    for (let i = 0; i < segs.length; i++) {
      const m = segs[i].match(/^\{(.+)\}$/);
      if (!m) continue;
      const v = await resolveParam(template, i, m[1], session);
      if (v === null) return { missing: m[1] };
      segs[i] = encodeURIComponent(v);
    }
    return { path: segs.join("/") };
  }

  // Path para sondas sem dado real (negativos de autorização).
  const probePath = (template) => template.replace(/\{[^}]+\}/g, PLACEHOLDER);

  return {
    resolvePath,
    probePath,
    listCalls,
    resolveParamValue: async (name, session, tpl) =>
      await resolveParam(
        tpl,
        tpl.split("/").findIndex((s) => s === `{${name}}`),
        name,
        session,
      ),
  };
}
