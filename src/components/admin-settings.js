function renderBooleanSetting(label, settingId, checked) {
  return `
    <label class="settings-toggle">
      <input
        type="checkbox"
        data-action="set-setting"
        data-setting-id="${settingId}"
        ${checked ? "checked" : ""}
      >
      <span>${label}</span>
    </label>
  `;
}

function renderModalBooleanConfig(label, configKey, checked) {
  return `
    <label class="settings-toggle">
      <input
        type="checkbox"
        data-action="set-modal-config"
        data-config-key="${configKey}"
        ${checked ? "checked" : ""}
      >
      <span>${label}</span>
    </label>
  `;
}

function renderOptionManager({ title, description, items, optionGroup }) {
  const rows = items
    .map(
      (item) => `
        <form class="option-row" data-action="update-option" data-option-group="${optionGroup}" data-option-id="${item.id}">
          <input class="option-row__input" type="text" name="label" value="${item.label}">
          <button class="option-row__save" type="submit">Salvar</button>
          <button
            class="option-row__remove"
            type="button"
            data-action="remove-option"
            data-option-group="${optionGroup}"
            data-option-id="${item.id}"
          >
            Excluir
          </button>
        </form>
      `
    )
    .join("");

  return `
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">${title}</h3>
        <p class="settings-card__text">${description}</p>
      </header>
      <div class="option-list">
        ${rows || '<span class="insight-empty">Sem opcoes cadastradas.</span>'}
      </div>
      <form class="option-add" data-action="add-option" data-option-group="${optionGroup}">
        <input class="option-add__input" type="text" name="label" placeholder="Adicionar nova opcao">
        <button class="option-add__button" type="submit">Adicionar</button>
      </form>
    </article>
  `;
}

function renderProductManager(products) {
  const rows = products
    .map(
      (product) => `
        <div class="product-row">
          <input
            class="product-row__input"
            type="text"
            value="${product.name}"
            data-action="update-product"
            data-product-id="${product.id}"
            data-product-field="name"
          >
          <input
            class="product-row__input"
            type="text"
            value="${product.category || ""}"
            data-action="update-product"
            data-product-id="${product.id}"
            data-product-field="category"
          >
          <input
            class="product-row__input"
            type="number"
            min="0"
            step="0.01"
            value="${Number(product.basePrice || 0)}"
            data-action="update-product"
            data-product-id="${product.id}"
            data-product-field="basePrice"
          >
          <button
            class="product-row__remove"
            type="button"
            data-action="remove-product"
            data-product-id="${product.id}"
          >
            Excluir
          </button>
        </div>
      `
    )
    .join("");

  return `
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Catalogo de produtos</h3>
        <p class="settings-card__text">
          Usado no search do modal. Depois voce pode trocar por API sem mudar o fluxo do fechamento.
        </p>
      </header>
      <div class="product-head">
        <span>Produto</span>
        <span>Categoria</span>
        <span>Preco base</span>
      </div>
      <div class="product-list">
        ${rows || '<span class="insight-empty">Sem produtos no catalogo.</span>'}
      </div>
      <form class="product-add" data-action="add-product">
        <input class="product-add__input" type="text" name="name" placeholder="Nome do produto">
        <input class="product-add__input" type="text" name="category" placeholder="Categoria">
        <input class="product-add__input" type="number" name="basePrice" min="0" step="0.01" placeholder="Preco base">
        <button class="product-add__button" type="submit">Adicionar produto</button>
      </form>
    </article>
  `;
}

function renderOperationTemplateManager(templates, selectedOperationTemplateId, canManageSettings) {
  const cards = (templates || [])
    .map(
      (template) => `
        <article class="settings-card">
          <header class="settings-card__header">
            <h3 class="settings-card__title">${template.label}</h3>
            <p class="settings-card__text">${template.description}</p>
          </header>
          <div class="option-list">
            <span class="insight-tag">Max simultaneo <strong>${template.settings.maxConcurrentServices}</strong></span>
            <span class="insight-tag">Fechamento rapido <strong>${template.settings.timingFastCloseMinutes} min</strong></span>
            <span class="insight-tag">Atendimento demorado <strong>${template.settings.timingLongServiceMinutes} min</strong></span>
          </div>
          <button
            class="option-add__button"
            type="button"
            data-action="apply-operation-template"
            data-template-id="${template.id}"
            ${!canManageSettings ? "disabled" : ""}
          >
            ${selectedOperationTemplateId === template.id ? "Template ativo" : "Aplicar template"}
          </button>
        </article>
      `
    )
    .join("");

  return `
    <section class="settings-grid">
      ${cards || ""}
    </section>
  `;
}

