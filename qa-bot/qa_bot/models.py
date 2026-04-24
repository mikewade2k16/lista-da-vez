from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass(slots=True)
class ScenarioStep:
  action: str
  name: str = ""
  target: str | None = None
  testid: str | None = None
  path: str | None = None
  value: Any = None
  milliseconds: int | None = None
  timeout_ms: int | None = None
  args: dict[str, Any] = field(default_factory=dict)

  @classmethod
  def from_dict(cls, data: dict[str, Any]) -> "ScenarioStep":
    reserved_keys = {
      "action",
      "name",
      "target",
      "testid",
      "path",
      "value",
      "milliseconds",
      "timeout_ms"
    }
    extra_args = {key: value for key, value in data.items() if key not in reserved_keys}
    return cls(
      action=str(data["action"]).strip(),
      name=str(data.get("name", "")).strip(),
      target=data.get("target"),
      testid=data.get("testid"),
      path=data.get("path"),
      value=data.get("value"),
      milliseconds=data.get("milliseconds"),
      timeout_ms=data.get("timeout_ms"),
      args=extra_args
    )


@dataclass(slots=True)
class Scenario:
  id: str
  name: str
  description: str = ""
  defaults: dict[str, Any] = field(default_factory=dict)
  steps: list[ScenarioStep] = field(default_factory=list)


@dataclass(slots=True)
class RunConfig:
  scenario_path: Path
  base_url: str
  browser: str
  headed: bool
  slow_mo_ms: int
  timeout_ms: int
  viewport_width: int
  viewport_height: int
  pause_before_close: bool
  hold_open_ms: int
  artifacts_dir: Path
