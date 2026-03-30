from __future__ import annotations

import argparse
import sys
import time
from pathlib import Path

from qa_bot.models import RunConfig


def build_parser() -> argparse.ArgumentParser:
  parser = argparse.ArgumentParser(
    description="Runner generico de cenarios QA com Python + Playwright."
  )
  parser.add_argument(
    "scenario",
    type=Path,
    nargs="?",
    help="Arquivo YAML do cenario a executar. Omitir quando usar --all."
  )
  parser.add_argument(
    "--all",
    action="store_true",
    dest="run_all",
    help="Executa todos os cenarios YAML da pasta scenarios/ em sequencia."
  )
  parser.add_argument(
    "--base-url",
    default="http://localhost:3000",
    help="URL base do app em teste. Ex.: http://localhost:3000"
  )
  parser.add_argument(
    "--browser",
    choices=("chromium", "firefox", "webkit"),
    default="chromium",
    help="Engine do Playwright."
  )
  parser.add_argument(
    "--headed",
    action="store_true",
    help="Abre o navegador visivel para acompanhar o teste."
  )
  parser.add_argument(
    "--slow-mo",
    type=int,
    default=0,
    help="Delay em milissegundos entre comandos do navegador."
  )
  parser.add_argument(
    "--timeout-ms",
    type=int,
    default=8000,
    help="Timeout padrao por acao."
  )
  parser.add_argument(
    "--viewport",
    default="1440x900",
    help="Viewport no formato LARGURAxALTURA. Ex.: 1440x900"
  )
  parser.add_argument(
    "--pause-before-close",
    action="store_true",
    help="Mantem o navegador aberto ate pressionar Enter ao final."
  )
  parser.add_argument(
    "--hold-open-ms",
    type=int,
    default=0,
    help="Mantem o navegador aberto por alguns ms no final, util em CI ou logs."
  )
  parser.add_argument(
    "--artifacts-dir",
    type=Path,
    default=Path("artifacts"),
    help="Diretorio relativo ao qa-bot para screenshots e artefatos."
  )
  return parser


def parse_viewport(raw_value: str) -> tuple[int, int]:
  normalized = raw_value.lower().replace(" ", "")

  if "x" not in normalized:
    raise ValueError("Viewport invalido. Use o formato LARGURAxALTURA.")

  width_text, height_text = normalized.split("x", maxsplit=1)
  width = int(width_text)
  height = int(height_text)

  if width <= 0 or height <= 0:
    raise ValueError("Viewport precisa ter largura e altura positivas.")

  return width, height


def discover_scenarios(base_dir: Path) -> list[Path]:
  scenarios_dir = base_dir / "scenarios"
  if not scenarios_dir.exists():
    return []
  return sorted(scenarios_dir.glob("*.yaml"))


def print_summary(results: list[tuple[str, bool, float]]) -> None:
  total = len(results)
  passed = sum(1 for _, ok, _ in results if ok)
  failed = total - passed
  total_time = sum(t for _, _, t in results)

  print()
  print("=" * 60)
  print(f"  RESUMO QA  —  {passed}/{total} cenarios passaram  ({total_time:.1f}s total)")
  print("=" * 60)
  for name, ok, elapsed in results:
    status = "PASSOU" if ok else "FALHOU"
    icon = "✓" if ok else "✗"
    print(f"  {icon} [{status}]  {name}  ({elapsed:.1f}s)")
  print("=" * 60)
  if failed:
    print(f"\n  {failed} cenario(s) com falha. Verifique os screenshots em artifacts/")
  else:
    print("\n  Todos os cenarios passaram.")
  print()


def main() -> int:
  parser = build_parser()
  args = parser.parse_args()

  if not args.run_all and args.scenario is None:
    parser.error("Informe um arquivo de cenario ou use --all para executar todos.")

  width, height = parse_viewport(args.viewport)

  from qa_bot.loader import load_scenario
  from qa_bot.runner import ScenarioRunner

  base_dir = Path(__file__).resolve().parent

  if args.run_all:
    scenario_paths = discover_scenarios(base_dir)
    if not scenario_paths:
      print("[qa-bot] Nenhum cenario encontrado em scenarios/", file=sys.stderr)
      return 1
    print(f"[qa-bot] Encontrados {len(scenario_paths)} cenarios para executar.")
  else:
    scenario_paths = [args.scenario.resolve()]

  results: list[tuple[str, bool, float]] = []

  for scenario_path in scenario_paths:
    config = RunConfig(
      scenario_path=scenario_path,
      base_url=args.base_url.rstrip("/"),
      browser=args.browser,
      headed=args.headed,
      slow_mo_ms=max(0, args.slow_mo),
      timeout_ms=max(1000, args.timeout_ms),
      viewport_width=width,
      viewport_height=height,
      pause_before_close=args.pause_before_close,
      hold_open_ms=max(0, args.hold_open_ms),
      artifacts_dir=args.artifacts_dir
    )

    try:
      scenario = load_scenario(scenario_path)
    except Exception as error:
      print(f"[qa-bot] Erro ao carregar cenario {scenario_path.name}: {error}", file=sys.stderr)
      results.append((scenario_path.stem, False, 0.0))
      continue

    runner = ScenarioRunner(config)
    started = time.perf_counter()
    exit_code = runner.run(scenario)
    elapsed = time.perf_counter() - started
    results.append((scenario.name, exit_code == 0, elapsed))

    if args.run_all:
      print()

  if args.run_all or len(scenario_paths) > 1:
    print_summary(results)

  any_failed = any(not ok for _, ok, _ in results)
  return 1 if any_failed else 0


if __name__ == "__main__":
  raise SystemExit(main())
