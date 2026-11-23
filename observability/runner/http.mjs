// Cliente HTTP do runner: throttle por rate limit, X-Request-ID rastreável,
// retry de 429, e login por perfil (POST /v1/auth/login + /v2/me/accounts).
import { config, credentialsFor } from "../config.mjs";
import { rateLimiter, sleep } from "./util.mjs";

export function createClient({
  runId,
  baseUrl = config.apiUrl,
  perMin = config.rateLimitPerMin,
} = {}) {
  const throttle = rateLimiter(perMin);
  let seq = 0;
  const client = {
    baseUrl,
    nextRequestId: () => `obsrun-${runId}-${++seq}`,
    get requestCount() {
      return seq;
    },
    async request(
      method,
      path,
      {
        token,
        accountId,
        body,
        headers = {},
        timeoutMs = config.timeouts.requestMs,
        keepBody = true,
        retry429 = true,
      } = {},
    ) {
      const requestId = client.nextRequestId();
      const h = {
        "X-Request-ID": requestId,
        "X-Obs-Source": "runner",
        Accept: "application/json",
        ...headers,
      };
      if (token) h.Authorization = `Bearer ${token}`;
      if (accountId) h["X-Account-Id"] = accountId;
      if (body !== undefined) h["Content-Type"] = "application/json";
      for (let attempt = 0; ; attempt++) {
        await throttle();
        const t0 = performance.now();
        try {
          const res = await fetch(baseUrl + path, {
            method,
            headers: h,
            body: body === undefined ? undefined : JSON.stringify(body),
            signal: AbortSignal.timeout(timeoutMs),
            redirect: "manual",
          });
          const buf = Buffer.from(await res.arrayBuffer());
          const ms = performance.now() - t0;
          if (res.status === 429 && retry429 && attempt < 2) {
            await sleep(Number(res.headers.get("retry-after") || 2) * 1000);
            continue;
          }
          let json;
          if (
            keepBody &&
            buf.length &&
            /json/i.test(res.headers.get("content-type") || "")
          ) {
            try {
              json = JSON.parse(buf.toString("utf8"));
            } catch {
              /* corpo não-JSON */
            }
          }
          return {
            status: res.status,
            ms,
            bytes: buf.length,
            json,
            requestId,
            contentType: res.headers.get("content-type") || "",
            location: res.headers.get("location") || "",
          };
        } catch (e) {
          return {
            status: 0,
            ms: performance.now() - t0,
            bytes: 0,
            requestId,
            error:
              e?.name === "TimeoutError"
                ? "timeout"
                : String(e?.cause?.code || e?.message || e),
          };
        }
      }
    },
  };
  return client;
}

// Valores de query que o próprio front envia (tenant/loja ativos e período),
// lidos do contexto da sessão. Usados só quando um GET responde 400 sem eles.
async function contextDefaults(client, sess) {
  const res = await client.request("GET", "/v1/me/context", {
    token: sess.token,
    accountId: sess.accountId,
  });
  const ctx = res.json?.context || {};
  const store =
    (ctx.stores || []).find((s) => s.id === ctx.activeStoreId) ||
    ctx.stores?.[0];
  const day = (d) => d.toISOString().slice(0, 10);
  const now = new Date();
  const from = new Date(now.getTime() - 30 * 864e5);
  const out = {
    tenantId: ctx.activeTenantId || store?.tenantId,
    storeId: ctx.activeStoreId || store?.id,
    storeCode: store?.code,
    accountId: sess.accountId,
    dateFrom: day(from),
    dateTo: day(now),
    from: day(from),
    to: day(now),
    month: day(now).slice(0, 7),
    weekStart: day(new Date(now.getTime() - ((now.getDay() + 6) % 7) * 864e5)),
    clientAccountId: ctx.activeTenantId || store?.tenantId,
    term: "a",
    search: "a",
  };
  return Object.fromEntries(Object.entries(out).filter(([, v]) => v));
}

// Sessões por perfil. Sem credencial => session null com motivo.
export async function loginProfiles(client, profiles, log = () => {}) {
  const sessions = {};
  for (const profile of profiles) {
    const cred = credentialsFor(profile);
    if (!cred) {
      sessions[profile] = {
        profile,
        error: `sem credencial do perfil ${profile} (defina OBS_USER_${profile}_EMAIL / OBS_USER_${profile}_PASSWORD)`,
      };
      continue;
    }
    const res = await client.request("POST", "/v1/auth/login", {
      body: { email: cred.email, password: cred.password },
    });
    const token = res.json?.session?.accessToken;
    if (res.status !== 200 || !token) {
      sessions[profile] = {
        profile,
        error: `login falhou para ${profile} (HTTP ${res.status})`,
      };
      log(`login ${profile}: HTTP ${res.status}`);
      continue;
    }
    const sess = {
      profile,
      token,
      user: res.json.user,
      role: res.json.user?.role || config.profiles[profile]?.role || null,
      email: cred.email,
      password: cred.password,
    };
    const acc = await client.request("GET", "/v2/me/accounts", { token });
    const accounts = acc.json?.accounts || acc.json?.data?.accounts || [];
    sess.accountId =
      accounts.find((a) => a.isDefault || a.active)?.id ||
      accounts[0]?.id ||
      null;
    sess.queryDefaults = await contextDefaults(client, sess);
    sessions[profile] = sess;
  }
  return sessions;
}
