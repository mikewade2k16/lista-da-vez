// Fase security: checagens automáticas, não destrutivas, do catálogo
// (config/security-catalog.json). Em produção só as passivas (headers, CORS,
// cookies, TLS, repositório, dependências). As de força bruta rodam por último
// e só fora de produção, para o 429 do próprio runner não atrapalhar as demais.
import { randomUUID, randomBytes } from "node:crypto";
import { readFileSync } from "node:fs";
import http from "node:http";
import https from "node:https";
import tls from "node:tls";
import { join } from "node:path";
import { config, credentialsFor, REPO } from "../config.mjs";
import { looksLikeProduction } from "./safety.mjs";
import { neverCallReason } from "./api.mjs";
import { rateLimiter, sh, round } from "./util.mjs";
import {
  TO_RESULT,
  evalHeaders,
  tamperSignature,
  tamperPayload,
  algNone,
  corsVerdict,
  redirectsExternal,
  isDirectoryListing,
  leaksPasswd,
  unescapedReflection,
  isIgnoredPath,
  scanText,
} from "./security-lib.mjs";

const EVIL = "https://evil.example";
const XSS = () => `obsxss${randomBytes(4).toString("hex")}`;
const o = (key, status, detail, evidence = {}) => ({
  key,
  status,
  detail,
  evidence,
});

// Distribui n itens ao longo da lista (diversidade de módulos).
export function spread(list, n) {
  if (list.length <= n) return list;
  const step = list.length / n;
  return Array.from({ length: n }, (_, i) => list[Math.floor(i * step)]);
}

function makeEnv(ctx) {
  const throttle = rateLimiter(config.rateLimitPerMin);
  let seq = 0;
  const rid = () => `obssec-${ctx.runId}-${++seq}`;
  async function probe(
    method,
    url,
    { headers = {}, body, timeoutMs = config.timeouts.requestMs } = {},
  ) {
    await throttle();
    const t0 = performance.now();
    try {
      const res = await fetch(url, {
        method,
        headers: {
          "X-Request-ID": rid(),
          "X-Obs-Source": "runner",
          ...headers,
        },
        body,
        signal: AbortSignal.timeout(timeoutMs),
        redirect: "manual",
      });
      const buf = Buffer.from(await res.arrayBuffer());
      return {
        status: res.status,
        ms: performance.now() - t0,
        headers: res.headers,
        h: (n) => res.headers.get(n) || "",
        text: buf.subarray(0, 262144).toString("utf8"),
        contentType: res.headers.get("content-type") || "",
        location: res.headers.get("location") || "",
        setCookies: res.headers.getSetCookie?.() ?? [],
      };
    } catch (e) {
      return {
        status: 0,
        ms: performance.now() - t0,
        headers: new Headers(),
        h: () => "",
        text: "",
        contentType: "",
        location: "",
        setCookies: [],
        error: String(e?.cause?.code || e?.name || e?.message || e),
      };
    }
  }
  // GET com caminho literal (sem normalizar ../ nem %2e%2e como o fetch faz).
  async function rawGet(base, rawPath) {
    await throttle();
    const u = new URL(base);
    const lib = u.protocol === "https:" ? https : http;
    return new Promise((resolve) => {
      const req = lib.request(
        {
          host: u.hostname,
          port: u.port || (u.protocol === "https:" ? 443 : 80),
          path: rawPath,
          method: "GET",
          headers: { "X-Request-ID": rid(), "X-Obs-Source": "runner" },
          timeout: config.timeouts.requestMs,
        },
        (res) => {
          const chunks = [];
          let n = 0;
          res.on("data", (d) => {
            if (n < 65536) chunks.push(d);
            n += d.length;
          });
          res.on("end", () =>
            resolve({
              status: res.statusCode,
              contentType: res.headers["content-type"] || "",
              text: Buffer.concat(chunks).toString("utf8").slice(0, 65536),
            }),
          );
        },
      );
      req.on("timeout", () => req.destroy(new Error("timeout")));
      req.on("error", (e) =>
        resolve({
          status: 0,
          contentType: "",
          text: "",
          error: e.code || e.message,
        }),
      );
      req.end();
    });
  }
  return { probe, rawGet };
}

const apiUrl = () => config.apiUrl;
const isHttps = (u) => u.startsWith("https:");
const reqEndpoints = (ctx, pred) =>
  ctx.inv.endpoints.filter(
    (e) => e.method === "GET" && !neverCallReason(e) && pred(e),
  );