function renderConsultantCrudManager(roster, canManageConsultants) {
  const rows = (roster || [])
    .map(
      (consultant) => `
        <form class="consultant-row" data-action="update-consultant" data-consultant-id="${consultant.id}">
          <input class="product-row__input" type="text" name="name" value="${consultant.name}" ${!canManageConsultants ? "disabled" : ""}>
          <input class="product-row__input" type="text" name="role" value="${consultant.role}" ${!canManageConsultants ? "disabled" : ""}>
          <input class="product-row__input" type="color" name="color" value="${consultant.color}" ${!canManageConsultants ? "disabled" : ""}>
          <input class="product-row__input" type="number" name="monthlyGoal" min="0" step="100" value="${Number(consultant.monthlyGoal || 0)}" ${!canManageConsultants ? "disabled" : ""}>
          <input class="product-row__input" type="number" name="commissionRate" min="0" max="1" step="0.001" value="${Number(consultant.commissionRate || 0)}" ${!canManageConsultants ? "disabled" : ""}>
          <button class="option-row__save" type="submit" ${!canManageConsultants ? "disabled" : ""}>Salvar</button>
          <button class="product-row__remove" type="button" data-action="archive-consultant" data-consultant-id="${consultant.id}" ${!canManageConsultants ? "disabled" : ""}>Arquivar</button>
        </form>
      `
    )
    .join("");

  return `
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Gestao de consultores</h3>
        <p class="settings-card__text">
          CRUD administrativo de perfil, meta e comissao.
        </p>
      </header>
      <div class="consultant-head">
        <span>Nome</span>
        <span>Cargo</span>
        <span>Cor</span>
        <span>Meta</span>
        <span>Comissao</span>
        <span></span>
        <span></span>
      </div>
      <div class="option-list">
        ${rows || '<span class="insight-empty">Nenhum consultor cadastrado.</span>'}
      </div>
      <form class="consultant-add" data-action="add-consultant">
        <input class="product-add__input" type="text" name="name" placeholder="Nome do consultor" ${!canManageConsultants ? "disabled" : ""}>
        <input class="product-add__input" type="text" name="role" placeholder="Cargo (ex: Atendimento)" ${!canManageConsultants ? "disabled" : ""}>
        <input class="product-add__input" type="number" name="monthlyGoal" min="0" step="100" placeholder="Meta mensal" ${!canManageConsultants ? "disabled" : ""}>
        <input class="product-add__input" type="number" name="commissionRate" min="0" max="1" step="0.001" placeholder="Comissao (0.03)" ${!canManageConsultants ? "disabled" : ""}>
        <input class="product-add__input" type="color" name="color" value="#168aad" ${!canManageConsultants ? "disabled" : ""}>
        <button class="product-add__button" type="submit" ${!canManageConsultants ? "disabled" : ""}>Adicionar consultor</button>
      </form>
    </article>
  `;
}

