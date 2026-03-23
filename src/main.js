import { renderHomePage } from "./pages/home-page.js";
import { renderSinglePickerSelectionMarkup } from "./components/catalog-picker.js";
import { exportReportCsv, exportReportPdf } from "./services/report-export.js";
import { loadQueueState, saveQueueState } from "./services/queue-service.js";
import { createAppStore } from "./store/app-store.js";
import {
  canAccessReports,
  canManageCampaigns,
  canManageConsultants,
  canManageSettings,
  canManageStores
} from "./utils/permissions.js";
import { buildReportData } from "./utils/reports.js";
import { formatDuration } from "./utils/time.js";

const appRoot = document.querySelector("#app");
const store = createAppStore();
const reportUiState = {
  filtersExpanded: false,
  expandedGroup: null
};

function toggleFilterValue(list, value) {
  const current = Array.isArray(list) ? list : [];

  if (current.includes(value)) {
    return current.filter((item) => item !== value);
  }

  return [...current, value];
}

function clearReportFilter(filterId, filterValue = "") {
  const state = store.getState();
  const currentValue = state.reportFilters?.[filterId];

  if (Array.isArray(currentValue)) {
    const nextValue = filterValue ? currentValue.filter((item) => item !== filterValue) : [];
    store.updateReportFilter(filterId, nextValue);
    return;
  }

  store.updateReportFilter(filterId, "");
}

function getCurrentProfile() {
  const state = store.getState();

  return state.profiles.find((profile) => profile.id === state.activeProfileId) || state.profiles[0] || null;
}

function getCurrentRole() {
  return getCurrentProfile()?.role || "consultant";
}

function ensureSettingsAccess() {
  if (canManageSettings(getCurrentRole())) {
    return true;
  }

  window.alert("Apenas admin pode alterar configuracoes.");
  return false;
}

function ensureConsultantCrudAccess() {
  if (canManageConsultants(getCurrentRole())) {
    return true;
  }

  window.alert("Apenas admin pode gerir consultores.");
  return false;
}

function ensureCampaignCrudAccess() {
  if (canManageCampaigns(getCurrentRole())) {
    return true;
  }

  window.alert("Apenas admin pode gerir campanhas.");
  return false;
}

function ensureReportsAccess() {
  if (canAccessReports(getCurrentRole())) {
    return true;
  }

  window.alert("Seu perfil nao tem acesso aos relatorios.");
  return false;
}

function ensureStoreCrudAccess() {
  if (canManageStores(getCurrentRole())) {
    return true;
  }

  window.alert("Apenas admin pode gerir lojas.");
  return false;
}

function buildCurrentReportData() {
  const state = store.getState();

  return buildReportData({
    history: state.serviceHistory,
    roster: state.roster,
    visitReasonOptions: state.visitReasonOptions,
    customerSourceOptions: state.customerSourceOptions,
    filters: state.reportFilters
  });
}

function renderApp() {
  if (!appRoot) {
    return;
  }

  appRoot.innerHTML = renderHomePage({
    ...store.getState(),
    reportUiState
  });
  syncFinishModalVisibility();
}

