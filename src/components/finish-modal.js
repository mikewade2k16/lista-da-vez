import {
  escapePickerHtml as escapeHtml,
  renderCatalogOptionButton,
  renderPickerSearchField,
  renderSingleCatalogPicker
} from "./catalog-picker.js";

function formatPrice(value) {
  return Number(value || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

function findOptionByLabel(options, label) {
  const normalizedLabel = String(label || "").trim().toLowerCase();

  if (!normalizedLabel) {
    return null;
  }

  return (options || []).find((item) => String(item?.label || "").trim().toLowerCase() === normalizedLabel) || null;
}

function findOptionById(options, id) {
  const normalizedId = String(id || "").trim();

  if (!normalizedId) {
    return null;
  }

  return (options || []).find((item) => String(item?.id || "").trim() === normalizedId) || null;
}

function buildCatalogOptions(catalog, pick) {
  return catalog
    .map(
      (p) =>
        renderCatalogOptionButton({
          label: p.name,
          metaHtml: `<span class="product-pick__option-meta">${escapeHtml(p.category)}${pick === "closed" ? ` &middot; ${formatPrice(p.basePrice)}` : ""}</span>`,
          dataset: {
            action: "product-pick-select",
            pick,
            productId: p.id,
            productName: p.name,
            productPrice: String(p.basePrice),
            filterHidden: "false",
            searchHidden: "false"
          },
          searchText: `${p.name} ${p.category || ""} ${pick === "closed" ? p.basePrice : ""}`.trim()
        })
    )
    .join("");
}

function renderProductTag(product) {
  return `
    <span class="product-pick__tag" data-pick-entry="${escapeHtml(product.id)}">
      ${escapeHtml(product.name)}
      <button type="button" class="product-pick__tag-remove"
        data-action="remove-product" data-pick="seen" data-product-id="${escapeHtml(product.id)}"
        title="Remover">
        <span class="material-icons-round">close</span>
      </button>
    </span>
    <input type="hidden" name="products-seen" data-pick-input="${escapeHtml(product.id)}" value="${escapeHtml(JSON.stringify(product))}">
  `;
}

function renderClosedProduct(product) {
  return `
    <div class="product-pick__closed-item" data-pick-entry="${escapeHtml(product.id)}">
      <span class="product-pick__closed-name">
        ${escapeHtml(product.name)}${product.code ? ` <small class="product-pick__closed-code">(${escapeHtml(product.code)})</small>` : ""}
      </span>
      <span class="product-pick__closed-price">${formatPrice(product.price)}</span>
      <button type="button" class="product-pick__tag-remove"
        data-action="remove-product" data-pick="closed" data-product-id="${escapeHtml(product.id)}"
        title="Remover">
        <span class="material-icons-round">close</span>
      </button>
    </div>
    <input type="hidden" name="products-closed" data-pick-input="${escapeHtml(product.id)}" value="${escapeHtml(JSON.stringify(product))}">
  `;
}

export function renderFinishModal({
  service,
  visitReasonOptions,
  customerSourceOptions,
  professionOptions,
  productCatalog,
  modalConfig,
  draft
}) {
  if (!service) {
    return "";
  }

  const currentDraft = draft || {};
  const selectedOutcome = currentDraft.outcome || "";
  const selectedVisitReasons = Array.isArray(currentDraft.visitReasons) ? currentDraft.visitReasons : [];
  const selectedSources = Array.isArray(currentDraft.customerSources) ? currentDraft.customerSources : [];
  const showVisitReasonDetails = Boolean(modalConfig.showVisitReasonDetails);
  const showSourceDetails = Boolean(modalConfig.showCustomerSourceDetails);
  const selectedVisitReasonId = selectedVisitReasons[0] || "";
  const selectedSourceId = selectedSources[0] || "";
  const selectedVisitReason = findOptionById(visitReasonOptions, selectedVisitReasonId);
  const selectedSource = findOptionById(customerSourceOptions, selectedSourceId);
  const visitReasonDetailValue = selectedVisitReasonId
    ? currentDraft.visitReasonDetails?.[selectedVisitReasonId] || ""
    : "";
  const sourceDetailValue = selectedSourceId
    ? currentDraft.customerSourceDetails?.[selectedSourceId] || ""
    : "";
  const visitReasonNoneSelected = Boolean(currentDraft.visitReasonsNotInformed) && !selectedVisitReasonId;
  const sourceNoneSelected = Boolean(currentDraft.customerSourcesNotInformed) && !selectedSourceId;
  const selectedProfessionOption = findOptionByLabel(professionOptions, currentDraft.customerProfession);

  const productsSeen = Array.isArray(currentDraft.productsSeen) ? currentDraft.productsSeen : [];
  const productsClosed = Array.isArray(currentDraft.productsClosed) ? currentDraft.productsClosed : [];
  const closedTotal = productsClosed.reduce((s, p) => s + (Number(p.price) || 0), 0);
  const visitReasonPickerOptions = (visitReasonOptions || []).map((reason) => ({
    id: reason.id,
    label: reason.label,
    outcomes: reason.outcomes,
    hidden: Boolean(selectedOutcome && reason.outcomes && !reason.outcomes.includes(selectedOutcome))
  }));
  const sourcePickerOptions = (customerSourceOptions || []).map((source) => ({
    id: source.id,
    label: source.label
  }));

  const isClosedVisible = selectedOutcome === "reserva" || selectedOutcome === "compra";
  const closedLabel =
    selectedOutcome === "compra"
      ? "Produto comprado"
      : selectedOutcome === "reserva"
        ? "Produto reservado"
        : "Produto comprado/reservado";
  const selectedProfessionId = selectedProfessionOption?.id || "";
  const selectedProfessionLabel = selectedProfessionOption?.label || "";
  const professionPickerOptions = (professionOptions || []).map((o) => ({ id: o.id, label: o.label }));

  return `
    <div class="modal-backdrop">
      <div class="finish-modal" role="dialog" aria-modal="true" aria-labelledby="finish-modal-title">
        <div class="finish-modal__header">
          <div>
            <h2 class="finish-modal__title" id="finish-modal-title">${escapeHtml(modalConfig.title)}</h2>
            <p class="finish-modal__subtitle">${service.name} | ID ${service.serviceId}</p>
          </div>
          <button class="finish-modal__close" type="button" data-action="close-finish-modal" aria-label="Fechar">
            X
          </button>
        </div>

        <form class="finish-form" data-action="finish-service-form" data-person-id="${service.id}">
          <section class="finish-form__section">
            <strong class="finish-form__label">Como terminou</strong>
            <div class="finish-form__options">
              <label class="modal-radio">
                <input type="radio" name="finish-outcome" value="reserva" ${selectedOutcome === "reserva" ? "checked" : ""}>
                <span>Reserva</span>
              </label>
              <label class="modal-radio">
                <input type="radio" name="finish-outcome" value="compra" ${selectedOutcome === "compra" ? "checked" : ""}>
                <span>Compra</span>
              </label>
              <label class="modal-radio">
                <input type="radio" name="finish-outcome" value="nao-compra" ${selectedOutcome === "nao-compra" ? "checked" : ""}>
                <span>Nao compra</span>
              </label>
            </div>
          </section>

          <section class="finish-form__section finish-form__grid">
            <label class="modal-checkbox">
              <input type="checkbox" name="is-window-service" ${currentDraft.isWindowService ? "checked" : ""}>
              <span>Atendimento de vitrine</span>
            </label>
            <label class="modal-checkbox" data-field="gift" ${isClosedVisible ? "" : "hidden"}>
              <input type="checkbox" name="is-gift" ${currentDraft.isGift ? "checked" : ""}>
              <span>Foi para presente</span>
            </label>
            <label class="modal-checkbox">
              <input type="checkbox" name="is-existing-customer" ${currentDraft.isExistingCustomer ? "checked" : ""}>
              <span>Ja era cliente</span>
            </label>
          </section>

          <section class="finish-form__section">
            <label class="finish-form__label">${escapeHtml(modalConfig.productSeenLabel || "Produto visto pelo cliente")}</label>
            <div class="product-pick" data-product-pick="seen">
              <div class="product-pick__trigger-row">
                <button type="button" class="product-pick__trigger" data-action="product-pick-toggle" data-pick="seen">
                  <span class="material-icons-round">add</span>
                  <span>Selecionar produto</span>
                </button>
                <button type="button" class="product-pick__none-btn ${currentDraft.productsSeenNone ? "is-active" : ""}" data-action="product-pick-none-toggle" data-pick="seen">
                  Nenhum
                </button>
              </div>
              <div class="product-pick__dropdown">
                ${renderPickerSearchField(modalConfig.productSeenPlaceholder || "Buscar produto")}
                <button type="button" class="product-pick__option product-pick__option--special"
                  data-action="product-pick-custom-toggle" data-pick="seen">
                  <span class="material-icons-round">add_circle_outline</span>
                  Produto nao cadastrado
                </button>
                ${buildCatalogOptions(productCatalog, "seen")}
              </div>
              <div class="product-pick__custom-form">
                <div class="product-pick__custom-fields">
                  <input type="text" class="finish-form__input" placeholder="Codigo (opcional)" data-custom="code">
                  <input type="text" class="finish-form__input" placeholder="Nome do produto *" data-custom="name">
                  <input type="number" class="finish-form__input" placeholder="Valor R$" data-custom="price" min="0" step="0.01">
                </div>
                <div class="product-pick__custom-actions">
                  <button type="button" class="column-action column-action--secondary" data-action="product-pick-custom-cancel" data-pick="seen">Cancelar</button>
                  <button type="button" class="column-action column-action--primary" data-action="product-pick-custom-add" data-pick="seen">Confirmar</button>
                </div>
              </div>
              <div class="product-pick__tags" data-pick-tags="seen">
                ${productsSeen.map(renderProductTag).join("")}
              </div>
              ${currentDraft.productsSeenNone ? '<input type="hidden" name="products-seen-none" value="1">' : ""}
            </div>
          </section>

          <section class="finish-form__section" data-field="product-closed" ${isClosedVisible ? "" : "hidden"}>
            <label class="finish-form__label" data-label-compra="Produto comprado" data-label-reserva="Produto reservado">${closedLabel}</label>
            <div class="product-pick" data-product-pick="closed">
              <button type="button" class="product-pick__trigger" data-action="product-pick-toggle" data-pick="closed">
                <span class="material-icons-round">add</span>
                <span>Selecionar produto</span>
              </button>
              <div class="product-pick__dropdown">
                ${renderPickerSearchField(modalConfig.productClosedPlaceholder || "Buscar produto")}
                <button type="button" class="product-pick__option product-pick__option--special"
                  data-action="product-pick-custom-toggle" data-pick="closed">
                  <span class="material-icons-round">add_circle_outline</span>
                  Produto nao cadastrado
                </button>
                ${buildCatalogOptions(productCatalog, "closed")}
              </div>
              <div class="product-pick__custom-form">
                <div class="product-pick__custom-fields">
                  <input type="text" class="finish-form__input" placeholder="Codigo (opcional)" data-custom="code">
                  <input type="text" class="finish-form__input" placeholder="Nome do produto *" data-custom="name">
                  <input type="number" class="finish-form__input" placeholder="Valor R$" data-custom="price" min="0" step="0.01">
                </div>
                <div class="product-pick__custom-actions">
                  <button type="button" class="column-action column-action--secondary" data-action="product-pick-custom-cancel" data-pick="closed">Cancelar</button>
                  <button type="button" class="column-action column-action--primary" data-action="product-pick-custom-add" data-pick="closed">Confirmar</button>
                </div>
              </div>
              <div class="product-pick__selected" data-pick-selected="closed">
                ${productsClosed.map(renderClosedProduct).join("")}
              </div>
              <div class="product-pick__total" ${closedTotal > 0 ? "" : "hidden"}>
                <span>Total:</span>
                <strong data-pick-total="closed">${formatPrice(closedTotal)}</strong>
              </div>
            </div>
          </section>

          <section class="finish-form__section">
            <strong class="finish-form__label">${escapeHtml(modalConfig.customerSectionLabel)}</strong>
          </section>

          <section class="finish-form__section finish-form__grid finish-form__grid--customer">
            <label class="finish-form__field">
              <span class="finish-form__label">Nome do cliente</span>
              <input class="finish-form__input" type="text" name="customer-name" value="${escapeHtml(currentDraft.customerName || "")}" placeholder="Nome">
            </label>
            <label class="finish-form__field">
              <span class="finish-form__label">Telefone</span>
              <input class="finish-form__input" type="tel" name="customer-phone" value="${escapeHtml(currentDraft.customerPhone || "")}" placeholder="Telefone">
            </label>
            ${
              modalConfig.showEmailField
                ? `
                  <label class="finish-form__field">
                    <span class="finish-form__label">Email</span>
                    <input class="finish-form__input" type="email" name="customer-email" value="${escapeHtml(currentDraft.customerEmail || "")}" placeholder="Email opcional">
                  </label>
                `
                : ""
            }
          </section>

          ${
            modalConfig.showProfessionField
              ? renderSingleCatalogPicker({
                  pick: "profession",
                  label: "Profissao",
                  triggerLabel: "Selecionar profissao",
                  searchPlaceholder: "Buscar profissao",
                  options: professionPickerOptions,
                  inputName: "customer-profession",
                  noneName: "customer-profession-none",
                  selectedValue: selectedProfessionId,
                  selectedLabel: selectedProfessionLabel,
                  noneLabel: "Nao informado"
                })
              : ""
          }

          <div class="finish-form__reasons-row">
            ${renderSingleCatalogPicker({
              pick: "visit-reason",
              label: "Motivo da visita",
              triggerLabel: "Selecionar motivo",
              searchPlaceholder: "Buscar motivo",
              options: visitReasonPickerOptions,
              inputName: "visit-reasons",
              noneName: "visit-reasons-none",
              selectedValue: selectedVisitReasonId,
              selectedLabel: selectedVisitReason?.label || "",
              noneSelected: visitReasonNoneSelected,
              showDetail: showVisitReasonDetails,
              detailName: "visit-reason-detail",
              detailValue: visitReasonDetailValue,
              detailPlaceholder: "Detalhe opcional"
            })}

            ${renderSingleCatalogPicker({
              pick: "customer-source",
              label: "De onde o cliente veio",
              triggerLabel: "Selecionar origem",
              searchPlaceholder: "Buscar origem",
              options: sourcePickerOptions,
              inputName: "customer-sources",
              noneName: "customer-sources-none",
              selectedValue: selectedSourceId,
              selectedLabel: selectedSource?.label || "",
              noneSelected: sourceNoneSelected,
              showDetail: showSourceDetails,
              detailName: "customer-source-detail",
              detailValue: sourceDetailValue,
              detailPlaceholder: "Detalhe opcional"
            })}
          </div>

          ${service.startMode === "queue-jump"
            ? `
              <section class="finish-form__section">
                <label class="finish-form__label" for="queue-jump-reason">${escapeHtml(modalConfig.queueJumpReasonLabel)}</label>
                <textarea
                  class="finish-form__textarea"
                  id="queue-jump-reason"
                  name="queue-jump-reason"
                  rows="2"
                  placeholder="${escapeHtml(modalConfig.queueJumpReasonPlaceholder)}"
                >${escapeHtml(currentDraft.queueJumpReason || "")}</textarea>
              </section>
            `
            : ""}

          ${
            modalConfig.showNotesField
              ? `
                <section class="finish-form__section">
                  <label class="finish-form__label" for="notes">${escapeHtml(modalConfig.notesLabel)}</label>
                  <textarea
                    class="finish-form__textarea"
                    id="notes"
                    name="notes"
                    rows="3"
                    placeholder="${escapeHtml(modalConfig.notesPlaceholder)}"
                  >${escapeHtml(currentDraft.notes || "")}</textarea>
                </section>
              `
              : ""
          }

          <div class="finish-form__actions">
            <button class="column-action column-action--secondary" type="button" data-action="close-finish-modal">
              Cancelar
            </button>
            <button class="column-action column-action--primary" type="submit">
              Salvar e encerrar
            </button>
          </div>
        </form>
      </div>
    </div>
  `;
}