// ---------------------------------------------------------------- headers
async function checkHeaders(ctx, E) {
  const out = [];
  const grade = (key, label, r, https) => {
    if (!r.status)
      return o(key, "not_tested", `${label} inacessível: ${r.error}`);
    const { missing } = evalHeaders(r.headers, { https });
    const st = !missing.length
      ? "pass"
      : missing.length <= 2
        ? "partial"
        : "fail";
    return o(
      key,
      st,
      missing.length
        ? `${label}: faltam ${missing.join(", ")}`
        : `${label}: todos os headers verificados presentes`,
      { missing, https },
    );
  };
  const a = await E.probe("GET", `${apiUrl()}/healthz`);
  const w = await E.probe("GET", config.webUrl + "/");
  out.push(grade("headers-api", "API", a, isHttps(apiUrl())));
  const hw = grade("headers-web", "Web", w, isHttps(config.webUrl));
  if (hw.status === "fail" && !isHttps(config.webUrl))
    hw.detail += " (ambiente local sem proxy; confirme em produção)";
  out.push(hw);
  const fa = a.status ? evalHeaders(a.headers).frame : null;
  const fw = w.status ? evalHeaders(w.headers).frame : null;
  if (fa === null && fw === null)
    out.push(o("clickjacking", "not_tested", "API e web inacessíveis"));
  else {
    const ok = [fa, fw].filter((x) => x === true).length;
    const known = [fa, fw].filter((x) => x !== null).length;
    out.push(
      o(
        "clickjacking",
        ok === known ? "pass" : ok ? "partial" : "fail",
        `frame-ancestors/X-Frame-Options: API ${fa === null ? "n/d" : fa ? "ok" : "ausente"}, web ${fw === null ? "n/d" : fw ? "ok" : "ausente"}`,
        { api: fa, web: fw },
      ),
    );
  }
  return out;
}

// ------------------------------------------------------------------- CORS
async function checkCors(ctx, E) {
  const tests = [];
  for (const origin of [EVIL, "null"]) {
    const g = await E.probe("GET", `${apiUrl()}/healthz`, {
      headers: { Origin: origin },
    });
    const p = await E.probe("OPTIONS", `${apiUrl()}/v1/auth/login`, {
      headers: {
        Origin: origin,
        "Access-Control-Request-Method": "POST",
        "Access-Control-Request-Headers": "authorization,content-type",
      },
    });
    for (const [kind, r] of [
      ["GET", g],
      ["preflight", p],
    ])
      if (r.status) tests.push({ origin, kind, ...corsVerdict(origin, r.h) });
  }
  if (!tests.length)
    return [o("cors-reflection", "not_tested", "API inacessível")];
  const bad = tests.filter((t) => !t.ok);
  return [
    o(
      "cors-reflection",
      bad.length ? "fail" : "pass",
      bad.length
        ? `API aceita origem não autorizada: ${bad[0].why}`
        : `Origin ${EVIL} e null não são refletidos (${tests.length} sondagens)`,
      {
        tests: tests.map((t) => ({ origin: t.origin, kind: t.kind, ok: t.ok })),
      },
    ),
  ];
}