function handleClick(event) {
  const actionElement = event.target.closest("[data-action]");

  if (!actionElement) {
    return;
  }

  const { action, personId, workspaceId, productId, optionGroup, optionId, consultantId, templateId } = actionElement.dataset;

  if (action === "set-workspace" && workspaceId) {
    store.setWorkspace(workspaceId);
  }

  if (action === "set-active-store" && actionElement.dataset.storeId) {
    store.setActiveStore(actionElement.dataset.storeId);
  }

  if (action === "select-consultant" && personId) {
    store.setSelectedConsultant(personId);
  }

  if (action === "add-to-queue" && personId) {
    store.addToQueue(personId);
  }

  if (action === "pause-employee" && personId) {
    const reason = window.prompt("Motivo da pausa do consultor:");

    if (reason) {
      store.pauseEmployee(personId, reason);
    }
  }

  if (action === "resume-employee" && personId) {
    store.resumeEmployee(personId);
  }

  if (action === "start-service") {
    store.startService(personId ?? null);
  }

  if (action === "open-finish-modal" && personId) {
    store.openFinishModal(personId);
  }

  if (action === "close-finish-modal") {
    store.closeFinishModal();
  }

  if (action === "collapse-toggle") {
    const body = actionElement.closest(".finish-form__section--collapse")?.querySelector(".finish-form__collapse-body");
    if (!(body instanceof HTMLElement)) return;
    body.hidden = !body.hidden;
    actionElement.setAttribute("aria-expanded", String(!body.hidden));
    return;
  }

  // Product pick widget actions — scoped to finish-modal only
  const PRODUCT_PICK_ACTIONS = new Set(["product-pick-toggle", "product-pick-select", "product-pick-custom-toggle", "product-pick-custom-cancel", "product-pick-custom-add", "remove-product", "product-pick-none-toggle"]);
  if (PRODUCT_PICK_ACTIONS.has(action) && actionElement.closest(".finish-modal")) {
    const modal = appRoot?.querySelector(".finish-modal");
    if (modal) handleProductPickAction(modal, actionElement);
    return;
  }

  const OPTION_PICK_ACTIONS = new Set(["option-pick-toggle", "option-pick-select", "option-pick-clear", "pick-detail-toggle", "pick-detail-close"]);
  if (OPTION_PICK_ACTIONS.has(action) && actionElement.closest(".finish-modal")) {
    const modal = appRoot?.querySelector(".finish-modal");
    if (modal) handleOptionPickAction(modal, actionElement);
    return;
  }

  if (action === "set-settings-tab") {
    const tabId = actionElement.dataset.tab;
    const panel = actionElement.closest(".admin-panel");
    if (!panel || !tabId) return;
    panel.querySelectorAll(".settings-tabs__btn").forEach((btn) => {
      btn.classList.toggle("is-active", /** @type {HTMLElement} */ (btn).dataset.tab === tabId);
    });
    panel.querySelectorAll("[data-tab-panel]").forEach((p) => {
      /** @type {HTMLElement} */ (p).hidden = /** @type {HTMLElement} */ (p).dataset.tabPanel !== tabId;
    });
    return;
  }

  if (action === "toggle-report-filters") {
    reportUiState.filtersExpanded = !reportUiState.filtersExpanded;

    if (!reportUiState.filtersExpanded) {
      reportUiState.expandedGroup = null;
    }

    renderApp();
    return;
  }

  if (action === "toggle-report-filter-group" && actionElement.dataset.filterGroup) {
    reportUiState.filtersExpanded = true;
    reportUiState.expandedGroup =
      reportUiState.expandedGroup === actionElement.dataset.filterGroup ? null : actionElement.dataset.filterGroup;
    renderApp();
    return;
  }

  if (action === "toggle-report-filter-value" && actionElement.dataset.filterId && actionElement.dataset.filterValue) {
    if (!ensureReportsAccess()) {
      return;
    }

    const filterId = actionElement.dataset.filterId;
    const filterValue = actionElement.dataset.filterValue;
    const currentValue = store.getState().reportFilters?.[filterId];

    if (!Array.isArray(currentValue)) {
      return;
    }

    store.updateReportFilter(filterId, toggleFilterValue(currentValue, filterValue));
    return;
  }

  if (action === "clear-report-filter" && actionElement.dataset.filterId) {
    if (!ensureReportsAccess()) {
      return;
    }

    clearReportFilter(actionElement.dataset.filterId, actionElement.dataset.filterValue || "");
    return;
  }

  // Close open product dropdowns when clicking outside them
  if (appRoot) {
    const modal = appRoot.querySelector(".finish-modal");
    if (modal && !actionElement.closest(".product-pick__dropdown")) {
      modal.querySelectorAll(".product-pick__dropdown.is-open").forEach((d) => {
        if (!d.closest(".product-pick")?.contains(actionElement)) d.classList.remove("is-open");
      });
    }
  }

  if (action === "remove-option") {
    if (!ensureSettingsAccess()) {
      return;
    }

    if (optionGroup === "visit-reason" && optionId) {
      store.removeVisitReasonOption(optionId);
    }

    if (optionGroup === "customer-source" && optionId) {
      store.removeCustomerSourceOption(optionId);
    }

    if (optionGroup === "profession" && optionId) {
      store.removeProfessionOption(optionId);
    }
  }

  if (action === "remove-product" && productId) {
    if (!ensureSettingsAccess()) {
      return;
    }

    store.removeCatalogProduct(productId);
  }

  if (action === "apply-operation-template" && templateId) {
    if (!ensureSettingsAccess()) {
      return;
    }

    store.applyOperationTemplate(templateId);
  }

  if (action === "archive-consultant" && consultantId) {
    if (!ensureConsultantCrudAccess()) {
      return;
    }

    const result = store.archiveConsultantProfile(consultantId);

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel arquivar consultor.");
    }
  }

  if (action === "reset-report-filters") {
    if (!ensureReportsAccess()) {
      return;
    }

    reportUiState.filtersExpanded = false;
    reportUiState.expandedGroup = null;
    store.resetReportFilters();
  }

  if (action === "export-report-csv") {
    if (!ensureReportsAccess()) {
      return;
    }

    exportReportCsv(buildCurrentReportData());
  }

  if (action === "export-report-pdf") {
    if (!ensureReportsAccess()) {
      return;
    }

    exportReportPdf(buildCurrentReportData());
  }

  if (action === "remove-campaign" && actionElement.dataset.campaignId) {
    if (!ensureCampaignCrudAccess()) {
      return;
    }

    store.removeCampaign(actionElement.dataset.campaignId);
  }

  if (action === "archive-store" && actionElement.dataset.storeId) {
    if (!ensureStoreCrudAccess()) {
      return;
    }

    const result = store.archiveStore(actionElement.dataset.storeId);

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel arquivar loja.");
    }
  }
}

