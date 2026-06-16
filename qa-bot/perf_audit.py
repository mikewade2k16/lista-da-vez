"""Auditoria de performance de navegacao do painel Omni.

Mede, por rota, 3 marcos em 3 rodadas SEM cache, nos modos in-app (SPA) e cold
(1a visita / F5), como platform_admin:

  T1 - clique -> troca de rota        (fetch bloqueante de setup/middleware)
  T2 - clique -> primeira pintura     (conteudo aparece em tela)
  T3 - clique -> carregamento final   (network idle + sem skeleton)

Saida: qa-bot/artifacts/perf-<timestamp>.csv e .md

Uso:
  OMNI_QA_EMAIL=... OMNI_QA_PASSWORD=... python perf_audit.py
Flags opcionais:
  --base-url http://localhost:3003  --runs 3  --headed  --only /operacao,/crm

Plano canonico: docs/PERFORMANCE_AUDIT_PLAN.md
"""

from __future__ import annotations

import argparse
import json
import os
import statistics
import sys
import time
from datetime import datetime
from pathlib import Path

from playwright.sync_api import Page, TimeoutError as PlaywrightTimeoutError, sync_playwright

ARTIFACTS_DIR = Path(__file__).resolve().parent / "artifacts"

# Modulo TS consumido pela pagina /performance do painel (Track A). Regenerado a
# cada run para a pagina refletir a ultima auditoria sem re-rodar nada no front.
PERF_DATA_TS = (
  Path(__file__).resolve().parent.parent
  / "web"
  / "app"
  / "components"
  / "performance"
  / "perf-data.ts"
)

# Rotas estaticas do platform_admin (espelha docs/PERFORMANCE_AUDIT_PLAN.md secao 7).
STATIC_ROUTES: list[str] = [
  "/",
  "/operacao",
  "/operacao/usuarios",
  "/operacao/clientes",
  "/tasks",
  "/tracking",
  "/editor",
  "/automation",
  "/omnichannel",
  "/consultor",
  "/ranking",
  "/dados",
  "/inteligencia",
  "/relatorios",
  "/bi",
  "/crm",
  "/erp",
  "/finance",
  "/monitoramento",
  "/multiloja",
  "/configuracoes",
  "/alertas",
  "/feedback",
  "/campanhas",
  "/meta-ads",
  "/cardapio",
  "/site/leads",
  "/site/produtos",
  "/site/tracking",
  "/site/bio",
  "/manage/clientes",
  "/manage/clientes-web",
  "/manage/produtos-web",
  "/manage/leads-web",
  "/manage/users",
  "/manage/organizations",
  "/manage/auditoria",
  "/manage/integracoes",
  "/themes",
  "/banco",
  "/roadmap",
  "/perfil",
  "/meus-feedbacks",
  "/usuarios",
  "/clientes",
  "/team/equipe",
  "/team/escalas",
  "/tools/qr-code",
  "/tools/encurtador-de-link",
  "/tools/scripts",
]

# Rota neutra usada como ponto de partida de cada navegacao in-app.
NEUTRAL_ROUTE = "/perfil"

# Instala contador de requests em voo + observer persistente de mutacao num no
# estavel (documentElement), que sobrevive a troca de pagina SPA. Roda a cada
# documento novo (login e cada cold load) e persiste pela sessao SPA.
INIT_SCRIPT = """
(() => {
  if (window.__perfInit) return;
  window.__perfInit = true;
  window.__perf = { inflight: 0, lastMutation: performance.now() };
  const inc = () => { window.__perf.inflight++; };
  const dec = () => { window.__perf.inflight = Math.max(0, window.__perf.inflight - 1); };
  const origFetch = window.fetch;
  if (origFetch) {
    window.fetch = function (...args) {
      inc();
      return origFetch.apply(this, args).finally(dec);
    };
  }
  const origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send = function (...args) {
    inc();
    this.addEventListener('loadend', dec, { once: true });
    return origSend.apply(this, args);
  };
  const startObserver = () => {
    try {
      const obs = new MutationObserver(() => { window.__perf.lastMutation = performance.now(); });
      obs.observe(document.documentElement, { childList: true, subtree: true, characterData: true });
      window.__perf._obs = obs;
    } catch (e) {}
  };
  if (document.documentElement) startObserver();
  else document.addEventListener('DOMContentLoaded', startObserver, { once: true });
})();
"""

