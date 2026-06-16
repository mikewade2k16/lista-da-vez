"""Pre-aquece (compila) TODAS as rotas no dev server.

No modo dev o Vite compila cada rota sob demanda na 1a visita da sessao (pode
levar segundos/minutos). Rodando este script LOGO APOS `docker compose up`, todas
as rotas ja entram compiladas e a 1a navegacao do dia fica instantanea.

Imprime o tempo de cada rota: a 1a passada mostra o custo de compile; rode de novo
e veja cair para ~0,1s (prova de que era compile, nao o app).

Uso:
  OMNI_QA_EMAIL=... OMNI_QA_PASSWORD=... python warmup_dev.py
  flags: --base-url http://localhost:3003  --headed  --timeout-ms 240000

Detalhe e contexto: docs/PERFORMANCE_AUDIT_PLAN.md (secao 3.1).
"""

from __future__ import annotations

import argparse
import os
import sys
import time

from playwright.sync_api import sync_playwright

from perf_audit import STATIC_ROUTES, login


def main() -> int:
  parser = argparse.ArgumentParser(description="Pre-aquece as rotas do dev server (compile do Vite)")
  parser.add_argument("--base-url", default=os.environ.get("OMNI_QA_BASE_URL", "http://localhost:3003"))
  parser.add_argument("--timeout-ms", type=int, default=240000)
  parser.add_argument("--headed", action="store_true")
  args = parser.parse_args()

  email = os.environ.get("OMNI_QA_EMAIL", "")
  password = os.environ.get("OMNI_QA_PASSWORD", "")
  if not email or not password:
    print("[warmup] ERRO: defina OMNI_QA_EMAIL e OMNI_QA_PASSWORD no ambiente.")
    return 2

  with sync_playwright() as pw:
    browser = pw.chromium.launch(headless=not args.headed)
    page = browser.new_page()
    page.set_default_timeout(args.timeout_ms)
    login(page, args.base_url, email, password, args.timeout_ms)

    total = len(STATIC_ROUTES)
    slowest: list[tuple[float, str]] = []
    started = time.perf_counter()
    for index, path in enumerate(STATIC_ROUTES, start=1):
      t0 = time.perf_counter()
      try:
        page.goto(f"{args.base_url}{path}", wait_until="domcontentloaded", timeout=args.timeout_ms)
        dt = time.perf_counter() - t0
        slowest.append((dt, path))
        print(f"[warmup] [{index:02d}/{total}] {path}  {dt:6.1f}s", flush=True)
      except Exception as error:  # noqa: BLE001 - rota pode redirecionar; nao trava o warmup
        print(f"[warmup] [{index:02d}/{total}] {path}  FALHOU: {str(error)[:80]}", flush=True)

    browser.close()

  total_s = time.perf_counter() - started
  slowest.sort(reverse=True)
  print(f"\n[warmup] Concluido em {total_s:.0f}s. As rotas mais lentas pra compilar:")
  for dt, path in slowest[:8]:
    print(f"[warmup]   {dt:6.1f}s  {path}")
  print("[warmup] Navegacao no dev agora deve estar instantanea.")
  return 0


if __name__ == "__main__":
  sys.exit(main())