export function renderSettingsPanel({
  settings,
  modalConfig,
  visitReasonOptions,
  customerSourceOptions,
  professionOptions,
  productCatalog,
  operationTemplates,
  selectedOperationTemplateId,
  roster,
  canManageSettings = true,
  canManageConsultants = true
}) {
  const tabs = [
    { id: "operacao",      label: "Operacao",      icon: "tune" },
    { id: "modal",         label: "Modal",          icon: "edit_note" },
    { id: "produtos",      label: "Produtos",       icon: "inventory_2" },
    { id: "consultores",   label: "Consultores",    icon: "group" },
    { id: "motivos",       label: "Motivos",        icon: "fact_check" },
    { id: "origens",       label: "Origens",        icon: "share_location" },
    { id: "profissoes",    label: "Profissoes",     icon: "badge" }
  ];

  const tabNav = tabs
    .map((t) => `
      <button type="button" class="settings-tabs__btn ${t.id === "operacao" ? "is-active" : ""}"
        data-action="set-settings-tab" data-tab="${t.id}">
        <span class="material-icons-round">${t.icon}</span>
        <span>${t.label}</span>
      </button>
    `)
    .join("");

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Configuracoes</h2>
      </header>

      <nav class="settings-tabs">${tabNav}</nav>

      <!-- TAB: Operacao -->
      <div data-tab-panel="operacao">
        ${renderOperationTemplateManager(operationTemplates, selectedOperationTemplateId, canManageSettings)}
        <div class="settings-grid" style="margin-top:16px">
          <article class="settings-card">
            <header class="settings-card__header">
              <h3 class="settings-card__title">Limites e timings</h3>
            </header>
            <label class="settings-field">
              <span>Atendimentos simultaneos</span>
              <input type="number" min="1" max="100" value="${Number(settings.maxConcurrentServices || 10)}" data-action="set-setting" data-setting-id="maxConcurrentServices">
            </label>
            <label class="settings-field">
              <span>Fechamento rapido (min)</span>
              <input type="number" min="1" max="120" value="${Number(settings.timingFastCloseMinutes || 5)}" data-action="set-setting" data-setting-id="timingFastCloseMinutes">
            </label>
            <label class="settings-field">
              <span>Atendimento demorado (min)</span>
              <input type="number" min="1" max="240" value="${Number(settings.timingLongServiceMinutes || 25)}" data-action="set-setting" data-setting-id="timingLongServiceMinutes">
            </label>
            <label class="settings-field">
              <span>Venda baixa (R$)</span>
              <input type="number" min="1" step="1" value="${Number(settings.timingLowSaleAmount || 1200)}" data-action="set-setting" data-setting-id="timingLowSaleAmount">
            </label>
            ${renderBooleanSetting("Modo teste", "testModeEnabled", Boolean(settings.testModeEnabled))}
            ${renderBooleanSetting("Preencher modal automaticamente", "autoFillFinishModal", Boolean(settings.autoFillFinishModal))}
          </article>
        </div>
      </div>

      <!-- TAB: Modal -->
      <div data-tab-panel="modal" hidden>
        <div class="settings-grid">
          <article class="settings-card">
            <header class="settings-card__header">
              <h3 class="settings-card__title">Textos</h3>
            </header>
            <label class="settings-field">
              <span>Titulo do modal</span>
              <input type="text" value="${modalConfig.title}" data-action="set-modal-config" data-config-key="title">
            </label>
            <label class="settings-field">
              <span>Label da secao de cliente</span>
              <input type="text" value="${modalConfig.customerSectionLabel}" data-action="set-modal-config" data-config-key="customerSectionLabel">
            </label>
            <label class="settings-field">
              <span>Label observacoes</span>
              <input type="text" value="${modalConfig.notesLabel}" data-action="set-modal-config" data-config-key="notesLabel">
            </label>
            <label class="settings-field">
              <span>Placeholder observacoes</span>
              <input type="text" value="${modalConfig.notesPlaceholder}" data-action="set-modal-config" data-config-key="notesPlaceholder">
            </label>
            <label class="settings-field">
              <span>Label motivo fora da vez</span>
              <input type="text" value="${modalConfig.queueJumpReasonLabel}" data-action="set-modal-config" data-config-key="queueJumpReasonLabel">
            </label>
            <label class="settings-field">
              <span>Placeholder motivo fora da vez</span>
              <input type="text" value="${modalConfig.queueJumpReasonPlaceholder}" data-action="set-modal-config" data-config-key="queueJumpReasonPlaceholder">
            </label>
          </article>
          <article class="settings-card">
            <header class="settings-card__header">
              <h3 class="settings-card__title">Campos e validacoes</h3>
            </header>
            ${renderModalBooleanConfig("Mostrar email", "showEmailField", Boolean(modalConfig.showEmailField))}
            ${renderModalBooleanConfig("Mostrar profissao", "showProfessionField", Boolean(modalConfig.showProfessionField))}
            ${renderModalBooleanConfig("Mostrar observacoes", "showNotesField", Boolean(modalConfig.showNotesField))}
            ${renderModalBooleanConfig("Detalhe por motivo de visita", "showVisitReasonDetails", Boolean(modalConfig.showVisitReasonDetails))}
            ${renderModalBooleanConfig("Detalhe por origem", "showCustomerSourceDetails", Boolean(modalConfig.showCustomerSourceDetails))}
            ${renderModalBooleanConfig("Exigir produto", "requireProduct", Boolean(modalConfig.requireProduct))}
            ${renderModalBooleanConfig("Exigir motivo da visita", "requireVisitReason", Boolean(modalConfig.requireVisitReason))}
            ${renderModalBooleanConfig("Exigir origem do cliente", "requireCustomerSource", Boolean(modalConfig.requireCustomerSource))}
            ${renderModalBooleanConfig("Exigir nome e telefone", "requireCustomerNamePhone", Boolean(modalConfig.requireCustomerNamePhone))}
          </article>
        </div>
      </div>

      <!-- TAB: Produtos -->
      <div data-tab-panel="produtos" hidden>
        ${renderProductManager(productCatalog)}
      </div>

      <!-- TAB: Consultores -->
      <div data-tab-panel="consultores" hidden>
        ${renderConsultantCrudManager(roster, canManageConsultants)}
      </div>

      <!-- TAB: Motivos -->
      <div data-tab-panel="motivos" hidden>
        ${renderOptionManager({
          title: "Motivo da visita",
          description: "Opcoes exibidas no modal de fechamento.",
          items: visitReasonOptions,
          optionGroup: "visit-reason"
        })}
      </div>

      <!-- TAB: Origens -->
      <div data-tab-panel="origens" hidden>
        ${renderOptionManager({
          title: "Origem do cliente",
          description: "Opcoes exibidas no modal de fechamento.",
          items: customerSourceOptions,
          optionGroup: "customer-source"
        })}
      </div>

      <!-- TAB: Profissoes -->
      <div data-tab-panel="profissoes" hidden>
        ${renderOptionManager({
          title: "Profissoes",
          description: "Lista usada no modal. Se nao existir, tambem pode cadastrar na hora no fechamento.",
          items: professionOptions,
          optionGroup: "profession"
        })}
      </div>

    </section>
  `;
}
