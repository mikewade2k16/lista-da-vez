// Fase páginas: Chrome headless (playwright-core, channel 'chrome'), login pela
// UI uma vez por perfil, 1 page.goto por página para medir Web Vitals de verdade.
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { config, credentialsFor } from "../config.mjs";
import { mapPool, routeMatcher, round } from "./util.mjs";

const VITALS_INIT = `(() => {
  const o = (window.__obs = { lcp: null, cls: 0, inp: null });
  const obs = (type, cb, extra) => { try { new PerformanceObserver((l) => l.getEntries().forEach(cb)).observe({ type, buffered: true, ...extra }); } catch (e) {} };
  obs('largest-contentful-paint', (e) => { o.lcp = e.renderTime || e.loadTime || e.startTime; });
  obs('layout-shift', (e) => { if (!e.hadRecentInput) o.cls += e.value; });
  obs('event', (e) => { if (e.interactionId > 0 || e.duration >= 40) o.inp = Math.max(o.inp || 0, e.duration); }, { durationThreshold: 16 });
})()`;

const slug = (p) =>
  (p.replace(/[^a-zA-Z0-9]+/g, "_").replace(/^_|_$/g, "") || "home").slice(
    0,
    80,
  );
const hasParam = (p) => /\[[^\]]+\]|:[A-Za-z]|\{[^}]+\}/.test(p);

export function selectPages(inv, args) {
  let pages = inv.pages || [];
  if (args.modules.length)
    pages = pages.filter((p) => args.modules.includes(p.module));
  if (args.page)
    pages = pages.filter(
      (p) => p.path === args.page || p.path.includes(args.page),
    );
  return pages;
}

async function uiLogin(browser, profile, cred) {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
  });
  const page = await context.newPage();
  try {
    for (let attempt = 0; attempt < 2; attempt++) {
      await page.goto(`${config.webUrl}/auth/login`, {
        waitUntil: "load",
        timeout: config.timeouts.loginMs,
      });
      await page.waitForSelector('input[name="username"]', {
        timeout: config.timeouts.loginMs,
      });
      await page
        .waitForLoadState("networkidle", {
          timeout: config.timeouts.networkIdleMs,
        })
        .catch(() => {});
      await page.fill('input[name="username"]', cred.email);
      await page.fill('input[name="password"]', cred.password);
      await page.click('button[type="submit"]');
      const ok = await page
        .waitForURL((u) => !u.pathname.startsWith("/auth/"), {
          timeout: config.timeouts.loginMs,
        })
        .then(() => true)
        .catch(() => false);
      if (ok) {
        await page.close();
        return { context };
      }
    }
    await context.close();
    return { error: `login pela UI falhou para ${profile}` };
  } catch (e) {
    await context.close().catch(() => {});
    return {
      error: `login pela UI falhou para ${profile}: ${e.message.split("\n")[0]}`,
    };
  }
}

async function resolvePagePath(pg, ctx, session) {
  if (!hasParam(pg.path)) return { path: pg.path };
  const params = [
    ...pg.path.matchAll(/\[([^\]]+)\]|:([A-Za-z]\w*)|\{([^}]+)\}/g),
  ].map((m) => m[1] || m[2] || m[3]);
  const tpls = (pg.apis || [])
    .map((a) => a.replace(/^\w+\s+/, ""))
    .filter((p) => /\{[^}]+\}/.test(p));
  let path = pg.path;
  for (const name of params) {
    const hint = config.paramHints[name];
    let value = hint !== undefined ? String(hint) : null;
    if (value === null && ctx.resolver) {
      const cand =
        tpls.find((t) => t.includes(`{${name}}`)) ||
        (params.length === 1 && tpls.length === 1 ? tpls[0] : null);
      if (cand) {
        const r = await ctx.resolver.resolvePath(cand, session);
        if (r.path) {
          const a = cand.split("/"),
            b = r.path.split("/");
          const i = a.findIndex(
            (s) =>
              /^\{.+\}$/.test(s) && (s === `{${name}}` || tpls.length === 1),
          );
          if (i >= 0) value = b[i];
        }
      }
    }
    if (value === null) return { missing: name };
    path = path.replace(
      new RegExp(`\\[${name}\\]|:${name}\\b|\\{${name}\\}`),
      encodeURIComponent(value),
    );
  }
  return { path };
}

