function renderSelectOptions(options, selectedValue) {
  return options
    .map((option) => `<option value="${option.value}" ${option.value === selectedValue ? "selected" : ""}>${option.label}</option>`)
    .join("");
}

function renderMultiCheckboxes(name, options, selectedValues, disabled) {
  return `
    <div class="campaign-option-list">
      ${(options || [])
        .map(
          (option) => `
            <label class="settings-toggle">
              <input
                type="checkbox"
                name="${name}"
                value="${option.id}"
                ${selectedValues.includes(option.id) ? "checked" : ""}
                ${disabled ? "disabled" : ""}
              >
              <span>${option.label}</span>
            </label>
          `
        )
        .join("")}
    </div>
  `;
}

function renderCampaignCard({
  campaign,
  visitReasonOptions,
  customerSourceOptions,
  stats,
  canManageCampaigns
}) {
  return `
    <form class="settings-card campaign-card" data-action="update-campaign" data-campaign-id="${campaign.id}">
      <header class="settings-card__header">
        <h3 class="settings-card__title">${campaign.name || "Campanha sem nome"}</h3>
        <p class="settings-card__text">${campaign.description || "Sem descricao"}</p>
      </header>

      <div class="insight-time-grid">
        <span class="insight-tag">Ativa: <strong>${campaign.isActive ? "Sim" : "Nao"}</strong></span>
        <span class="insight-tag">Aplicacoes: <strong>${stats.hits}</strong></span>
        <span class="insight-tag">Bonus total: <strong>${stats.bonusLabel}</strong></span>
      </div>

      <div class="campaign-grid">
        <label class="settings-field">
          <span>Nome</span>
          <input type="text" name="name" value="${campaign.name}" ${!canManageCampaigns ? "disabled" : ""}>
        </label>
        <label class="settings-field">
          <span>Descricao</span>
          <input type="text" name="description" value="${campaign.description}" ${!canManageCampaigns ? "disabled" : ""}>
        </label>
        <label class="settings-field">
          <span>Inicio</span>
          <input type="date" name="startsAt" value="${campaign.startsAt}" ${!canManageCampaigns ? "disabled" : ""}>
        </label>
        <label class="settings-field">
          <span>Fim</span>
          <input type="date" name="endsAt" value="${campaign.endsAt}" ${!canManageCampaigns ? "disabled" : ""}>
        </label>
        <label class="settings-field">
          <span>Desfecho alvo</span>
          <select name="targetOutcome" ${!canManageCampaigns ? "disabled" : ""}>
            ${renderSelectOptions(
              [
                { value: "compra-reserva", label: "Compra ou reserva" },
                { value: "compra", label: "Compra" },
                { value: "reserva", label: "Reserva" },
                { value: "nao-compra", label: "Nao compra" },
                { value: "qualquer", label: "Qualquer desfecho" }
              ],
              campaign.targetOutcome
            )}
          </select>
        </label>
        <label class="settings-field">
          <span>Cliente recorrente</span>
          <select name="existingCustomerFilter" ${!canManageCampaigns ? "disabled" : ""}>
            ${renderSelectOptions(
              [
                { value: "all", label: "Todos" },
                { value: "yes", label: "Somente sim" },
                { value: "no", label: "Somente nao" }
              ],
              campaign.existingCustomerFilter
            )}
          </select>
        </label>
        <label class="settings-field">
          <span>Venda minima (R$)</span>
          <input
            type="number"
            min="0"
            step="1"
            name="minSaleAmount"
            value="${Number(campaign.minSaleAmount || 0)}"
            ${!canManageCampaigns ? "disabled" : ""}
          >
        </label>
        <label class="settings-field">
          <span>Duracao maxima (min)</span>
          <input
            type="number"
            min="0"
            step="1"
            name="maxServiceMinutes"
            value="${Number(campaign.maxServiceMinutes || 0)}"
            ${!canManageCampaigns ? "disabled" : ""}
          >
        </label>
        <label class="settings-field">
          <span>Bonus fixo (R$)</span>
          <input
            type="number"
            min="0"
            step="0.01"
            name="bonusFixed"
            value="${Number(campaign.bonusFixed || 0)}"
            ${!canManageCampaigns ? "disabled" : ""}
          >
        </label>
        <label class="settings-field">
          <span>Bonus percentual (0.01 = 1%)</span>
          <input
            type="number"
            min="0"
            max="1"
            step="0.001"
            name="bonusRate"
            value="${Number(campaign.bonusRate || 0)}"
            ${!canManageCampaigns ? "disabled" : ""}
          >
        </label>
      </div>

      <div class="campaign-grid campaign-grid--toggles">
        <label class="settings-toggle">
          <input type="checkbox" name="isActive" ${campaign.isActive ? "checked" : ""} ${!canManageCampaigns ? "disabled" : ""}>
          <span>Campanha ativa</span>
        </label>
        <label class="settings-toggle">
          <input type="checkbox" name="queueJumpOnly" ${campaign.queueJumpOnly ? "checked" : ""} ${!canManageCampaigns ? "disabled" : ""}>
          <span>Somente fora da vez</span>
        </label>
      </div>

      <div class="campaign-grid campaign-grid--options">
        <div class="settings-field">
          <span>Origens alvo</span>
          ${renderMultiCheckboxes("sourceIds", customerSourceOptions, campaign.sourceIds || [], !canManageCampaigns)}
        </div>
        <div class="settings-field">
          <span>Motivos alvo</span>
          ${renderMultiCheckboxes("reasonIds", visitReasonOptions, campaign.reasonIds || [], !canManageCampaigns)}
        </div>
      </div>

      ${
        canManageCampaigns
          ? `
            <div class="report-actions">
              <button class="option-row__save" type="submit">Salvar campanha</button>
              <button class="option-row__remove" type="button" data-action="remove-campaign" data-campaign-id="${campaign.id}">
                Excluir campanha
              </button>
            </div>
          `
          : ""
      }
    </form>
  `;
}