function handleChange(event) {
  const target = event.target;

  if (!(target instanceof Element)) {
    return;
  }

  const controlTarget = target.closest("input, select, textarea");

  if (!(controlTarget instanceof HTMLInputElement) && !(controlTarget instanceof HTMLSelectElement) && !(controlTarget instanceof HTMLTextAreaElement)) {
    return;
  }

  const actionTarget = controlTarget.closest("[data-action]");

  if (!(actionTarget instanceof HTMLInputElement) && !(actionTarget instanceof HTMLSelectElement) && !(actionTarget instanceof HTMLTextAreaElement)) {
    if (controlTarget.closest(".finish-modal")) {
      syncFinishModalVisibility();
    }

    return;
  }

  if (actionTarget.dataset.action === "set-active-profile") {
    store.setActiveProfile(actionTarget.value);
    return;
  }

  if (actionTarget.dataset.action === "set-active-store") {
    store.setActiveStore(actionTarget.value);
    return;
  }

  if (actionTarget.dataset.action === "set-report-filter" && actionTarget.dataset.filterId) {
    if (!ensureReportsAccess()) {
      renderApp();
      return;
    }

    store.updateReportFilter(actionTarget.dataset.filterId, actionTarget.value);
    return;
  }

  if (actionTarget.dataset.action === "set-simulation-value") {
    store.setConsultantSimulationAdditionalSales(actionTarget.value);
  }

  if (actionTarget.dataset.action === "set-setting" && actionTarget.dataset.settingId) {
    if (!ensureSettingsAccess()) {
      renderApp();
      return;
    }

    const settingId = actionTarget.dataset.settingId;
    const value =
      actionTarget.type === "checkbox"
        ? actionTarget.checked
        : ["maxConcurrentServices", "timingFastCloseMinutes", "timingLongServiceMinutes", "timingLowSaleAmount"].includes(
            settingId
          )
          ? Math.max(1, Number(actionTarget.value) || 1)
          : actionTarget.value;
    store.updateSetting(settingId, value);
  }

  if (actionTarget.dataset.action === "set-modal-config" && actionTarget.dataset.configKey) {
    if (!ensureSettingsAccess()) {
      renderApp();
      return;
    }

    const value = actionTarget.type === "checkbox" ? actionTarget.checked : actionTarget.value;
    store.updateModalConfig(actionTarget.dataset.configKey, value);
  }

  if (actionTarget.dataset.action === "update-product" && actionTarget.dataset.productId && actionTarget.dataset.productField) {
    if (!ensureSettingsAccess()) {
      renderApp();
      return;
    }

    const field = actionTarget.dataset.productField;
    const rawValue = actionTarget.value;
    const patch =
      field === "basePrice"
        ? { [field]: Math.max(0, Number(rawValue) || 0) }
        : { [field]: rawValue };

    store.updateCatalogProduct(actionTarget.dataset.productId, patch);
  }

  if (controlTarget.closest(".finish-modal")) {
    syncFinishModalVisibility();
  }
}

// ── Product Pick helpers ─────────────────────────────────────────────────────

function handleInput(event) {
  const target = event.target;

  if (!(target instanceof HTMLInputElement) || !target.hasAttribute("data-picker-search-input")) {
    return;
  }

  const dropdown = target.closest(".product-pick__dropdown");

  if (!(dropdown instanceof HTMLElement)) {
    return;
  }

  applyPickerSearch(dropdown, target.value);
}

function handleKeydown(event) {
  const target = event.target;

  if (!(target instanceof HTMLInputElement) || !target.hasAttribute("data-picker-search-input")) {
    return;
  }

  if (event.key === "Enter") {
    event.preventDefault();
  }
}