export async function runPages(ctx) {
  const { inv, args, sessions, outDir, log } = ctx;
  const pages = selectPages(inv, args);
  const results = [];
  if (!pages.length) return results;
  const mk = (pg, status, reason, metrics = {}) => ({
    kind: "page",
    module: pg.module || "core",
    target: pg.path,
    status,
    reason,
    metrics,
  });
  let playwright, browser;
  try {
    playwright = await import("playwright-core");
    browser = await playwright.chromium.launch({
      channel: "chrome",
      headless: true,
    });
  } catch (e) {
    return pages.map((p) =>
      mk(
        p,
        "skip",
        `navegador indisponível (Google Chrome/playwright-core): ${e.message.split("\n")[0]}`,
      ),
    );
  }
  const shotDir = join(outDir, "screenshots");
  mkdirSync(shotDir, { recursive: true });
  const match = routeMatcher(inv.endpoints || []);
  const session = sessions[args.profile];
  const contexts = {};
  try {
    const needLogin = pages.some((p) => p.requiresLogin !== false);
    let authCtx = null,
      authErr = null;
    if (needLogin) {
      const cred = credentialsFor(args.profile);
      if (!cred)
        authErr = `sem credencial do perfil ${args.profile} (defina OBS_USER_${args.profile}_EMAIL)`;
      else {
        log(`  login pela UI (${args.profile})`);
        const r = await uiLogin(browser, args.profile, cred);
        authCtx = r.context;
        authErr = r.error;
      }
    }
    contexts.auth = authCtx;
    contexts.anon = await browser.newContext({
      viewport: { width: 1280, height: 800 },
    });
    await mapPool(pages, 2, async (pg) => {
      const needs = pg.requiresLogin !== false;
      if (needs && !authCtx) return results.push(mk(pg, "skip", authErr));
      const r = await resolvePagePath(pg, ctx, session);
      if (r.missing)
        return results.push(mk(pg, "skip", `sem valor para {${r.missing}}`));
      results.push(
        await measure(needs ? authCtx : contexts.anon, pg, r.path, {
          shotDir,
          match,
          args,
        }),
      );
    });
  } finally {
    for (const c of Object.values(contexts)) await c?.close().catch(() => {});
    await browser.close().catch(() => {}); // Playwright remove o perfil temporário ao fechar
  }
  return results;
}

