from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml

from qa_bot.models import Scenario, ScenarioStep


def load_scenario(path: Path) -> Scenario:
  if not path.exists():
    raise FileNotFoundError(f"Cenario nao encontrado: {path}")

  raw_data = yaml.safe_load(path.read_text(encoding="utf-8"))

  if not isinstance(raw_data, dict):
    raise ValueError("Arquivo de cenario invalido. O YAML precisa ser um objeto na raiz.")

  scenario_id = str(raw_data.get("id", path.stem)).strip()
  scenario_name = str(raw_data.get("name", scenario_id)).strip()
  scenario_description = str(raw_data.get("description", "")).strip()
  defaults = raw_data.get("defaults") or {}
  raw_steps = raw_data.get("steps") or []

  if not isinstance(defaults, dict):
    raise ValueError("`defaults` precisa ser um objeto.")

  if not isinstance(raw_steps, list) or not raw_steps:
    raise ValueError("`steps` precisa ser uma lista nao vazia.")

  steps: list[ScenarioStep] = []

  for index, raw_step in enumerate(raw_steps, start=1):
    if not isinstance(raw_step, dict):
      raise ValueError(f"Passo {index} invalido. Cada item de `steps` precisa ser um objeto.")

    if "action" not in raw_step:
      raise ValueError(f"Passo {index} invalido. Falta a chave `action`.")

    steps.append(ScenarioStep.from_dict(raw_step))

  return Scenario(
    id=scenario_id,
    name=scenario_name,
    description=scenario_description,
    defaults=defaults,
    steps=steps
  )