function formatPricePt(value) {
  return Number(value || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

function escHtml(s) {
  return String(s || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function normalizePickerSearchText(value) {
  return String(value || "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();
}

function setPickerItemHidden(item, searchHidden) {
  const isFilterHidden = item.dataset.filterHidden === "true";
  item.dataset.searchHidden = searchHidden ? "true" : "false";
  item.hidden = isFilterHidden || searchHidden;
}

function applyPickerSearch(dropdown, rawQuery) {
  const query = normalizePickerSearchText(rawQuery);

  dropdown.querySelectorAll("[data-picker-search-target]").forEach((item) => {
    if (!(item instanceof HTMLElement)) {
      return;
    }

    if (item.dataset.pickerSearchStatic === "true") {
      item.hidden = item.dataset.filterHidden === "true";
      item.dataset.searchHidden = "false";
      return;
    }

    const searchTarget = normalizePickerSearchText(item.dataset.pickerSearchTarget || item.textContent || "");
    setPickerItemHidden(item, Boolean(query) && !searchTarget.includes(query));
  });
}

function resetPickerSearch(widget) {
  const dropdown = widget.querySelector(".product-pick__dropdown");

  if (!(dropdown instanceof HTMLElement)) {
    return;
  }

  const searchInput = dropdown.querySelector("[data-picker-search-input]");

  if (searchInput instanceof HTMLInputElement) {
    searchInput.value = "";
  }

  applyPickerSearch(dropdown, "");
}

function focusPickerSearch(widget) {
  const searchInput = widget.querySelector("[data-picker-search-input]");

  if (searchInput instanceof HTMLInputElement) {
    window.requestAnimationFrame(() => searchInput.focus());
  }
}

function closePickerDropdown(widget) {
  const dropdown = widget.querySelector(".product-pick__dropdown");

  if (dropdown instanceof HTMLElement) {
    dropdown.classList.remove("is-open");
    resetPickerSearch(widget);
  }
}

function togglePickerDropdown(modal, widget) {
  const dropdown = widget.querySelector(".product-pick__dropdown");

  if (!(dropdown instanceof HTMLElement)) {
    return;
  }

  const shouldOpen = !dropdown.classList.contains("is-open");

  modal.querySelectorAll(".product-pick").forEach((pickWidget) => {
    if (!(pickWidget instanceof HTMLElement) || pickWidget === widget) {
      return;
    }

    closePickerDropdown(pickWidget);
  });

  if (!shouldOpen) {
    closePickerDropdown(widget);
    return;
  }

  resetPickerSearch(widget);
  dropdown.classList.add("is-open");
  focusPickerSearch(widget);
}

function buildProductTagHtml(product) {
  return `
    <span class="product-pick__tag" data-pick-entry="${escHtml(product.id)}">
      ${escHtml(product.name)}
      <button type="button" class="product-pick__tag-remove"
        data-action="remove-product" data-pick="seen" data-product-id="${escHtml(product.id)}"
        title="Remover"><span class="material-icons-round">close</span></button>
    </span>
    <input type="hidden" name="products-seen" data-pick-input="${escHtml(product.id)}" value="${escHtml(JSON.stringify(product))}">
  `;
}

function buildClosedItemHtml(product) {
  return `
    <div class="product-pick__closed-item" data-pick-entry="${escHtml(product.id)}">
      <span class="product-pick__closed-name">${escHtml(product.name)}${product.code ? ` <small>(${escHtml(product.code)})</small>` : ""}</span>
      <span class="product-pick__closed-price">${formatPricePt(product.price)}</span>
      <button type="button" class="product-pick__tag-remove"
        data-action="remove-product" data-pick="closed" data-product-id="${escHtml(product.id)}"
        title="Remover"><span class="material-icons-round">close</span></button>
    </div>
    <input type="hidden" name="products-closed" data-pick-input="${escHtml(product.id)}" value="${escHtml(JSON.stringify(product))}">
  `;
}

function recalcClosedTotal(widget) {
  let total = 0;

  widget.querySelectorAll('input[name="products-closed"]').forEach((input) => {
    try { total += Number(JSON.parse(input.value)?.price || 0); } catch { /* noop */ }
  });

  const totalEl = widget.querySelector("[data-pick-total]");
  const totalWrap = totalEl?.closest(".product-pick__total");

  if (totalEl) totalEl.textContent = formatPricePt(total);
  if (totalWrap instanceof HTMLElement) totalWrap.hidden = total === 0;
}

function syncOptionPickDetail(widget) {
  const detailField = widget.querySelector("[data-option-detail]");
  const detailInput = detailField?.querySelector("input");
  const hasSelectedValue = widget.querySelector("[data-option-selection-input]") instanceof HTMLInputElement;
  const hasNoneValue = widget.querySelector("[data-option-none-input]") instanceof HTMLInputElement;
  const shouldShow = hasSelectedValue && !hasNoneValue;

  if (detailField instanceof HTMLElement) {
    detailField.hidden = !shouldShow;
    if (!shouldShow) detailField.classList.remove("is-open");
  }

  if (detailInput instanceof HTMLInputElement) {
    if (!shouldShow) detailInput.value = "";
  }
}

function applyOptionSelection(widget, { pick, label, value = "", isNone = false }) {
  const selection = widget.querySelector("[data-option-selection]");

  if (!(selection instanceof HTMLElement)) {
    return;
  }

  const previousValue = widget.querySelector("[data-option-selection-input]")?.getAttribute("value") || "";
  const previousWasNone = widget.querySelector("[data-option-none-input]") instanceof HTMLInputElement;
  const inputName = widget.dataset.inputName || "";
  const noneName = widget.dataset.noneName || "";
  const detailInput = widget.querySelector("[data-option-detail] input");

  selection.innerHTML = label
    ? renderSinglePickerSelectionMarkup({
        pick,
        label,
        inputName,
        value: isNone ? "" : value,
        noneName,
        isNone
      })
    : "";

  if (detailInput instanceof HTMLInputElement && (previousValue !== value || previousWasNone !== isNone)) {
    detailInput.value = "";
  }

  syncOptionPickDetail(widget);
}

function clearOptionPick(widget) {
  applyOptionSelection(widget, {
    pick: widget.dataset.optionPick || "",
    label: "",
    value: "",
    isNone: false
  });
}

function syncVisitReasonPickerOutcome(modal, outcome) {
  const widget = modal.querySelector('[data-option-pick="visit-reason"]');

  if (!(widget instanceof HTMLElement)) {
    return;
  }

  widget.querySelectorAll("[data-option-id]").forEach((option) => {
    if (!(option instanceof HTMLElement)) {
      return;
    }

    const outcomes = String(option.dataset.outcomes || "").split(" ").filter(Boolean);
    const isVisible = outcomes.length === 0 || !outcome || outcomes.includes(outcome);

    option.dataset.filterHidden = isVisible ? "false" : "true";
    option.hidden = option.dataset.searchHidden === "true" || !isVisible;
  });

  const selectedInput = widget.querySelector("[data-option-selection-input]");

  if (selectedInput instanceof HTMLInputElement) {
    const selectedOption = widget.querySelector(`[data-option-id="${selectedInput.value}"]`);

    if (!(selectedOption instanceof HTMLElement) || selectedOption.dataset.filterHidden === "true") {
      clearOptionPick(widget);
    }
  }

  const dropdown = widget.querySelector(".product-pick__dropdown");
  const currentSearch = widget.querySelector("[data-picker-search-input]");

  if (dropdown instanceof HTMLElement && currentSearch instanceof HTMLInputElement) {
    applyPickerSearch(dropdown, currentSearch.value);
  }
}

function handleProductPickAction(modal, actionElement) {
  const { action, pick } = actionElement.dataset;
  const widget = pick ? modal.querySelector(`[data-product-pick="${pick}"]`) : null;

  if (!(widget instanceof HTMLElement)) {
    return;
  }

  if (action === "product-pick-toggle") {
    const noneBtn = widget.querySelector(".product-pick__none-btn");

    if (noneBtn?.classList.contains("is-active")) {
      noneBtn.classList.remove("is-active");
      widget.querySelector('input[name="products-seen-none"]')?.remove();
    }

    togglePickerDropdown(modal, widget);
    return;
  }

  if (action === "product-pick-select") {
    closePickerDropdown(widget);

    const { productId, productName, productPrice } = actionElement.dataset;

    if (widget.querySelector(`[data-pick-entry="${productId}"]`)) {
      return;
    }

    const product = { id: productId, name: productName, price: Number(productPrice) };

    if (pick === "seen") {
      const tagsEl = widget.querySelector("[data-pick-tags]");

      if (tagsEl) {
        tagsEl.insertAdjacentHTML("beforeend", buildProductTagHtml(product));
      }

      widget.querySelector(".product-pick__none-btn")?.classList.remove("is-active");
      widget.querySelector('input[name="products-seen-none"]')?.remove();
    } else {
      const selectedEl = widget.querySelector("[data-pick-selected]");

      if (selectedEl) {
        selectedEl.insertAdjacentHTML("beforeend", buildClosedItemHtml(product));
      }

      recalcClosedTotal(widget);
    }

    return;
  }

  if (action === "product-pick-custom-toggle") {
    closePickerDropdown(widget);
    const form = widget.querySelector(".product-pick__custom-form");

    if (form instanceof HTMLElement) {
      form.classList.toggle("is-open");
    }

    return;
  }

  if (action === "product-pick-custom-cancel") {
    const form = widget.querySelector(".product-pick__custom-form");

    if (form instanceof HTMLElement) {
      form.classList.remove("is-open");
      form.querySelectorAll("input").forEach((input) => {
        input.value = "";
      });
    }

    return;
  }

  if (action === "product-pick-custom-add") {
    const form = widget.querySelector(".product-pick__custom-form");

    if (!(form instanceof HTMLElement)) {
      return;
    }

    const code = /** @type {HTMLInputElement|null} */ (form.querySelector('[data-custom="code"]'))?.value.trim() || "";
    const name = /** @type {HTMLInputElement|null} */ (form.querySelector('[data-custom="name"]'))?.value.trim() || "";
    const price = Number((/** @type {HTMLInputElement|null} */ (form.querySelector('[data-custom="price"]')))?.value || 0);

    if (!name) {
      window.alert("Informe o nome do produto.");
      return;
    }

    const product = { id: `__custom__${Date.now()}`, name, price, code, isCustom: true };

    if (pick === "seen") {
      const tagsEl = widget.querySelector("[data-pick-tags]");

      if (tagsEl) {
        tagsEl.insertAdjacentHTML("beforeend", buildProductTagHtml(product));
      }

      widget.querySelector(".product-pick__none-btn")?.classList.remove("is-active");
      widget.querySelector('input[name="products-seen-none"]')?.remove();
    } else {
      const selectedEl = widget.querySelector("[data-pick-selected]");

      if (selectedEl) {
        selectedEl.insertAdjacentHTML("beforeend", buildClosedItemHtml(product));
      }

      recalcClosedTotal(widget);
    }

    form.classList.remove("is-open");
    form.querySelectorAll("input").forEach((input) => {
      input.value = "";
    });
    return;
  }

  if (action === "product-pick-none-toggle") {
    const noneBtn = widget.querySelector(".product-pick__none-btn");
    const isActive = noneBtn?.classList.contains("is-active");

    if (isActive) {
      noneBtn?.classList.remove("is-active");
      widget.querySelector('input[name="products-seen-none"]')?.remove();
      return;
    }

    widget.querySelectorAll("[data-pick-entry]").forEach((item) => item.remove());
    widget.querySelectorAll('input[name="products-seen"]').forEach((item) => item.remove());
    closePickerDropdown(widget);
    noneBtn?.classList.add("is-active");
    const noneInput = document.createElement("input");
    noneInput.type = "hidden";
    noneInput.name = "products-seen-none";
    noneInput.value = "1";
    widget.appendChild(noneInput);
    return;
  }

  if (action === "remove-product") {
    const { productId } = actionElement.dataset;
    widget.querySelector(`[data-pick-entry="${productId}"]`)?.remove();
    widget.querySelector(`[data-pick-input="${productId}"]`)?.remove();

    if (pick === "closed") {
      recalcClosedTotal(widget);
    }
  }
}

function handleOptionPickAction(modal, actionElement) {
  const { action, pick, optionId, optionLabel, optionNone } = actionElement.dataset;
  const widget = pick ? modal.querySelector(`[data-option-pick="${pick}"]`) : null;

  if (!(widget instanceof HTMLElement)) {
    return;
  }

  if (action === "option-pick-toggle") {
    togglePickerDropdown(modal, widget);
    return;
  }

  if (action === "option-pick-select") {
    applyOptionSelection(widget, {
      pick,
      label: optionLabel || "",
      value: optionNone === "true" ? "" : optionId || "",
      isNone: optionNone === "true"
    });
    closePickerDropdown(widget);
    return;
  }

  if (action === "option-pick-clear") {
    clearOptionPick(widget);
    return;
  }

  if (action === "pick-detail-toggle") {
    const detailWrap = widget?.querySelector("[data-option-detail]");
    if (detailWrap instanceof HTMLElement) {
      detailWrap.classList.add("is-open");
      detailWrap.querySelector("input")?.focus();
    }
    return;
  }

  if (action === "pick-detail-close") {
    const detailWrap = widget?.querySelector("[data-option-detail]");
    if (detailWrap instanceof HTMLElement) {
      detailWrap.classList.remove("is-open");
      const input = detailWrap.querySelector("input");
      if (input instanceof HTMLInputElement) input.value = "";
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────

function syncFinishModalVisibility() {
  if (!appRoot) {
    return;
  }

  const modal = appRoot.querySelector(".finish-modal");

  if (!modal) {
    return;
  }

  const outcome = modal.querySelector('input[name="finish-outcome"]:checked')?.value || "";
  const showClosedFields = outcome === "reserva" || outcome === "compra";

  // Gift checkbox
  const giftField = modal.querySelector('[data-field="gift"]');
  if (giftField instanceof HTMLElement) {
    giftField.hidden = !showClosedFields;
    const giftInput = giftField.querySelector('input[name="is-gift"]');
    if (giftInput instanceof HTMLInputElement) {
      giftInput.disabled = !showClosedFields;
      if (!showClosedFields) giftInput.checked = false;
    }
  }

  // Product-closed section
  const productClosedField = modal.querySelector('[data-field="product-closed"]');
  if (productClosedField instanceof HTMLElement) {
    productClosedField.hidden = !showClosedFields;
  }

  // Update label (Produto comprado vs Produto reservado)
  const productClosedLabelEl = modal.querySelector('[data-field="product-closed"] .finish-form__label');
  if (productClosedLabelEl instanceof HTMLElement) {
    const labelKey = `label${outcome.charAt(0).toUpperCase()}${outcome.slice(1)}`;
    const dynamic = productClosedLabelEl.dataset[labelKey];
    if (dynamic) productClosedLabelEl.textContent = dynamic;
  }

  syncVisitReasonPickerOutcome(modal, outcome);

  modal.querySelectorAll("[data-option-pick]").forEach((widget) => {
    if (widget instanceof HTMLElement) {
      syncOptionPickDetail(widget);
    }
  });

}

function isEditingFieldActive() {
  const activeElement = document.activeElement;

  if (!(activeElement instanceof Element)) {
    return false;
  }

  return (
    activeElement instanceof HTMLInputElement ||
    activeElement instanceof HTMLSelectElement ||
    activeElement instanceof HTMLTextAreaElement
  );
}

function parseCampaignPayload(formData) {
  return {
    name: String(formData.get("name") || "").trim(),
    description: String(formData.get("description") || "").trim(),
    startsAt: String(formData.get("startsAt") || "").trim(),
    endsAt: String(formData.get("endsAt") || "").trim(),
    targetOutcome: String(formData.get("targetOutcome") || "compra-reserva"),
    existingCustomerFilter: String(formData.get("existingCustomerFilter") || "all"),
    minSaleAmount: Math.max(0, Number(formData.get("minSaleAmount") || 0)),
    maxServiceMinutes: Math.max(0, Number(formData.get("maxServiceMinutes") || 0)),
    bonusFixed: Math.max(0, Number(formData.get("bonusFixed") || 0)),
    bonusRate: Math.max(0, Number(formData.get("bonusRate") || 0)),
    isActive: formData.has("isActive"),
    queueJumpOnly: formData.has("queueJumpOnly"),
    sourceIds: formData.getAll("sourceIds").map((value) => String(value)),
    reasonIds: formData.getAll("reasonIds").map((value) => String(value))
  };
}

function handleSubmit(event) {
  const form = event.target;

  if (!(form instanceof HTMLFormElement)) {
    return;
  }

  if (form.dataset.action === "add-option") {
    event.preventDefault();
    if (!ensureSettingsAccess()) {
      return;
    }

    const optionGroup = form.dataset.optionGroup;
    const label = String(new FormData(form).get("label") || "").trim();

    if (!label) {
      return;
    }

    if (optionGroup === "visit-reason") {
      store.addVisitReasonOption(label);
    }

    if (optionGroup === "customer-source") {
      store.addCustomerSourceOption(label);
    }

    if (optionGroup === "profession") {
      store.addProfessionOption(label);
    }

    form.reset();
    return;
  }

  if (form.dataset.action === "update-option") {
    event.preventDefault();
    if (!ensureSettingsAccess()) {
      return;
    }

    const optionGroup = form.dataset.optionGroup;
    const optionId = form.dataset.optionId;
    const label = String(new FormData(form).get("label") || "").trim();

    if (!label || !optionId) {
      return;
    }

    if (optionGroup === "visit-reason") {
      store.updateVisitReasonOption(optionId, label);
    }

    if (optionGroup === "customer-source") {
      store.updateCustomerSourceOption(optionId, label);
    }

    if (optionGroup === "profession") {
      store.updateProfessionOption(optionId, label);
    }

    return;
  }

  if (form.dataset.action === "add-product") {
    event.preventDefault();
    if (!ensureSettingsAccess()) {
      return;
    }

    const formData = new FormData(form);
    const name = String(formData.get("name") || "").trim();
    const category = String(formData.get("category") || "").trim();
    const basePrice = Number(formData.get("basePrice") || 0);

    if (!name) {
      return;
    }

    store.addCatalogProduct(name, category, basePrice);
    form.reset();
    return;
  }

  if (form.dataset.action === "add-consultant") {
    event.preventDefault();

    if (!ensureConsultantCrudAccess()) {
      return;
    }

    const formData = new FormData(form);
    const result = store.createConsultantProfile({
      name: String(formData.get("name") || ""),
      role: String(formData.get("role") || ""),
      color: String(formData.get("color") || ""),
      monthlyGoal: Number(formData.get("monthlyGoal") || 0),
      commissionRate: Number(formData.get("commissionRate") || 0)
    });

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel criar consultor.");
      return;
    }

    form.reset();
    return;
  }

  if (form.dataset.action === "update-consultant" && form.dataset.consultantId) {
    event.preventDefault();

    if (!ensureConsultantCrudAccess()) {
      return;
    }

    const formData = new FormData(form);
    const result = store.updateConsultantProfile(form.dataset.consultantId, {
      name: String(formData.get("name") || ""),
      role: String(formData.get("role") || ""),
      color: String(formData.get("color") || ""),
      monthlyGoal: Number(formData.get("monthlyGoal") || 0),
      commissionRate: Number(formData.get("commissionRate") || 0)
    });

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel atualizar consultor.");
    }

    return;
  }

  if (form.dataset.action === "add-store") {
    event.preventDefault();

    if (!ensureStoreCrudAccess()) {
      return;
    }

    const formData = new FormData(form);
    const result = store.createStore({
      name: String(formData.get("name") || ""),
      code: String(formData.get("code") || ""),
      city: String(formData.get("city") || ""),
      cloneActiveRoster: formData.has("clone-active-roster")
    });

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel criar loja.");
      return;
    }

    form.reset();
    return;
  }

  if (form.dataset.action === "update-store" && form.dataset.storeId) {
    event.preventDefault();

    if (!ensureStoreCrudAccess()) {
      return;
    }

    const formData = new FormData(form);
    const result = store.updateStore(form.dataset.storeId, {
      name: String(formData.get("name") || ""),
      code: String(formData.get("code") || ""),
      city: String(formData.get("city") || "")
    });

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel atualizar loja.");
    }

    return;
  }

  if (form.dataset.action === "add-campaign") {
    event.preventDefault();

    if (!ensureCampaignCrudAccess()) {
      return;
    }

    const payload = parseCampaignPayload(new FormData(form));
    const result = store.createCampaign(payload);

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel criar campanha.");
      return;
    }

    form.reset();
    return;
  }

  if (form.dataset.action === "update-campaign" && form.dataset.campaignId) {
    event.preventDefault();

    if (!ensureCampaignCrudAccess()) {
      return;
    }

    const payload = parseCampaignPayload(new FormData(form));
    const result = store.updateCampaign(form.dataset.campaignId, payload);

    if (!result?.ok) {
      window.alert(result?.message || "Nao foi possivel atualizar campanha.");
    }

    return;
  }

  if (form.dataset.action !== "finish-service-form") {
    return;
  }

  event.preventDefault();

  const state = store.getState();
  const modalConfig = state.modalConfig;
  const personId = form.dataset.personId;
  const formData = new FormData(form);
  const outcome = String(formData.get("finish-outcome") || "");

  const parseProducts = (/** @type {FormDataEntryValue[]} */ entries) =>
    entries.map((v) => { try { return JSON.parse(String(v)); } catch { return null; } }).filter(Boolean);

  const productsSeen = parseProducts(formData.getAll("products-seen"));
  const productsClosed = (outcome === "compra" || outcome === "reserva")
    ? parseProducts(formData.getAll("products-closed"))
    : [];
  const saleAmount = productsClosed.reduce((sum, p) => sum + (Number(p.price) || 0), 0);

  const productSeen = productsSeen[0]?.name || "";
  const productClosed = productsClosed[0]?.name || "";

  const visitReasons = formData.getAll("visit-reasons").map((value) => String(value));
  const visitReasonsNotInformed = formData.has("visit-reasons-none");
  const customerName = String(formData.get("customer-name") || "").trim();
  const customerPhone = String(formData.get("customer-phone") || "").trim();
  const customerEmail = String(formData.get("customer-email") || "").trim();
  const selectedProfessionId = String(formData.get("customer-profession") || "").trim();
  const customerProfession = state.professionOptions.find((option) => option.id === selectedProfessionId)?.label || "";
  const customerSources = formData.getAll("customer-sources").map((value) => String(value));
  const customerSourcesNotInformed = formData.has("customer-sources-none");
  const queueJumpReason = String(formData.get("queue-jump-reason") || "").trim();
  const visitReasonDetail = String(formData.get("visit-reason-detail") || "").trim();
  const sourceDetail = String(formData.get("customer-source-detail") || "").trim();
  const visitReasonDetails =
    visitReasons[0] && visitReasonDetail
      ? { [visitReasons[0]]: visitReasonDetail }
      : {};
  const customerSourceDetails =
    customerSources[0] && sourceDetail
      ? { [customerSources[0]]: sourceDetail }
      : {};
  const activeService = state.activeServices.find((service) => service.id === personId);

  if (!personId || !outcome) {
    window.alert("Selecione como o atendimento terminou.");
    return;
  }

  if (modalConfig.requireVisitReason && visitReasons.length === 0 && !visitReasonsNotInformed) {
    window.alert("Selecione um motivo da visita ou marque 'Nao informado'.");
    return;
  }

  const productsSeenNone = formData.has("products-seen-none");
  if (modalConfig.requireProduct && productsSeen.length === 0 && !productsSeenNone) {
    window.alert("Selecione pelo menos um produto visto ou marque 'Nenhum'.");
    return;
  }

  if ((outcome === "reserva" || outcome === "compra") && modalConfig.requireProduct && productsClosed.length === 0) {
    window.alert("Selecione o produto comprado/reservado.");
    return;
  }

  if (modalConfig.requireCustomerNamePhone && (!customerName || !customerPhone)) {
    window.alert("Nome e telefone do cliente sao obrigatorios.");
    return;
  }

  if (modalConfig.requireCustomerSource && customerSources.length === 0 && !customerSourcesNotInformed) {
    window.alert("Selecione uma origem do cliente ou marque 'Nao informado'.");
    return;
  }

  if (activeService?.startMode === "queue-jump" && !queueJumpReason) {
    window.alert("Preencha o motivo do atendimento fora da vez.");
    return;
  }

  store.finishService(personId, {
    outcome,
    isWindowService: formData.has("is-window-service"),
    isGift: formData.has("is-gift"),
    productSeen,
    productClosed,
    productsSeen,
    productsClosed,
    productDetails: productClosed || productSeen,
    customerName,
    customerPhone,
    customerEmail,
    customerProfession,
    isExistingCustomer: formData.has("is-existing-customer"),
    visitReasons,
    visitReasonDetails,
    customerSources,
    customerSourceDetails,
    saleAmount: outcome === "reserva" || outcome === "compra" ? saleAmount : 0,
    queueJumpReason,
    notes: String(formData.get("notes") || "").trim()
  });
}

async function bootstrap() {
  renderApp();

  if (!appRoot) {
    return;
  }

  appRoot.addEventListener("click", handleClick);
  appRoot.addEventListener("change", handleChange);
  appRoot.addEventListener("input", handleInput);
  appRoot.addEventListener("keydown", handleKeydown);
  appRoot.addEventListener("submit", handleSubmit);

  // Close product-pick dropdowns when clicking outside any widget
  document.addEventListener("click", (e) => {
    if (!(e.target instanceof Element)) return;
    if (e.target.closest(".product-pick")) return;
    appRoot.querySelectorAll(".product-pick").forEach((widget) => {
      if (widget instanceof HTMLElement) {
        closePickerDropdown(widget);
      }
    });
  });
  store.subscribe(renderApp);
  store.subscribe((state) => saveQueueState(state));

  const initialState = await loadQueueState();
  store.hydrate(initialState);

  window.setInterval(() => {
    const state = store.getState();
    const snapshotHasLiveTimers = Object.values(state.storeSnapshots || {}).some(
      (snapshot) =>
        Array.isArray(snapshot?.activeServices) && snapshot.activeServices.length > 0
          ? true
          : Array.isArray(snapshot?.waitingList) && snapshot.waitingList.length > 0
            ? true
            : Array.isArray(snapshot?.pausedEmployees) && snapshot.pausedEmployees.length > 0
    );
    const hasLiveTimers =
      state.activeServices.length > 0 ||
      state.waitingList.length > 0 ||
      state.pausedEmployees.length > 0 ||
      snapshotHasLiveTimers;

    if (!hasLiveTimers || state.finishModalPersonId || isEditingFieldActive()) {
      return;
    }

    // Workspace operacao: atualiza apenas o texto dos timers, sem re-renderizar o DOM.
    // Isso preserva scroll interno das colunas e evita flicker na apresentacao.
    if (state.activeWorkspace === "operacao") {
      if (!appRoot) return;
      appRoot.querySelectorAll("[data-timer-start]").forEach((el) => {
        el.textContent = formatDuration(Date.now() - Number(el.dataset.timerStart));
      });
      return;
    }

    // Demais workspaces com dados ao vivo: re-renderiza normalmente.
    const shouldRefreshWorkspace = ["dados", "inteligencia", "multiloja"].includes(state.activeWorkspace);
    if (shouldRefreshWorkspace) {
      renderApp();
    }
  }, 1000);
}

bootstrap();