async function measure(context, pg, path, { shotDir, match, args }) {
  const page = await context.newPage();
  await page.addInitScript(VITALS_INIT);
  await page.setViewportSize({ width: 1280, height: 800 });
  const consoleErrors = [],
    pageErrors = [],
    failed = [],
    apis = new Map();
  const apiBase = config.apiUrl;
  page.on("console", (m) => {
    if (m.type() === "error") consoleErrors.push(m.text().slice(0, 300));
  });
  page.on("pageerror", (e) => pageErrors.push(String(e.message).slice(0, 300)));
  page.on("request", (rq) => {
    if (!rq.url().startsWith(apiBase)) return;
    const u = new URL(rq.url());
    const ep = match(rq.method(), u.pathname);
    const key = ep
      ? ep.id
      : `${rq.method()} ${u.pathname} (fora do inventário)`;
    apis.set(key, (apis.get(key) || 0) + 1);
  });
  page.on("response", (rs) => {
    if (rs.status() >= 400)
      failed.push({
        method: rs.request().method(),
        url: rs.url().split("?")[0],
        status: rs.status(),
      });
  });
  page.on("requestfailed", (rq) => {
    const f = rq.failure()?.errorText || "failed";
    if (!/ERR_ABORTED/.test(f))
      failed.push({
        method: rq.method(),
        url: rq.url().split("?")[0],
        status: 0,
        error: f,
      });
  });
  const t0 = Date.now();
  let main = null,
    navError = null;
  try {
    main = await page.goto(config.webUrl + path, {
      waitUntil: "load",
      timeout: config.timeouts.pageMs,
    });
  } catch (e) {
    navError = e.message.split("\n")[0];
  }
  let idle = true;
  if (!navError)
    idle = await page
      .waitForLoadState("networkidle", {
        timeout: config.timeouts.networkIdleMs,
      })
      .then(() => true)
      .catch(() => false);
  const loadMs = Date.now() - t0;
  let v = {};
  if (!navError) {
    await page.waitForTimeout(600);
    await page.keyboard.press("Shift").catch(() => {});
    await page.waitForTimeout(250);
    v = await page
      .evaluate(() => {
        const n = performance.getEntriesByType("navigation")[0];
        const o = window.__obs || {};
        return {
          lcp: o.lcp,
          cls: o.cls,
          inp: o.inp,
          ttfb: n ? n.responseStart : null,
          domLoaded: n ? n.domContentLoadedEventEnd : null,
          finalPath: location.pathname,
        };
      })
      .catch(() => ({}));
  }
  const file = `${slug(path)}.jpg`;
  let screenshot = null;
  if (!navError)
    screenshot = await page
      .screenshot({ path: join(shotDir, file), type: "jpeg", quality: 50 })
      .then(() => `screenshots/${file}`)
      .catch(() => null);
  await page.close().catch(() => {});
  const lim = config.limits.vitals;
  const metrics = {
    profile: pg.requiresLogin === false ? null : args.profile,
    loadMs,
    networkIdle: idle,
    lcp: round(v.lcp),
    cls: round(v.cls, 3),
    inp: round(v.inp),
    ttfb: round(v.ttfb),
    httpStatus: main?.status() ?? null,
    finalPath: v.finalPath,
    consoleErrors: consoleErrors.slice(0, 10),
    pageErrors: pageErrors.slice(0, 10),
    failedRequests: failed.slice(0, 20),
    apisObserved: [...apis].map(([id, count]) => ({ id, count })),
    screenshot,
  };
  const reasons = [];
  let status = "pass";
  const worse = () => {
    if (status !== "fail") status = "warn";
  };
  if (navError) {
    status = "fail";
    reasons.push(`navegação falhou: ${navError}`);
  } else {
    if (metrics.httpStatus >= 500) {
      status = "fail";
      reasons.push(`HTTP ${metrics.httpStatus}`);
    }
    if (failed.some((f) => f.status >= 500)) {
      status = "fail";
      reasons.push(
        `${failed.filter((f) => f.status >= 500).length} chamada(s) 5xx`,
      );
    }
    if (
      pg.requiresLogin !== false &&
      v.finalPath &&
      v.finalPath.startsWith("/auth/") &&
      !path.startsWith("/auth/")
    ) {
      worse();
      reasons.push("redirecionou para login");
    }
    if (pageErrors.length) {
      worse();
      reasons.push(`${pageErrors.length} erro(s) JS não tratado(s)`);
    }
    if (consoleErrors.length) {
      worse();
      reasons.push(`${consoleErrors.length} erro(s) no console`);
    }
    if (
      failed.some((f) => f.status >= 400 && f.status < 500) ||
      failed.some((f) => f.status === 0)
    ) {
      worse();
      reasons.push(
        `${failed.filter((f) => f.status < 500).length} request(s) com falha`,
      );
    }
    for (const [k, l] of [
      ["lcp", lim.lcp],
      ["inp", lim.inp],
      ["cls", lim.cls],
      ["ttfb", lim.ttfb],
    ])
      if (metrics[k] != null && metrics[k] > l) {
        worse();
        reasons.push(`${k.toUpperCase()} ${metrics[k]} > ${l}`);
      }
    if (loadMs > config.limits.pageLoadMs) {
      worse();
      reasons.push(`carga ${loadMs}ms > ${config.limits.pageLoadMs}ms`);
    }
    if (!idle) reasons.push("networkidle não atingido");
  }
  return {
    kind: "page",
    module: pg.module || "core",
    target: pg.path,
    status,
    reason: reasons.join("; "),
    metrics,
  };
}
