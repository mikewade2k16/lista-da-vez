from __future__ import annotations

import sys
import time
from datetime import datetime
from pathlib import Path
from typing import Any
from urllib.parse import urljoin

from playwright.sync_api import Locator, Page, TimeoutError as PlaywrightTimeoutError, sync_playwright

from qa_bot.models import RunConfig, Scenario, ScenarioStep


class ScenarioRunError(RuntimeError):
  """Erro levantado quando um passo do cenario falha."""


class ScenarioRunner:
  def __init__(self, config: RunConfig) -> None:
    self.config = config
    self.base_dir = config.scenario_path.resolve().parent.parent
    self.artifacts_dir = (self.base_dir / config.artifacts_dir).resolve()
    self.artifacts_dir.mkdir(parents=True, exist_ok=True)

  def run(self, scenario: Scenario) -> int:
    print(f"[qa-bot] Executando cenario: {scenario.name} ({scenario.id})")
    print(f"[qa-bot] Base URL: {self.config.base_url}")
    started_at = time.perf_counter()

    with sync_playwright() as playwright:
      browser_type = getattr(playwright, self.config.browser)
      browser = browser_type.launch(
        headless=not self.config.headed,
        slow_mo=self.config.slow_mo_ms
      )
      context = browser.new_context(
        viewport={
          "width": self.config.viewport_width,
          "height": self.config.viewport_height
        }
      )
      page = context.new_page()
      page.set_default_timeout(self.config.timeout_ms)
      page.set_default_navigation_timeout(self.config.timeout_ms)

      try:
        for index, step in enumerate(scenario.steps, start=1):
          self._run_step(page, scenario, step, index)

        final_screenshot = self._artifact_path(f"{scenario.id}-final.png")
        page.screenshot(path=str(final_screenshot), full_page=True)
        elapsed = time.perf_counter() - started_at
        print(f"[qa-bot] Cenario concluido em {elapsed:.2f}s")
        print(f"[qa-bot] Screenshot final: {final_screenshot}")
        return 0
      except Exception as error:  # noqa: BLE001 - queremos screenshot em qualquer falha
        failure_screenshot = self._artifact_path(f"{scenario.id}-failure.png")
        page.screenshot(path=str(failure_screenshot), full_page=True)
        print(f"[qa-bot] Falha no cenario: {error}", file=sys.stderr)
        print(f"[qa-bot] Screenshot de falha: {failure_screenshot}", file=sys.stderr)
        return 1
      finally:
        self._hold_browser_open(page)
        browser.close()

  def _run_step(self, page: Page, scenario: Scenario, step: ScenarioStep, index: int) -> None:
    label = step.name or step.action
    print(f"[qa-bot] [{index:02d}] {label}")

    timeout_ms = int(step.timeout_ms or scenario.defaults.get("timeout_ms") or self.config.timeout_ms)
    pause_after_step_ms = int(scenario.defaults.get("pause_after_step_ms") or 0)
    action = step.action.lower().strip()

    try:
      if action == "goto":
        target_url = self._resolve_url(step.path or step.value or "/")
        page.goto(target_url, wait_until="domcontentloaded", timeout=timeout_ms)
      elif action == "reload":
        page.reload(wait_until="domcontentloaded", timeout=timeout_ms)
      elif action == "clear_storage":
        self._clear_storage(page, step, timeout_ms)
      elif action == "click":
        self._locator(page, step).click(timeout=timeout_ms)
      elif action == "fill":
        self._locator(page, step).fill(str(step.value or ""), timeout=timeout_ms)
      elif action == "press":
        self._locator(page, step).press(str(step.value or ""), timeout=timeout_ms)
      elif action == "select":
        self._locator(page, step).select_option(str(step.value or ""), timeout=timeout_ms)
      elif action == "check":
        self._locator(page, step).check(timeout=timeout_ms)
      elif action == "uncheck":
        self._locator(page, step).uncheck(timeout=timeout_ms)
      elif action == "expect_visible":
        self._locator(page, step).wait_for(state="visible", timeout=timeout_ms)
      elif action == "expect_hidden":
        self._locator(page, step).wait_for(state="hidden", timeout=timeout_ms)
      elif action == "expect_text":
        actual_text = self._locator(page, step).inner_text(timeout=timeout_ms)
        expected_text = str(step.value or "")
        if expected_text not in actual_text:
          raise ScenarioRunError(
            f"Texto esperado nao encontrado. Esperado conter '{expected_text}', recebido '{actual_text}'."
          )
      elif action == "expect_url_contains":
        expected_url_fragment = str(step.value or "")
        page.wait_for_url(f"**{expected_url_fragment}*", timeout=timeout_ms)
        if expected_url_fragment not in page.url:
          raise ScenarioRunError(
            f"URL atual nao contem '{expected_url_fragment}'. URL atual: {page.url}"
          )
      elif action == "wait":
        milliseconds = int(step.milliseconds or step.value or 0)
        page.wait_for_timeout(milliseconds)
      elif action == "screenshot":
        screenshot_name = str(step.path or step.value or f"{scenario.id}-{index:02d}.png")
        screenshot_path = self._artifact_path(screenshot_name)
        page.screenshot(path=str(screenshot_path), full_page=bool(step.args.get("full_page", True)))
      else:
        raise ScenarioRunError(f"Acao nao suportada: {step.action}")
    except PlaywrightTimeoutError as error:
      raise ScenarioRunError(f"Timeout na acao '{step.action}': {error}") from error

    if pause_after_step_ms > 0:
      page.wait_for_timeout(pause_after_step_ms)

  def _clear_storage(self, page: Page, step: ScenarioStep, timeout_ms: int) -> None:
    bootstrap_path = step.path or "/"
    storage_scope = str(step.args.get("storage", "both")).strip().lower()
    reload_page = bool(step.args.get("reload", True))

    if page.url == "about:blank":
      page.goto(self._resolve_url(bootstrap_path), wait_until="domcontentloaded", timeout=timeout_ms)

    page.evaluate(
      """
      (scope) => {
        if (scope === "local" || scope === "both") {
          window.localStorage.clear()
        }
        if (scope === "session" || scope === "both") {
          window.sessionStorage.clear()
        }
      }
      """,
      storage_scope
    )

    if reload_page:
      page.reload(wait_until="domcontentloaded", timeout=timeout_ms)

  def _locator(self, page: Page, step: ScenarioStep) -> Locator:
    if step.testid:
      return page.get_by_test_id(step.testid)

    if step.target:
      return page.locator(step.target)

    raise ScenarioRunError(f"O passo '{step.action}' precisa de `testid` ou `target`.")

  def _resolve_url(self, path_or_url: Any) -> str:
    value = str(path_or_url or "").strip()

    if not value:
      return self.config.base_url

    if value.startswith("http://") or value.startswith("https://"):
      return value

    return urljoin(f"{self.config.base_url}/", value.lstrip("/"))

  def _artifact_path(self, filename: str) -> Path:
    safe_name = filename.replace("\\", "-").replace("/", "-")
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")

    if "." in safe_name:
      stem, suffix = safe_name.rsplit(".", maxsplit=1)
      safe_name = f"{timestamp}-{stem}.{suffix}"
    else:
      safe_name = f"{timestamp}-{safe_name}"

    return self.artifacts_dir / safe_name

  def _hold_browser_open(self, page: Page) -> None:
    if self.config.pause_before_close and sys.stdin.isatty():
      input("[qa-bot] Pressione Enter para fechar o navegador...")
      return

    if self.config.hold_open_ms > 0:
      page.wait_for_timeout(self.config.hold_open_ms)