# Define window.__perfNavigate(path): dispara navegacao SPA pelo Vue Router e
# resolve com {t1,t2,t3,finalUrl}. Equivale a clicar no item de menu (mesma
# middleware + setup + Suspense), mas funciona tambem para rotas hidden.
NAV_SCRIPT = """
window.__perfNavigate = function (path) {
  return new Promise((resolve) => {
    try {
      const root = document.getElementById('__nuxt') || document.body;
      const app = root && root.__vue_app__;
      const router = app && app.config && app.config.globalProperties && app.config.globalProperties.$router;
      if (!router) { resolve({ error: 'no-router app=' + (!!app) }); return; }

      const t0 = performance.now();
      let routeChangeAt = null;
      let firstPaintAt = null;
      let settledAt = null;
      let navError = null;
      let capped = false;
      const fromPath = location.pathname;

      // Troca de rota detectada por history.pushState/replaceState (robusto e
      // independente de afterEach, que pode nao disparar se a navegacao falhar).
      const origPush = history.pushState;
      const origReplace = history.replaceState;
      function onUrlChange() {
        if (routeChangeAt === null && location.pathname !== fromPath) {
          routeChangeAt = performance.now();
        }
      }
      history.pushState = function (...a) { const r = origPush.apply(this, a); onUrlChange(); return r; };
      history.replaceState = function (...a) { const r = origReplace.apply(this, a); onUrlChange(); return r; };

      function cleanup() {
        try { history.pushState = origPush; history.replaceState = origReplace; } catch (e) {}
        resolve({
          t1: routeChangeAt !== null ? routeChangeAt - t0 : null,
          t2: firstPaintAt !== null ? firstPaintAt - t0 : null,
          t3: settledAt !== null ? settledAt - t0 : null,
          finalUrl: location.pathname,
          navError: navError,
          capped: capped,
        });
      }

      function tick() {
        const now = performance.now();
        // Primeira pintura: 1a mutacao do DOM (observer global) apos a troca de rota.
        if (routeChangeAt !== null && firstPaintAt === null && window.__perf.lastMutation > routeChangeAt) {
          firstPaintAt = now;
        }
        const noSkeleton = !document.querySelector('.core-skeleton');
        const quiet = (now - window.__perf.lastMutation) > 650;
        // streamed: 4s apos a 1a pintura o DOM ainda muda -> e atualizacao
        // realtime ao vivo, nao mais "carga inicial". Encerramos e marcamos.
        const streamed = firstPaintAt !== null && (now - firstPaintAt) > 4000;
        if (routeChangeAt !== null && firstPaintAt !== null && noSkeleton && (quiet || streamed)) {
          capped = streamed && !quiet;
          settledAt = now; cleanup(); return;
        }
        if (now - t0 > 15000) { capped = true; settledAt = now; cleanup(); return; }
        setTimeout(tick, 80);
      }

      // router.push pode lancar sincronamente em alguns estados; isolamos.
      Promise.resolve().then(() => router.push(path)).catch((e) => {
        navError = String((e && e.message) || e);
      });
      setTimeout(tick, 80);
    } catch (e) {
      resolve({ error: 'exec: ' + String((e && e.message) || e) });
    }
  });
};
"""

COLD_TIMINGS_SCRIPT = """
() => {
  const nav = performance.getEntriesByType('navigation')[0] || {};
  const fcp = performance.getEntriesByType('paint').find((p) => p.name === 'first-contentful-paint');
  return {
    t1: typeof nav.domInteractive === 'number' ? nav.domInteractive : null,
    t2: fcp ? fcp.startTime : (typeof nav.domContentLoadedEventEnd === 'number' ? nav.domContentLoadedEventEnd : null),
    now: performance.now(),
    finalUrl: location.pathname,
  };
}
"""

