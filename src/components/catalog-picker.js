function escapeHtml(value) {
  return String(value || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function renderDatasetAttributes(dataset = {}) {
  return Object.entries(dataset)
    .filter(([, value]) => value !== undefined && value !== null && value !== false && value !== "")
    .map(([key, value]) => {
      const attrName = key.replace(/[A-Z]/g, (char) => `-${char.toLowerCase()}`);
      return `data-${attrName}="${escapeHtml(value === true ? "true" : String(value))}"`;
    })
    .join(" ");
}

export function escapePickerHtml(value) {
  return escapeHtml(value);
}

export function renderPickerSearchField(placeholder = "Buscar") {
  return `
    <label class="catalog-picker__search">
      <span class="material-icons-round">search</span>
      <input
        class="catalog-picker__search-input"
        type="search"
        placeholder="${escapeHtml(placeholder)}"
        autocomplete="off"
        spellcheck="false"
        data-picker-search-input
      >
    </label>
  `;
}

export function renderCatalogOptionButton({
  label,
  meta = "",
  metaHtml = "",
  icon = "",
  special = false,
  hidden = false,
  disabled = false,
  dataset = {},
  searchText = ""
}) {
  const dataAttributes = renderDatasetAttributes({
    ...dataset,
    pickerSearchTarget: searchText || `${label} ${meta}`.trim(),
    pickerSearchStatic: special ? "true" : dataset?.pickerSearchStatic
  });

  return `
    <button
      type="button"
      class="product-pick__option ${special ? "product-pick__option--special" : ""}"
      ${dataAttributes}
      ${hidden ? "hidden" : ""}
      ${disabled ? "disabled" : ""}
    >
      ${icon ? `<span class="material-icons-round">${escapeHtml(icon)}</span>` : ""}
      <span class="product-pick__option-name">${escapeHtml(label)}</span>
      ${metaHtml || (meta ? `<span class="product-pick__option-meta">${escapeHtml(meta)}</span>` : "")}
    </button>
  `;
}

export function renderSinglePickerSelectionMarkup({
  pick,
  label,
  inputName,
  value = "",
  noneName = "",
  isNone = false
}) {
  return `
    <span class="product-pick__tag ${isNone ? "product-pick__tag--muted" : ""}" data-option-entry="${escapeHtml(value || "__none__")}">
      ${escapeHtml(label)}
      <button type="button" class="product-pick__tag-remove"
        data-action="option-pick-clear"
        data-pick="${escapeHtml(pick)}"
        title="Remover">
        <span class="material-icons-round">close</span>
      </button>
    </span>
    ${value ? `<input type="hidden" data-option-selection-input name="${escapeHtml(inputName)}" value="${escapeHtml(value)}">` : ""}
    ${isNone ? `<input type="hidden" data-option-none-input name="${escapeHtml(noneName)}" value="1">` : ""}
  `;
}

export function renderSingleCatalogPicker({
  pick,
  label,
  triggerLabel,
  searchPlaceholder,
  options = [],
  inputName,
  noneName,
  selectedValue = "",
  selectedLabel = "",
  noneSelected = false,
  noneLabel = "Nao informado",
  showDetail = false,
  detailName = "",
  detailValue = "",
  detailPlaceholder = "Detalhe opcional"
}) {
  const selectedMarkup =
    selectedValue || noneSelected
      ? renderSinglePickerSelectionMarkup({
          pick,
          label: noneSelected ? noneLabel : selectedLabel,
          inputName,
          value: noneSelected ? "" : selectedValue,
          noneName,
          isNone: noneSelected
        })
      : "";
  const isDetailVisible = showDetail && Boolean(selectedValue) && !noneSelected;
  const optionsMarkup = [
    renderCatalogOptionButton({
      label: noneLabel,
      icon: "remove_circle_outline",
      special: true,
      dataset: {
        action: "option-pick-select",
        pick,
        optionNone: "true",
        optionLabel: noneLabel,
        filterHidden: "false",
        searchHidden: "false"
      },
      searchText: noneLabel
    }),
    ...options.map((option) =>
      renderCatalogOptionButton({
        label: option.label,
        meta: option.meta,
        metaHtml: option.metaHtml,
        hidden: Boolean(option.hidden),
        disabled: Boolean(option.disabled),
        dataset: {
          action: "option-pick-select",
          pick,
          optionId: option.id,
          optionLabel: option.label,
          outcomes: Array.isArray(option.outcomes) ? option.outcomes.join(" ") : "",
          filterHidden: option.hidden ? "true" : "false",
          searchHidden: "false"
        },
        searchText: option.searchText || `${option.label} ${option.meta || ""}`.trim()
      })
    )
  ].join("");

  return `
    <section class="finish-form__section">
      <label class="finish-form__label">${escapeHtml(label)}</label>
      <div
        class="product-pick product-pick--single"
        data-option-pick="${escapeHtml(pick)}"
        data-input-name="${escapeHtml(inputName)}"
        data-none-name="${escapeHtml(noneName)}"
      >
        <button type="button" class="product-pick__trigger" data-action="option-pick-toggle" data-pick="${escapeHtml(pick)}">
          <span class="material-icons-round">search</span>
          <span>${escapeHtml(triggerLabel)}</span>
        </button>
        <div class="product-pick__dropdown">
          ${renderPickerSearchField(searchPlaceholder)}
          ${optionsMarkup}
        </div>
        <div class="product-pick__tags product-pick__tags--single" data-option-selection>${selectedMarkup}</div>
        ${
          showDetail
            ? `
              <div class="pick-detail-wrap ${detailValue ? "is-open" : ""}" data-option-detail ${isDetailVisible ? "" : "hidden"}>
                <button type="button" class="pick-detail-add" data-action="pick-detail-toggle" data-pick="${escapeHtml(pick)}">
                  <span class="material-icons-round">add</span>
                  <span>Adicionar detalhe</span>
                </button>
                <div class="pick-detail-field">
                  <input
                    class="finish-form__input pick-detail-input"
                    type="text"
                    name="${escapeHtml(detailName)}"
                    value="${escapeHtml(detailValue)}"
                    placeholder="${escapeHtml(detailPlaceholder)}"
                  >
                  <button type="button" class="pick-detail-close" data-action="pick-detail-close" data-pick="${escapeHtml(pick)}" title="Remover detalhe">
                    <span class="material-icons-round">close</span>
                  </button>
                </div>
              </div>
            `
            : ""
        }
      </div>
    </section>
  `;
}