// ---------------------------------------------------------- cookie de sessão
function cookieFlags(sc) {
  const parts = sc.split(";").map((s) => s.trim());
  const name = parts[0].split("=")[0];
  const has = (re) => parts.slice(1).some((p) => re.test(p));
  return {
    name,
    httpOnly: has(/^httponly$/i),
    secure: has(/^secure$/i),
    sameSite:
      (parts.find((p) => /^samesite=/i.test(p)) || "").split("=")[1] || "",
  };
}
async function checkCookie(ctx, E) {
  const cred = credentialsFor(ctx.args.profile);
  if (!cred)
    return [
      o("session-cookie", "not_tested", "sem credencial do perfil para logar"),
    ];
  const r = await E.probe("POST", `${apiUrl()}/v1/auth/login`, {
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email: cred.email, password: cred.password }),
  });
  if (r.status !== 200)
    return [
      o("session-cookie", "not_tested", `login falhou (HTTP ${r.status})`),
    ];
  const cookies = r.setCookies.map(cookieFlags);
  // O front guarda o token da sessão via useCookie (sempre legível por JS).
  let front = null;
  try {
    const src = readFileSync(join(REPO, "web/app/stores/auth.ts"), "utf8");
    const m = src.match(/useCookie\(\s*AUTH_TOKEN_COOKIE\s*,\s*\{([^}]*)\}/);
    if (m)
      front = {
        secure: /secure\s*:\s*true/.test(m[1]),
        sameSite: (m[1].match(/sameSite\s*:\s*['"](\w+)['"]/) || [])[1] || "",
      };
  } catch {
    /* arquivo ausente */
  }
  const weak = cookies.filter(
    (c) => !c.httpOnly || !c.sameSite || (isHttps(apiUrl()) && !c.secure),
  );
  if (front)
    return [
      o(
        "session-cookie",
        "fail",
        `token de sessão fica em cookie legível por JS (useCookie no front): sem HttpOnly${front.secure ? "" : " e sem Secure"}; um XSS rouba a sessão (achados M-08 e C-01)${cookies.length ? `. Cookies emitidos pela API: ${weak.length ? "com flags faltando" : "ok"}` : ""}`,
        { apiCookies: cookies, frontCookie: front },
      ),
    ];
  if (cookies.length)
    return [
      o(
        "session-cookie",
        weak.length ? "fail" : "pass",
        weak.length
          ? `cookie(s) ${weak.map((c) => c.name).join(", ")} sem HttpOnly/SameSite/Secure`
          : "cookies da API com HttpOnly, SameSite e Secure",
        { cookies },
      ),
    ];
  return [
    o(
      "session-cookie",
      "not_tested",
      "login sem Set-Cookie e não foi possível localizar onde o front guarda o token",
    ),
  ];
}

// -------------------------------------------------------------------- JWT
async function checkJwt(ctx, E) {
  const sess = ctx.sessions[ctx.args.profile];
  if (!sess?.token) {
    const why = "sem sessão do perfil para obter um token válido";
    return [
      o("jwt-tampered", "not_tested", why),
      o("jwt-alg-none", "not_tested", why),
    ];
  }
  const path = "/v1/auth/me";
  const ask = (t) =>
    E.probe("GET", apiUrl() + path, {
      headers: { Authorization: `Bearer ${t}` },
    });
  const base = await ask(sess.token);
  if (base.status !== 200) {
    const why = `token válido não foi aceito em ${path} (HTTP ${base.status})`;
    return [
      o("jwt-tampered", "not_tested", why),
      o("jwt-alg-none", "not_tested", why),
    ];
  }
  const rejected = (s) => s === 401 || s === 403;
  const sig = tamperSignature(sess.token);
  const pay = tamperPayload(sess.token);
  const none = algNone(sess.token);
  if (!sig || !none)
    return [
      o(
        "jwt-tampered",
        "not_tested",
        "token da sessão não é um JWT de 3 partes",
      ),
      o(
        "jwt-alg-none",
        "not_tested",
        "token da sessão não é um JWT de 3 partes",
      ),
    ];
  const rs = (await ask(sig)).status;
  const rp = pay ? (await ask(pay)).status : null;
  const rn = (await ask(none)).status;
  const tampered = rejected(rs) && (rp === null || rejected(rp));
  return [
    o(
      "jwt-tampered",
      tampered ? "pass" : "fail",
      tampered
        ? "assinatura e payload adulterados rejeitados"
        : `token adulterado aceito (assinatura HTTP ${rs}, payload HTTP ${rp})`,
      { signature: rs, payload: rp },
    ),
    o(
      "jwt-alg-none",
      rejected(rn) ? "pass" : "fail",
      rejected(rn)
        ? "token alg none rejeitado"
        : `token alg none aceito (HTTP ${rn})`,
      { status: rn },
    ),
  ];
}

// -------------------------------------------------- autorização sem token
function checkAuthzNoToken(ctx) {
  const az = ctx.results.filter((r) => r.kind === "authz");
  if (!az.length)
    return [
      o(
        "authz-no-token",
        "not_tested",
        "fase api não rodou neste run (use --only=api,security)",
      ),
    ];
  const n = (s) => az.filter((r) => r.status === s).length;
  const ev = {
    pass: n("pass"),
    fail: n("fail"),
    skip: n("skip"),
    total: az.length,
  };
  const st = ev.fail ? "fail" : ev.skip ? "partial" : "pass";
  return [
    o(
      "authz-no-token",
      st,
      ev.fail
        ? `${ev.fail} rota(s) protegida(s) responderam sem token`
        : `${ev.pass} rotas rejeitaram a chamada sem token${ev.skip ? `; ${ev.skip} não avaliadas` : ""}`,
      ev,
    ),
  ];
}

// ----------------------------------------------------- isolamento de tenant
async function loginAs(ctx, profile) {
  const cred = credentialsFor(profile);
  if (!cred) return null;
  const r = await ctx.client.request("POST", "/v1/auth/login", {
    body: { email: cred.email, password: cred.password },
    keepBody: true,
  });
  const token = r.json?.session?.accessToken;
  if (r.status !== 200 || !token)
    return { error: `login ${profile} HTTP ${r.status}` };
  const acc = await ctx.client.request("GET", "/v2/me/accounts", { token });
  const list = acc.json?.accounts || acc.json?.data?.accounts || [];
  const accountId =
    (list.find((a) => a.isDefault || a.active) || list[0] || {}).id || null;
  return { token, accountId };
}
const firstList = (json) => {
  const arr = Array.isArray(json)
    ? json
    : Object.values(json || {}).find((v) => Array.isArray(v)) ||
      Object.values(json?.data || {}).find((v) => Array.isArray(v));
  return Array.isArray(arr) ? arr : [];
};
async function checkTenantIsolation(ctx) {
  const A = credentialsFor("TENANT_A");
  const B = credentialsFor("TENANT_B");
  if (!A || !B)
    return [
      o(
        "tenant-isolation",
        "not_tested",
        "faltam credenciais de duas contas diferentes",
      ),
    ];
  const a = await loginAs(ctx, "TENANT_A");
  const b = await loginAs(ctx, "TENANT_B");
  if (!a?.token || !b?.token)
    return [
      o(
        "tenant-isolation",
        "not_tested",
        `login das contas de teste falhou (${a?.error || b?.error})`,
      ),
    ];
  if (!a.accountId || !b.accountId || a.accountId === b.accountId)
    return [
      o(
        "tenant-isolation",
        "not_tested",
        "contas A e B não resolveram dois tenants distintos",
      ),
    ];
  const eps = reqEndpoints(
    ctx,
    (e) => e.auth === "auth_account" && !e.pathParams.length,
  );
  const leaks = [];
  let swapChecked = 0;
  for (const e of spread(eps, 25)) {
    const own = await ctx.client.request("GET", e.path, {
      token: a.token,
      accountId: a.accountId,
      keepBody: false,
    });
    if (own.status < 200 || own.status >= 300) continue; // sem baseline não prova nada
    const cross = await ctx.client.request("GET", e.path, {
      token: a.token,
      accountId: b.accountId,
      keepBody: false,
    });
    swapChecked++;
    if (cross.status >= 200 && cross.status < 300)
      leaks.push(`${e.id} (X-Account-Id)`);
  }
  // ids de B nas rotas de detalhe, lidos por A com a própria conta
  let idChecked = 0;
  for (const e of eps) {
    if (idChecked >= 10) break;
    const detail = ctx.inv.endpoints.find(
      (d) =>
        d.method === "GET" &&
        d.pathParams.length === 1 &&
        d.path.startsWith(e.path + "/{") &&
        d.path === `${e.path}/{${d.pathParams[0]}}`,
    );
    if (!detail || neverCallReason(detail)) continue;
    const list = await ctx.client.request("GET", e.path, {
      token: b.token,
      accountId: b.accountId,
    });
    const id = firstList(list.json)[0]?.id;
    if (list.status !== 200 || typeof id !== "string") continue;
    const path = detail.path.replace(/\{[^}]+\}/, encodeURIComponent(id));
    const mine = await ctx.client.request("GET", path, {
      token: b.token,
      accountId: b.accountId,
      keepBody: false,
    });
    if (mine.status !== 200) continue;
    const cross = await ctx.client.request("GET", path, {
      token: a.token,
      accountId: a.accountId,
      keepBody: false,
    });
    idChecked++;
    if (cross.status >= 200 && cross.status < 300)
      leaks.push(`${detail.id} (id de B)`);
  }
  const ev = {
    headerSwapChecked: swapChecked,
    idSwapChecked: idChecked,
    leaks,
  };
  if (leaks.length)
    return [
      o(
        "tenant-isolation",
        "fail",
        `A leu dados de B em ${leaks.length} rota(s): ${leaks.slice(0, 5).join("; ")}`,
        ev,
      ),
    ];
  if (swapChecked + idChecked < 5)
    return [
      o(
        "tenant-isolation",
        "partial",
        `só ${swapChecked + idChecked} rotas puderam ser comparadas (contas de teste com pouco dado)`,
        ev,
      ),
    ];
  return [
    o(
      "tenant-isolation",
      "pass",
      `A não leu B em ${swapChecked} troca(s) de X-Account-Id e ${idChecked} troca(s) de id (só leitura)`,
      ev,
    ),
  ];
}

// -------------------------------------------------------- enumeração/403
async function checkUserEnumeration(ctx) {
  const admin = credentialsFor("ADMIN");
  if (!admin)
    return [o("user-enumeration", "not_tested", "sem credencial ADMIN")];
  const login = (email, password) =>
    ctx.client.request("POST", "/v1/auth/login", {
      body: { email, password },
      retry429: false,
    });
  const unknown2 = await login(
    `obs-enum-${randomBytes(4).toString("hex")}@example.invalid`,
    `Wrong-${randomBytes(6).toString("hex")}!`,
  );
  const unknown = await login(
    `obs-enum-${randomBytes(4).toString("hex")}@example.invalid`,
    `Wrong-${randomBytes(6).toString("hex")}!`,
  );
  const known = await login(
    admin.email,
    `Wrong-${randomBytes(6).toString("hex")}!`,
  );
  const sig = (r) => ({
    status: r.status,
    code: r.json?.error?.code || "",
    message: r.json?.error?.message || "",
  });
  const su = sig(unknown);
  const sk = sig(known);
  const ev = {
    unknown: su,
    known: sk,
    msUnknown: round(unknown.ms),
    msKnown: round(known.ms),
  };
  if (!su.status || !sk.status)
    return [o("user-enumeration", "not_tested", "login inacessível", ev)];
  const same =
    su.status === sk.status && su.code === sk.code && su.message === sk.message;
  const avgUnknown = (unknown.ms + unknown2.ms) / 2;
  const timing = known.ms > avgUnknown * 3 && known.ms - avgUnknown > 30;
  ev.msUnknown = round(avgUnknown);
  return [
    o(
      "user-enumeration",
      !same ? "fail" : timing ? "partial" : "pass",
      !same
        ? `resposta difere: inexistente ${su.status}/${su.code}, existente ${sk.status}/${sk.code}`
        : timing
          ? `mesma resposta (HTTP ${su.status} ${su.code}), mas o tempo difere (${ev.msUnknown}ms para e-mail inexistente x ${ev.msKnown}ms para existente): enumeração por tempo possível`
          : `mesma resposta (HTTP ${su.status} ${su.code}); tempos ${ev.msUnknown}ms x ${ev.msKnown}ms`,
      ev,
    ),
  ];
}
async function checkEnum403(ctx) {
  const sess = ctx.sessions[ctx.args.profile];
  if (!sess?.token)
    return [o("enum-403-404", "not_tested", "sem sessão do perfil")];
  const ep = reqEndpoints(
    ctx,
    (e) => e.auth === "auth_account" && !e.pathParams.length,
  )[0];
  if (!ep)
    return [
      o("enum-403-404", "not_tested", "nenhuma rota escopada no inventário"),
    ];
  const r = await ctx.client.request("GET", ep.path, {
    token: sess.token,
    accountId: randomUUID(),
    keepBody: false,
  });
  const ev = { endpoint: ep.id, status: r.status };
  if (r.status === 404)
    return [
      o(
        "enum-403-404",
        "pass",
        "conta inexistente/fora do escopo responde 404",
        ev,
      ),
    ];
  if (r.status === 403)
    return [
      o(
        "enum-403-404",
        "fail",
        "conta fora do escopo responde 403, o que confirma que o UUID de conta existe (achado B-01)",
        ev,
      ),
    ];
  return [
    o(
      "enum-403-404",
      "partial",
      `resposta HTTP ${r.status} para conta aleatória (esperado 404)`,
      ev,
    ),
  ];
}

// -------------------------------------------------------- SQLi e XSS
function injectionTargets(ctx, n) {
  return spread(
    reqEndpoints(
      ctx,
      (e) =>
        e.queryParams.length &&
        !e.pathParams.length &&
        ["auth", "auth_account", "role"].includes(e.auth) &&
        !e.deps?.external?.length,
    ),
    n,
  );
}
const qs = (names, value) =>
  "?" +
  names
    .map((n) => `${encodeURIComponent(n)}=${encodeURIComponent(value)}`)
    .join("&");
async function checkSqli(ctx) {
  const sess = ctx.sessions[ctx.args.profile];
  if (!sess?.token)
    return [o("sqli-basic", "not_tested", "sem sessão do perfil")];
  const targets = injectionTargets(ctx, 20);
  const payloads = ["' OR '1'='1", "1;select pg_sleep(2)"];
  const problems = [];
  let validated = 0;
  let reached = 0;
  let baselineErrors = 0;
  for (const e of targets) {
    const opt = {
      token: sess.token,
      accountId: sess.accountId,
      keepBody: false,
    };
    const base = await ctx.client.request(
      "GET",
      e.path + qs(e.queryParams, "a"),
      opt,
    );
    for (const p of payloads) {
      const r = await ctx.client.request(
        "GET",
        e.path + qs(e.queryParams, p),
        opt,
      );
      if (base.status >= 500) baselineErrors++;
      else if (r.status >= 500 || r.status === 0)
        problems.push(`${e.id}: HTTP ${r.status}`);
      else if (r.ms - base.ms > 1500)
        problems.push(`${e.id}: +${Math.round(r.ms - base.ms)}ms`);
      else if (r.status >= 400) validated++;
      else reached++;
    }
  }
  const ev = {
    endpoints: targets.length,
    rejected4xx: validated,
    accepted2xx: reached,
    baselineErrors,
    problems,
  };
  if (!targets.length)
    return [
      o("sqli-basic", "not_tested", "nenhum GET com query no inventário"),
    ];
  return [
    o(
      "sqli-basic",
      problems.length ? "fail" : "pass",
      problems.length
        ? `${problems.length} resposta(s) suspeita(s): ${problems.slice(0, 3).join("; ")}`
        : `${targets.length} rotas com 2 payloads: sem 500 nem atraso (${reached} chegaram à consulta, ${validated} barradas por validação${baselineErrors ? `; ${baselineErrors} já davam 500 sem payload e foram ignoradas` : ""})`,
      ev,
    ),
  ];
}
async function checkXss(ctx, E) {
  const sess = ctx.sessions[ctx.args.profile];
  const m = XSS();
  const payload = `<script>${m}</script>`;
  const enc = encodeURIComponent(payload);
  const urls = [
    { url: `${apiUrl()}/${enc}`, auth: false },
    { url: `${apiUrl()}/healthz?q=${enc}`, auth: false },
    { url: `${config.webUrl}/?q=${enc}`, auth: false },
    { url: `${config.webUrl}/login?redirect=${enc}&next=${enc}`, auth: false },
    { url: `${config.webUrl}/${enc}`, auth: false },
  ];
  if (sess?.token)
    for (const e of injectionTargets(ctx, 8))
      urls.push({
        url: apiUrl() + e.path + qs(e.queryParams, payload),
        auth: true,
      });
  let checked = 0;
  const bad = [];
  for (const t of urls) {
    const headers = t.auth
      ? {
          Authorization: `Bearer ${sess.token}`,
          ...(sess.accountId ? { "X-Account-Id": sess.accountId } : {}),
        }
      : {};
    const r = await E.probe("GET", t.url, { headers });
    if (!r.status) continue;
    checked++;
    if (unescapedReflection(r.contentType, r.text, m))
      bad.push(new URL(t.url).pathname);
  }
  if (!checked)
    return [o("xss-reflected", "not_tested", "API e web inacessíveis")];
  return [
    o(
      "xss-reflected",
      bad.length ? "fail" : "pass",
      bad.length
        ? `payload voltou sem escape em HTML: ${bad.slice(0, 3).join(", ")}`
        : `${checked} respostas sem eco do payload em HTML (não cobre XSS armazenado nem baseado em DOM)`,
      { checked, reflected: bad },
    ),
  ];
}

// ----------------------------------------------------------------- uploads
async function checkUploads(ctx, E) {
  const out = [];
  const dirs = ["/uploads/", "/uploads/avatars/", "/uploads/feedback/"];
  const listed = [];
  let reached = 0;
  const statuses = {};
  for (const d of dirs) {
    const r = await E.probe("GET", apiUrl() + d);
    statuses[d] = r.status;
    if (!r.status) continue;
    reached++;
    if (isDirectoryListing(r.contentType, r.text)) listed.push(d);
  }
  out.push(
    !reached
      ? o("uploads-listing", "not_tested", "API inacessível")
      : o(
          "uploads-listing",
          listed.length ? "fail" : "pass",
          listed.length
            ? `GET ${listed.join(", ")} lista o diretório sem autenticação (achado A-02)`
            : "diretórios de /uploads não são listados",
          { statuses, listed },
        ),
  );
  const paths = [
    "/uploads/../../../../etc/passwd",
    "/uploads/%2e%2e/%2e%2e/%2e%2e/etc/passwd",
    "/uploads/..%2f..%2f..%2fetc/passwd",
    "/uploads/....//....//etc/passwd",
    "/uploads/..%5c..%5cetc/passwd",
  ];
  const res = [];
  const leaked = [];
  for (const p of paths) {
    const r = await E.rawGet(apiUrl(), p);
    res.push(r.status);
    if (leaksPasswd(r.text)) leaked.push(p);
  }
  out.push(
    res.every((s) => s === 0)
      ? o("uploads-traversal", "not_tested", "API inacessível")
      : o(
          "uploads-traversal",
          leaked.length ? "fail" : "pass",
          leaked.length
            ? `traversal devolveu /etc/passwd em ${leaked.length} variante(s)`
            : `${paths.length} variantes de ../ não saem da pasta (HTTP ${[...new Set(res)].join("/")})`,
          { statuses: res, leaked },
        ),
  );
  return out;
}

// ------------------------------------------------------ payload grande
async function checkPayload(ctx, E) {
  const big = `{"email":"obs-big@example.invalid","password":"${"A".repeat(5 * 1024 * 1024)}"}`;
  const r = await E.probe("POST", `${apiUrl()}/v1/auth/login`, {
    headers: { "Content-Type": "application/json" },
    body: big,
    timeoutMs: 30000,
  });
  if (!r.status) {
    return [
      /ECONNRESET|UND_ERR_SOCKET|EPIPE/.test(r.error || "")
        ? o(
            "payload-large",
            "pass",
            "servidor encerrou a conexão ao receber 5 MB",
            { error: r.error },
          )
        : o("payload-large", "not_tested", `sem resposta (${r.error})`),
    ];
  }
  const ok = r.status === 413 || r.status === 400;
  return [
    o(
      "payload-large",
      ok ? "pass" : "fail",
      ok
        ? `corpo de 5 MB rejeitado (HTTP ${r.status})`
        : `corpo de 5 MB foi processado (HTTP ${r.status}); sem teto de tamanho no login`,
      { status: r.status },
    ),
  ];
}

// ----------------------------------------------------------- open redirect
async function checkOpenRedirect(ctx, E) {
  const params = [
    "redirect",
    "next",
    "return",
    "returnUrl",
    "redirect_uri",
    "continue",
  ];
  const q =
    "?" + params.map((p) => `${p}=${encodeURIComponent(EVIL)}`).join("&");
  const targets = [
    `${config.webUrl}/login${q}`,
    `${config.webUrl}/auth/login${q}`,
    `${apiUrl()}/v1/auth/gateway/verify${q}`,
    ...reqEndpoints(
      ctx,
      (e) =>
        /\/auth\//.test(e.path) && !e.pathParams.length && e.auth === "public",
    )
      .slice(0, 5)
      .map((e) => apiUrl() + e.path + q),
  ];
  let checked = 0;
  const bad = [];
  for (const u of targets) {
    const r = await E.probe("GET", u);
    if (!r.status) continue;
    checked++;
    if (
      r.status >= 300 &&
      r.status < 400 &&
      redirectsExternal(r.location, u, [
        new URL(config.webUrl).hostname,
        new URL(apiUrl()).hostname,
      ])
    )
      bad.push(new URL(u).pathname);
  }
  if (!checked)
    return [o("open-redirect", "not_tested", "API e web inacessíveis")];
  return [
    o(
      "open-redirect",
      bad.length ? "fail" : "partial",
      bad.length
        ? `redireciona para domínio externo em ${bad.join(", ")}`
        : `${checked} rotas de auth não redirecionam para domínio externo; o redirecionamento pós-login feito no front não foi verificado`,
      { checked, redirecting: bad },
    ),
  ];
}

// ---------------------------------------------------------------- segredos
async function checkSecrets() {
  const ls = await sh("git", ["ls-files", "-z"], { cwd: REPO, timeout: 30000 });
  if (!ls.ok)
    return [o("secrets-repo", "not_tested", "git ls-files indisponível")];
  const files = ls.stdout.split("\0").filter((f) => f && !isIgnoredPath(f));
  const high = [];
  const heur = [];
  let scanned = 0;
  for (const f of files) {
    let text;
    try {
      const buf = readFileSync(join(REPO, f));
      if (buf.length > 512 * 1024 || buf.includes(0)) continue;
      text = buf.toString("utf8");
    } catch {
      continue;
    }
    scanned++;
    for (const h of scanText(text))
      (h.level === "high" ? high : heur).push({
        file: f,
        line: h.line,
        rule: h.rule,
      });
  }
  const ev = {
    filesScanned: scanned,
    high: high.slice(0, 40),
    heuristic: heur.slice(0, 40),
    heuristicTotal: heur.length,
  };
  if (high.length)
    return [
      o(
        "secrets-repo",
        "fail",
        `${high.length} credencial(is) com formato conhecido, ex.: ${high
          .slice(0, 3)
          .map((h) => `${h.file}:${h.line}`)
          .join(", ")}`,
        ev,
      ),
    ];
  if (heur.length)
    return [
      o(
        "secrets-repo",
        "partial",
        `${heur.length} atribuição(ões) literal(is) de senha/segredo a revisar (podem ser defaults de dev), ex.: ${heur
          .slice(0, 3)
          .map((h) => `${h.file}:${h.line}`)
          .join(", ")}`,
        ev,
      ),
    ];
  return [
    o(
      "secrets-repo",
      "pass",
      `${scanned} arquivos versionados sem credencial em formato conhecido`,
      ev,
    ),
  ];
}

// ------------------------------------------------------------- dependências
function checkDeps(ctx) {
  const find = (t) =>
    ctx.results.find((r) => r.kind === "health" && r.target === t);
  const noHealth =
    "fase health não rodou neste run (use --only=health,security)";
  const out = [];
  const npm = ["audit:web", "audit:observability"].map(find).filter(Boolean);
  if (!npm.length) out.push(o("deps-npm", "not_tested", noHealth));
  else {
    const warn = npm.filter((r) => r.status === "warn");
    const skip = npm.filter((r) => r.status === "skip");
    out.push(
      warn.length
        ? o(
            "deps-npm",
            "fail",
            warn.map((r) => `${r.target}: ${r.reason}`).join("; "),
            { audits: npm.map((r) => ({ t: r.target, s: r.status })) },
          )
        : skip.length === npm.length
          ? o(
              "deps-npm",
              "not_tested",
              `npm audit não rodou: ${skip[0].reason}`,
            )
          : o(
              "deps-npm",
              skip.length ? "partial" : "pass",
              skip.length
                ? `sem vulnerabilidade alta/crítica nos que rodaram; ${skip.length} audit(s) pulado(s)`
                : "npm audit (prod) sem vulnerabilidade alta/crítica",
              { audits: npm.map((r) => ({ t: r.target, s: r.status })) },
            ),
    );
  }
  const g = find("govulncheck");
  out.push(
    !g
      ? o("deps-go", "not_tested", noHealth)
      : g.status === "skip"
        ? o("deps-go", "not_tested", `govulncheck não rodou: ${g.reason}`)
        : g.status === "warn"
          ? o("deps-go", "fail", g.reason, g.metrics || {})
          : o(
              "deps-go",
              "pass",
              "govulncheck sem vulnerabilidade alcançável",
              g.metrics || {},
            ),
  );
  return out;
}

// --------------------------------------------------------------------- TLS
async function certOf(url) {
  const u = new URL(url);
  return new Promise((resolve) => {
    const s = tls.connect(
      {
        host: u.hostname,
        port: Number(u.port) || 443,
        servername: u.hostname,
        rejectUnauthorized: false,
        timeout: 8000,
      },
      () => {
        const c = s.getPeerCertificate();
        resolve({
          authorized: s.authorized,
          why: s.authorizationError ? String(s.authorizationError) : "",
          protocol: s.getProtocol(),
          days: c?.valid_to
            ? Math.floor((Date.parse(c.valid_to) - Date.now()) / 864e5)
            : null,
        });
        s.end();
      },
    );
    s.on("error", (e) => resolve({ error: e.code || e.message }));
    s.on("timeout", () => {
      s.destroy();
      resolve({ error: "timeout" });
    });
  });
}
async function checkTls() {
  const urls = [apiUrl(), config.webUrl].filter(isHttps);
  if (!urls.length)
    return [
      o(
        "tls",
        "not_tested",
        "ambiente sem HTTPS (URLs locais em http); testar contra produção",
      ),
    ];
  const found = [];
  for (const u of urls)
    found.push({ host: new URL(u).hostname, ...(await certOf(u)) });
  const bad = found.filter(
    (f) =>
      f.error ||
      !f.authorized ||
      (f.days ?? 0) < 0 ||
      !/TLSv1\.[23]/.test(f.protocol || ""),
  );
  const soon = found.filter(
    (f) => f.days != null && f.days >= 0 && f.days < 14,
  );
  return [
    o(
      "tls",
      bad.length ? "fail" : soon.length ? "partial" : "pass",
      bad.length
        ? `TLS com problema em ${bad.map((f) => f.host).join(", ")}: ${bad[0].error || bad[0].why || "protocolo/validade"}`
        : soon.length
          ? `certificado de ${soon[0].host} expira em ${soon[0].days} dias`
          : `certificado válido e ${found[0].protocol}`,
      { hosts: found },
    ),
  ];
}

// ------------------------------------------------------ força bruta (última)
async function checkBruteforce(ctx) {
  const n = Number(process.env.OBS_BRUTEFORCE_ATTEMPTS) || 30;
  const email = `obs-bruteforce-${randomBytes(5).toString("hex")}@example.invalid`;
  let first429 = null;
  const seen = {};
  for (let i = 1; i <= n; i++) {
    const r = await ctx.client.request("POST", "/v1/auth/login", {
      body: { email, password: `Wrong-${randomBytes(6).toString("hex")}!` },
      keepBody: false,
      retry429: false,
    });
    seen[r.status] = (seen[r.status] || 0) + 1;
    if (r.status === 429) {
      first429 = i;
      break;
    }
    if (r.status === 0) break;
  }
  const ev = { attempts: n, statuses: seen, first429 };
  if (seen[0] && !first429)
    return [o("login-bruteforce", "not_tested", "login inacessível", ev)];
  return [
    o(
      "login-bruteforce",
      first429 ? "pass" : "fail",
      first429
        ? `429 na tentativa ${first429} com e-mail inexistente`
        : `${n} tentativas seguidas sem 429: login sem limite por e-mail/IP (achado A-01)`,
      ev,
    ),
  ];
}

// -------------------------------------------------------------- orquestração
export async function runSecurity(ctx) {
  const prod = looksLikeProduction().length > 0;
  const E = makeEnv(ctx);
  // passive: pode rodar em produção. last: rodar por último (consome rate limit).
  const checks = [
    {
      id: "headers",
      keys: ["headers-api", "headers-web", "clickjacking"],
      passive: true,
      fn: () => checkHeaders(ctx, E),
    },
    {
      id: "cors",
      keys: ["cors-reflection"],
      passive: true,
      fn: () => checkCors(ctx, E),
    },
    {
      id: "cookie",
      keys: ["session-cookie"],
      passive: true,
      fn: () => checkCookie(ctx, E),
    },
    { id: "tls", keys: ["tls"], passive: true, fn: () => checkTls() },
    {
      id: "secrets",
      keys: ["secrets-repo"],
      passive: true,
      fn: () => checkSecrets(),
    },
    {
      id: "deps",
      keys: ["deps-npm", "deps-go"],
      passive: true,
      fn: () => checkDeps(ctx),
    },
    { id: "authz", keys: ["authz-no-token"], fn: () => checkAuthzNoToken(ctx) },
    {
      id: "jwt",
      keys: ["jwt-tampered", "jwt-alg-none"],
      fn: () => checkJwt(ctx, E),
    },
    {
      id: "tenant",
      keys: ["tenant-isolation"],
      fn: () => checkTenantIsolation(ctx),
    },
    { id: "enum403", keys: ["enum-403-404"], fn: () => checkEnum403(ctx) },
    { id: "sqli", keys: ["sqli-basic"], fn: () => checkSqli(ctx) },
    { id: "xss", keys: ["xss-reflected"], fn: () => checkXss(ctx, E) },
    {
      id: "uploads",
      keys: ["uploads-listing", "uploads-traversal"],
      fn: () => checkUploads(ctx, E),
    },
    {
      id: "redirect",
      keys: ["open-redirect"],
      fn: () => checkOpenRedirect(ctx, E),
    },
    {
      id: "payload",
      keys: ["payload-large"],
      fn: () => checkPayload(ctx, E),
      last: true,
    },
    {
      id: "enumeration",
      keys: ["user-enumeration"],
      fn: () => checkUserEnumeration(ctx),
      last: true,
    },
    {
      id: "bruteforce",
      keys: ["login-bruteforce"],
      fn: () => checkBruteforce(ctx),
      last: true,
    },
  ];
  const ordered = [
    ...checks.filter((c) => !c.last),
    ...checks.filter((c) => c.last),
  ];
  const outcomes = [];
  for (const c of ordered) {
    if (prod && !c.passive) {
      for (const k of c.keys)
        outcomes.push(
          o(
            k,
            "not_tested",
            "ambiente de produção: só checagens passivas (headers, TLS, cookies, CORS, repositório, dependências)",
          ),
        );
      continue;
    }
    try {
      outcomes.push(...(await c.fn()));
    } catch (e) {
      for (const k of c.keys)
        outcomes.push(
          o(
            k,
            "not_tested",
            `erro ao executar a checagem: ${String(e?.message || e).slice(0, 120)}`,
          ),
        );
    }
  }
  const results = outcomes.map((x) => ({
    kind: "security",
    module: "security",
    target: x.key,
    status: TO_RESULT[x.status],
    reason: x.detail,
    metrics: { securityStatus: x.status, ...x.evidence },
  }));
  return { results, outcomes };
}