COLD_SETTLE_SCRIPT = """
() => {
  const p = window.__perf || {};
  const fcp = performance.getEntriesByType('paint').find((e) => e.name === 'first-contentful-paint');
  if (!fcp) return { settled: false, streaming: false };
  const noSkeleton = !document.querySelector('.core-skeleton');
  const now = performance.now();
  const quiet = (now - (p.lastMutation || 0)) > 650;
  const streamed = (now - fcp.startTime) > 4000;
  return { settled: noSkeleton && (quiet || streamed), streaming: noSkeleton && !quiet && streamed };
}
"""


def log(msg: str) -> None:
  print(f"[perf-audit] {msg}", flush=True)


def login(page: Page, base_url: str, email: str, password: str, timeout_ms: int) -> None:
  # Timeout generoso: em dev a 1a compilacao da rota de login pode passar de 30s.
  boot_ms = max(timeout_ms, 120000)
  log("Fazendo login como platform_admin (1a carga pode compilar a rota)...")
  page.goto(f"{base_url}/auth/login", wait_until="commit", timeout=boot_ms)
  page.wait_for_selector('input[name="username"]', timeout=boot_ms)
  # Espera o Vue hidratar antes de submeter. Em dev a 1a compilacao da rota de
  # login pode levar minutos; sem hidratacao o submit cai no GET nativo do form
  # (@submit.prevent ainda nao ligado) -> /auth/login?username=... e trava.
  page.wait_for_function(
    "() => { const r = document.getElementById('__nuxt'); return !!(r && r.__vue_app__); }",
    timeout=boot_ms,
  )
  page.fill('input[name="username"]', email, timeout=timeout_ms)
  page.fill('input[name="password"]', password, timeout=timeout_ms)
  page.click('button[type="submit"].admin-auth-submit', timeout=timeout_ms)
  # Espera sair de /auth/* (redirect para homePath).
  page.wait_for_url(lambda url: "/auth/" not in url, timeout=boot_ms)
  page.wait_for_load_state("networkidle", timeout=boot_ms)
  log(f"Login OK. Rota inicial: {page.evaluate('() => location.pathname')}")


def warmup_routes(page: Page, base_url: str, routes: list[str], timeout_ms: int) -> None:
  """Visita cada rota 1x para compilar (dev) / aquecer cache antes de medir.

  Isola o custo de 1a-compilacao do Vite do custo real de navegacao do app.
  Em build de producao e praticamente no-op.
  """
  log(f"Warm-up: pre-compilando/aquecendo {len(routes)} rotas...")
  for path in routes:
    try:
      page.goto(f"{base_url}{path}", wait_until="domcontentloaded", timeout=max(timeout_ms, 120000))
    except Exception:  # noqa: BLE001 - rota pode redirecionar/timeout; warm-up nao falha o run
      pass


def discover_dynamic_routes(page: Page, base_url: str, timeout_ms: int) -> list[str]:
  """Acha o 1o id real de /site/bio e /cardapio clicando na 1a linha da lista.

  As listas navegam por @click na linha (navigateTo), nao por <a href>, entao
  scrapear href nao funciona — clicamos e capturamos a URL de detalhe.
  """
  extra: list[str] = []
  targets = (
    ("/site/bio", "/site/bio/", ".bio-list__row"),
    ("/cardapio", "/cardapio/", "tbody tr"),
  )
  for list_path, prefix, row_sel in targets:
    try:
      page.goto(f"{base_url}{list_path}", wait_until="load", timeout=timeout_ms)
      row = page.locator(row_sel).first
      row.wait_for(state="visible", timeout=6000)
      row.click()
      page.wait_for_url(lambda u: prefix in u, timeout=8000)
      path = page.evaluate("() => location.pathname")
      if path.startswith(prefix) and len(path) > len(prefix):
        extra.append(path)
        log(f"Rota dinamica descoberta: {path}")
      else:
        log(f"Clique em {list_path} nao levou a {prefix} (pulada).")
    except Exception as error:  # noqa: BLE001 - lista vazia/sem item: pula
      log(f"Sem item clicavel em {list_path} (rota [id] pulada): {str(error)[:70]}")
  return extra