function formatCurrency(value) {
  return new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(Number(value || 0));
}

function buildCampaignStats(campaigns, history) {
  const statsByCampaignId = new Map((campaigns || []).map((campaign) => [campaign.id, { hits: 0, bonus: 0 }]));

  (history || []).forEach((entry) => {
    const matches = Array.isArray(entry.campaignMatches) ? entry.campaignMatches : [];

    matches.forEach((match) => {
      const current = statsByCampaignId.get(match.campaignId);

      if (!current) {
        return;
      }

      current.hits += 1;
      current.bonus += Number(match.bonusValue || 0);
    });
  });

  return statsByCampaignId;
}

export function renderCampaignsPanel({
  campaigns,
  history,
  visitReasonOptions,
  customerSourceOptions,
  canManageCampaigns
}) {
  const statsByCampaignId = buildCampaignStats(campaigns, history);
  const totalBonus = [...statsByCampaignId.values()].reduce((sum, item) => sum + item.bonus, 0);
  const totalHits = [...statsByCampaignId.values()].reduce((sum, item) => sum + item.hits, 0);
  const activeCampaigns = (campaigns || []).filter((campaign) => campaign.isActive).length;
  const campaignCards = (campaigns || [])
    .map((campaign) => {
      const stats = statsByCampaignId.get(campaign.id) || { hits: 0, bonus: 0 };

      return renderCampaignCard({
        campaign,
        visitReasonOptions,
        customerSourceOptions,
        stats: {
          hits: stats.hits,
          bonusLabel: formatCurrency(stats.bonus)
        },
        canManageCampaigns
      });
    })
    .join("");

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Campanhas e regras comerciais</h2>
        <p class="admin-panel__text">Regras aplicadas automaticamente no fechamento para auditoria e bonus.</p>
      </header>

      <section class="metric-grid metric-grid--tight">
        <article class="metric-card">
          <span class="metric-card__label">Campanhas cadastradas</span>
          <strong class="metric-card__value">${campaigns.length}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Campanhas ativas</span>
          <strong class="metric-card__value">${activeCampaigns}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Aplicacoes no historico</span>
          <strong class="metric-card__value">${totalHits}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Bonus acumulado</span>
          <strong class="metric-card__value">${formatCurrency(totalBonus)}</strong>
        </article>
      </section>

      ${canManageCampaigns ? `
        <form class="settings-card campaign-create" data-action="add-campaign">
          <header class="settings-card__header">
            <h3 class="settings-card__title">Nova campanha</h3>
            <p class="settings-card__text">Cadastro rapido de regra comercial.</p>
          </header>
          <div class="campaign-grid">
            <label class="settings-field">
              <span>Nome</span>
              <input type="text" name="name" placeholder="Ex: Ticket premium noite">
            </label>
            <label class="settings-field">
              <span>Descricao</span>
              <input type="text" name="description" placeholder="Opcional">
            </label>
            <label class="settings-field">
              <span>Desfecho alvo</span>
              <select name="targetOutcome">
                <option value="compra-reserva">Compra ou reserva</option>
                <option value="compra">Compra</option>
                <option value="reserva">Reserva</option>
                <option value="nao-compra">Nao compra</option>
                <option value="qualquer">Qualquer desfecho</option>
              </select>
            </label>
            <label class="settings-field">
              <span>Venda minima (R$)</span>
              <input type="number" min="0" step="1" name="minSaleAmount" value="0">
            </label>
            <label class="settings-field">
              <span>Bonus fixo (R$)</span>
              <input type="number" min="0" step="0.01" name="bonusFixed" value="0">
            </label>
            <label class="settings-field">
              <span>Bonus percentual</span>
              <input type="number" min="0" max="1" step="0.001" name="bonusRate" value="0">
            </label>
          </div>
          <div class="campaign-grid campaign-grid--toggles">
            <label class="settings-toggle">
              <input type="checkbox" name="isActive" checked>
              <span>Campanha ativa</span>
            </label>
            <label class="settings-toggle">
              <input type="checkbox" name="queueJumpOnly">
              <span>Somente fora da vez</span>
            </label>
          </div>
          <div class="report-actions">
            <button class="option-add__button" type="submit">Criar campanha</button>
          </div>
        </form>
      ` : ""}

      <div class="settings-grid campaign-list">
        ${campaignCards || '<article class="settings-card"><span class="insight-empty">Nenhuma campanha cadastrada.</span></article>'}
      </div>
    </section>
  `;
}