def measure_inapp(page: Page, base_url: str, path: str, timeout_ms: int) -> dict:
  # Parte de uma rota neutra ESTATICA e diferente do alvo (push para a propria
  # rota e no-op e nunca troca a URL -> estouraria o cap de settle).
  start = NEUTRAL_ROUTE if path.rstrip("/") != NEUTRAL_ROUTE.rstrip("/") else "/roadmap"
  page.goto(f"{base_url}{start}", wait_until="load", timeout=timeout_ms)
  page.wait_for_selector("#__nuxt", timeout=timeout_ms)
  page.wait_for_timeout(300)  # deixa o app montar/estabilizar antes de medir
  page.evaluate(NAV_SCRIPT)
  return page.evaluate("(p) => window.__perfNavigate(p)", path)


def measure_cold(page: Page, base_url: str, path: str, timeout_ms: int) -> dict:
  page.goto(f"{base_url}{path}", wait_until="commit", timeout=timeout_ms)
  deadline = time.monotonic() + 15.0  # safety cap
  idle_hits = 0
  capped = False
  while time.monotonic() < deadline:
    try:
      state = page.evaluate(COLD_SETTLE_SCRIPT)
    except Exception:  # noqa: BLE001 - documento ainda trocando
      state = {"settled": False, "streaming": False}
    idle_hits = idle_hits + 1 if state.get("settled") else 0
    if idle_hits >= 2:
      capped = bool(state.get("streaming"))
      break
    page.wait_for_timeout(150)
  timings = page.evaluate(COLD_TIMINGS_SCRIPT)
  return {
    "t1": timings.get("t1"),
    "t2": timings.get("t2"),
    "t3": timings.get("now"),
    "finalUrl": timings.get("finalUrl"),
    "capped": capped,
  }


def fmt(value) -> str:
  return f"{value / 1000:.2f}s" if isinstance(value, (int, float)) else "—"


def avg(values: list[float]) -> float | None:
  clean = [v for v in values if isinstance(v, (int, float))]
  return statistics.mean(clean) if clean else None


def main() -> int:
  parser = argparse.ArgumentParser(description="Auditoria de performance de navegacao Omni")
  parser.add_argument("--base-url", default=os.environ.get("OMNI_QA_BASE_URL", "http://localhost:3003"))
  parser.add_argument("--runs", type=int, default=3)
  parser.add_argument("--timeout-ms", type=int, default=30000)
  parser.add_argument("--headed", action="store_true")
  parser.add_argument("--only", default="", help="lista de rotas separadas por virgula")
  parser.add_argument("--modes", default="inapp,cold")
  parser.add_argument("--warmup", action="store_true", help="visita cada rota 1x antes de medir (isola compile do Vite no dev)")
  parser.add_argument("--discover", action="store_true", help="descobre rotas dinamicas (/site/bio/[id], /cardapio/[id]) mesmo com --only")
  args = parser.parse_args()

  email = os.environ.get("OMNI_QA_EMAIL", "")
  password = os.environ.get("OMNI_QA_PASSWORD", "")
  if not email or not password:
    log("ERRO: defina OMNI_QA_EMAIL e OMNI_QA_PASSWORD no ambiente.")
    return 2

  modes = [m.strip() for m in args.modes.split(",") if m.strip()]
  ARTIFACTS_DIR.mkdir(parents=True, exist_ok=True)

  with sync_playwright() as pw:
    browser = pw.chromium.launch(headless=not args.headed)
    context = browser.new_context(viewport={"width": 1440, "height": 900})
    context.add_init_script(INIT_SCRIPT)
    page = context.new_page()
    page.set_default_timeout(args.timeout_ms)
    cdp = context.new_cdp_session(page)
    cdp.send("Network.setCacheDisabled", {"cacheDisabled": True})

    login(page, args.base_url, email, password, args.timeout_ms)

    routes = [r.strip() for r in args.only.split(",") if r.strip()] if args.only else list(STATIC_ROUTES)
    if not args.only or args.discover:
      routes += discover_dynamic_routes(page, args.base_url, args.timeout_ms)

    if args.warmup:
      warmup_routes(page, args.base_url, routes, args.timeout_ms)

    # Warm-up do processo (descartado): 1 navegacao que nao entra no relatorio.
    try:
      measure_inapp(page, args.base_url, "/perfil", args.timeout_ms)
    except Exception:  # noqa: BLE001
      pass

    rows: list[dict] = []
    total = len(routes) * len(modes)
    done = 0
    for path in routes:
      for mode in modes:
        done += 1
        runs: list[dict] = []
        for run_idx in range(1, args.runs + 1):
          try:
            result = measure_inapp(page, args.base_url, path, args.timeout_ms) if mode == "inapp" \
              else measure_cold(page, args.base_url, path, args.timeout_ms)
          except Exception as error:  # noqa: BLE001 - registramos a falha e seguimos
            result = {"t1": None, "t2": None, "t3": None, "finalUrl": "", "error": str(error)[:120]}
          runs.append(result)
          rows.append({
            "path": path, "mode": mode, "run": run_idx,
            "t1": result.get("t1"), "t2": result.get("t2"), "t3": result.get("t3"),
            "finalUrl": result.get("finalUrl", ""),
            "error": result.get("error", "") or result.get("navError", "") or "",
            "capped": bool(result.get("capped")),
          })
        final_url = next((r.get("finalUrl") for r in runs if r.get("finalUrl")), "")
        redirected = bool(final_url) and final_url.rstrip("/") != path.rstrip("/")
        log(
          f"[{done:02d}/{total}] {mode:5s} {path}"
          + (f"  -> REDIRECIONOU para {final_url}" if redirected else "")
          + "  T1med={t1} T2med={t2} T3med={t3}".format(
            t1=fmt(avg([r.get("t1") for r in runs])),
            t2=fmt(avg([r.get("t2") for r in runs])),
            t3=fmt(avg([r.get("t3") for r in runs])),
          )
        )

    browser.close()

  stamp = datetime.now().strftime("%Y%m%d-%H%M%S")
  write_reports(rows, args, stamp)
  write_perf_data_ts(rows, args, stamp)
  return 0


def write_reports(rows: list[dict], args, stamp: str | None = None) -> None:
  stamp = stamp or datetime.now().strftime("%Y%m%d-%H%M%S")
  csv_path = ARTIFACTS_DIR / f"perf-{stamp}.csv"
  md_path = ARTIFACTS_DIR / f"perf-{stamp}.md"

  with csv_path.open("w", encoding="utf-8", newline="") as fh:
    fh.write("path,mode,run,t1_ms,t2_ms,t3_ms,final_url,error\n")
    for r in rows:
      fh.write(
        f"{r['path']},{r['mode']},{r['run']},"
        f"{_num(r['t1'])},{_num(r['t2'])},{_num(r['t3'])},{r['finalUrl']},{r['error']}\n"
      )

  # Agrega por (path, mode).
  agg: dict[tuple[str, str], dict] = {}
  for r in rows:
    key = (r["path"], r["mode"])
    bucket = agg.setdefault(key, {"t1": [], "t2": [], "t3": [], "runs": [], "finalUrl": "", "error": "", "capped": False})
    bucket["t1"].append(r["t1"]); bucket["t2"].append(r["t2"]); bucket["t3"].append(r["t3"])
    bucket["runs"].append(r)
    if r["finalUrl"]:
      bucket["finalUrl"] = r["finalUrl"]
    if r["error"]:
      bucket["error"] = r["error"]
    if r.get("capped"):
      bucket["capped"] = True

  lines: list[str] = []
  lines.append(f"# Relatorio de performance de navegacao — {stamp}")
  lines.append("")
  lines.append(f"- Base URL: {args.base_url} · Rodadas: {args.runs} · Modos: {args.modes}")
  lines.append("- Marcos: T1 clique->troca de rota · T2 clique->primeira pintura · T3 clique->carregamento final")
  lines.append("- Plano: docs/PERFORMANCE_AUDIT_PLAN.md")
  lines.append("")

  for mode in [m.strip() for m in args.modes.split(",") if m.strip()]:
    mode_keys = [k for k in agg if k[1] == mode]
    mode_keys.sort(key=lambda k: avg(agg[k]["t3"]) or 1e12, reverse=True)
    lines.append(f"## Modo {mode} — ranking (mais lenta -> mais rapida por T3)")
    lines.append("")
    lines.append("| Rota | T1 med | T2 med | T3 med | Obs |")
    lines.append("|---|---|---|---|---|")
    for key in mode_keys:
      b = agg[key]
      obs = []
      if b["finalUrl"] and b["finalUrl"].rstrip("/") != key[0].rstrip("/"):
        obs.append(f"redirect->{b['finalUrl']}")
      if b["capped"]:
        obs.append("nunca-quiet (realtime: T3 no cap 15s)")
      if b["error"]:
        obs.append(f"erro: {b['error']}")
      lines.append(
        f"| {key[0]} | {fmt(avg(b['t1']))} | {fmt(avg(b['t2']))} | {fmt(avg(b['t3']))} | {', '.join(obs) or '-'} |"
      )
    lines.append("")

  lines.append("## Detalhe por rota (3 rodadas)")
  lines.append("")
  for key in sorted(agg.keys()):
    b = agg[key]
    lines.append(f"### {key[0]}  ({key[1]})")
    for marco, idx in (("T1 clique->troca", "t1"), ("T2 clique->pintura", "t2"), ("T3 clique->final", "t3")):
      per_run = "  ".join(f"run{r['run']} {fmt(r[idx])}" for r in b["runs"])
      lines.append(f"- {marco}: {per_run}  | media {fmt(avg(b[idx]))}")
    lines.append("")

  md_path.write_text("\n".join(lines), encoding="utf-8")
  log(f"CSV: {csv_path}")
  log(f"MD:  {md_path}")


def _ts_num(value) -> str:
  return f"{round(value, 1)}" if isinstance(value, (int, float)) else "null"


def write_perf_data_ts(rows: list[dict], args, stamp: str | None = None) -> None:
  """Emite web/app/components/performance/perf-data.ts (modulo TS tipado).

  Agrega por (path, mode) preservando a ordem de primeira aparicao, tira a media
  de T1/T2/T3 (ms) e marca `capped` se qualquer rodada bateu o cap (realtime/15s).
  Consumido pela pagina /performance (Track A); regenerado a cada run.
  """
  stamp = stamp or datetime.now().strftime("%Y%m%d-%H%M%S")

  agg: dict[tuple[str, str], dict] = {}
  order: list[tuple[str, str]] = []
  for r in rows:
    key = (r["path"], r["mode"])
    if key not in agg:
      agg[key] = {"t1": [], "t2": [], "t3": [], "capped": False}
      order.append(key)
    bucket = agg[key]
    bucket["t1"].append(r.get("t1"))
    bucket["t2"].append(r.get("t2"))
    bucket["t3"].append(r.get("t3"))
    if r.get("capped"):
      bucket["capped"] = True

  lines: list[str] = []
  lines.append("// AUTOGERADO por qa-bot/perf_audit.py (write_perf_data_ts).")
  lines.append("// Nao editar a mao: re-rodar a auditoria sobrescreve este arquivo.")
  lines.append("// Tempos em ms. T1 clique->troca de rota · T2 clique->primeira pintura · T3 clique->carregamento final.")
  lines.append("")
  lines.append("export interface PerfRow {")
  lines.append("  path: string")
  lines.append("  mode: 'inapp' | 'cold'")
  lines.append("  t1: number | null")
  lines.append("  t2: number | null")
  lines.append("  t3: number | null")
  lines.append("  capped: boolean")
  lines.append("}")
  lines.append("")
  lines.append("export const PERF_RUN = {")
  lines.append(f"  stamp: '{stamp}',")
  lines.append(f"  baseUrl: '{args.base_url}',")
  lines.append("}")
  lines.append("")
  lines.append("export const PERF_ROWS: PerfRow[] = [")
  for key in order:
    bucket = agg[key]
    capped = "true" if bucket["capped"] else "false"
    lines.append(
      f"  {{ path: '{key[0]}', mode: '{key[1]}', "
      f"t1: {_ts_num(avg(bucket['t1']))}, "
      f"t2: {_ts_num(avg(bucket['t2']))}, "
      f"t3: {_ts_num(avg(bucket['t3']))}, "
      f"capped: {capped} }},"
    )
  lines.append("]")
  lines.append("")

  PERF_DATA_TS.parent.mkdir(parents=True, exist_ok=True)
  PERF_DATA_TS.write_text("\n".join(lines), encoding="utf-8")
  log(f"TS:  {PERF_DATA_TS}")


def _num(value) -> str:
  return f"{value:.1f}" if isinstance(value, (int, float)) else ""


if __name__ == "__main__":
  sys.exit(main())
