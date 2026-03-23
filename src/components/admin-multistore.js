import {
  buildOperationalIntelligence,
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent
} from "../utils/admin-metrics.js";

function createEmptyScopedData() {
  return {
    waitingList: [],
    activeServices: [],
    pausedEmployees: [],
    serviceHistory: [],
    roster: [],
    consultantCurrentStatus: {},
    consultantActivitySessions: []
  };
}

function getStoreSnapshot(snapshotByStoreId, storeId) {
  const snapshot = snapshotByStoreId?.[storeId];

  if (!snapshot) {
    return createEmptyScopedData();
  }

  return {
    ...createEmptyScopedData(),
    ...snapshot
  };
}

function buildStoreRow({
  store,
  snapshot,
  visitReasonOptions,
  customerSourceOptions,
  settings
}) {
  const history = Array.isArray(snapshot.serviceHistory) ? snapshot.serviceHistory : [];
  const converted = history.filter((entry) => entry.finishOutcome === "compra" || entry.finishOutcome === "reserva");
  const soldValue = converted.reduce((sum, entry) => sum + Number(entry.saleAmount || 0), 0);
  const queueJumpCount = history.filter((entry) => entry.startMode === "queue-jump").length;
  const intelligence = buildOperationalIntelligence({
    history,
    visitReasonOptions,
    customerSourceOptions,
    roster: snapshot.roster,
    waitingList: snapshot.waitingList,
    activeServices: snapshot.activeServices,
    pausedEmployees: snapshot.pausedEmployees,
    consultantCurrentStatus: snapshot.consultantCurrentStatus,
    consultantActivitySessions: snapshot.consultantActivitySessions,
    settings
  });

  return {
    storeId: store.id,
    storeName: store.name,
    storeCode: store.code || "-",
    storeCity: store.city || "-",
    queueCount: snapshot.waitingList.length,
    activeCount: snapshot.activeServices.length,
    pausedCount: snapshot.pausedEmployees.length,
    consultants: snapshot.roster.length,
    attendances: history.length,
    conversionRate: intelligence.conversionRate,
    soldValue,
    ticketAverage: intelligence.ticketAverage,
    averageQueueWaitMs: intelligence.time.avgQueueWaitMs,
    queueJumpRate: history.length ? (queueJumpCount / history.length) * 100 : 0,
    healthScore: intelligence.healthScore
  };
}

function renderStoreRows(rows, activeStoreId) {
  const body = rows
    .map(
      (row) => `
        <tr>
          <td>${row.storeName}</td>
          <td>${row.storeCode}</td>
          <td>${row.storeCity}</td>
          <td>${row.consultants}</td>
          <td>${row.queueCount}</td>
          <td>${row.activeCount}</td>
          <td>${row.pausedCount}</td>
          <td>${row.attendances}</td>
          <td>${formatPercent(row.conversionRate)}</td>
          <td>${formatCurrencyBRL(row.soldValue)}</td>
          <td>${formatCurrencyBRL(row.ticketAverage)}</td>
          <td>${formatDurationMinutes(row.averageQueueWaitMs)}</td>
          <td>${formatPercent(row.queueJumpRate)}</td>
          <td>${Math.round(row.healthScore)}</td>
          <td>
            ${
              row.storeId === activeStoreId
                ? '<span class="insight-tag">Loja ativa</span>'
                : `
                  <button
                    class="option-row__save"
                    type="button"
                    data-action="set-active-store"
                    data-store-id="${row.storeId}"
                  >
                    Abrir loja
                  </button>
                `
            }
          </td>
        </tr>
      `
    )
    .join("");

  return `
    <article class="insight-card insight-card--wide">
      <h3 class="insight-card__title">Comparativo consolidado por loja</h3>
      <div class="insight-table-wrap">
        <table class="insight-table">
          <thead>
            <tr>
              <th>Loja</th>
              <th>Codigo</th>
              <th>Cidade</th>
              <th>Consultores</th>
              <th>Fila</th>
              <th>Em atendimento</th>
              <th>Pausados</th>
              <th>Atendimentos</th>
              <th>Conversao</th>
              <th>Vendas</th>
              <th>Ticket medio</th>
              <th>Espera media</th>
              <th>Fora da vez</th>
              <th>Score</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            ${body || '<tr><td colspan="15">Sem lojas cadastradas.</td></tr>'}
          </tbody>
        </table>
      </div>
    </article>
  `;
}

function renderStoreManagement(stores, canManageStores) {
  if (!canManageStores) {
    return "";
  }

  const storeRows = stores
    .map(
      (store) => `
        <form class="consultant-row multistore-row" data-action="update-store" data-store-id="${store.id}">
          <input class="product-row__input" type="text" name="name" value="${store.name}">
          <input class="product-row__input" type="text" name="code" value="${store.code || ""}" placeholder="Codigo">
          <input class="product-row__input" type="text" name="city" value="${store.city || ""}" placeholder="Cidade">
          <button class="option-row__save" type="submit">Salvar</button>
          <button class="product-row__remove" type="button" data-action="archive-store" data-store-id="${store.id}">
            Arquivar
          </button>
        </form>
      `
    )
    .join("");

  return `
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Gestao de lojas</h3>
        <p class="settings-card__text">Cadastro basico para operacao multi-loja.</p>
      </header>
      <div class="option-list">
        ${storeRows || '<span class="insight-empty">Nenhuma loja cadastrada.</span>'}
      </div>
      <form class="consultant-add multistore-add" data-action="add-store">
        <input class="product-add__input" type="text" name="name" placeholder="Nome da loja">
        <input class="product-add__input" type="text" name="code" placeholder="Codigo curto (opcional)">
        <input class="product-add__input" type="text" name="city" placeholder="Cidade (opcional)">
        <label class="settings-toggle">
          <input type="checkbox" name="clone-active-roster" checked>
          <span>Copiar consultores da loja ativa</span>
        </label>
        <button class="product-add__button" type="submit">Adicionar loja</button>
      </form>
    </article>
  `;
}

export function renderMultiStorePanel({
  stores,
  activeStoreId,
  snapshotByStoreId,
  visitReasonOptions,
  customerSourceOptions,
  settings,
  canManageStores
}) {
  const rows = (stores || [])
    .map((store) =>
      buildStoreRow({
        store,
        snapshot: getStoreSnapshot(snapshotByStoreId, store.id),
        visitReasonOptions,
        customerSourceOptions,
        settings
      })
    )
    .sort((a, b) => {
      if (b.soldValue !== a.soldValue) {
        return b.soldValue - a.soldValue;
      }

      return b.conversionRate - a.conversionRate;
    });
  const totalAttendances = rows.reduce((sum, row) => sum + row.attendances, 0);
  const totalSoldValue = rows.reduce((sum, row) => sum + row.soldValue, 0);
  const totalQueue = rows.reduce((sum, row) => sum + row.queueCount, 0);
  const totalActiveServices = rows.reduce((sum, row) => sum + row.activeCount, 0);
  const averageHealthScore = rows.length
    ? rows.reduce((sum, row) => sum + row.healthScore, 0) / rows.length
    : 0;

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Visao consolidada multi-loja</h2>
        <p class="admin-panel__text">Comparativo operacional para acompanhar performance entre lojas.</p>
      </header>

      <section class="metric-grid">
        <article class="metric-card">
          <span class="metric-card__label">Lojas ativas</span>
          <strong class="metric-card__value">${rows.length}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Atendimentos consolidados</span>
          <strong class="metric-card__value">${totalAttendances}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Vendas consolidadas</span>
          <strong class="metric-card__value">${formatCurrencyBRL(totalSoldValue)}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Fila atual total</span>
          <strong class="metric-card__value">${totalQueue}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Em atendimento agora</span>
          <strong class="metric-card__value">${totalActiveServices}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Score medio operacional</span>
          <strong class="metric-card__value">${Math.round(averageHealthScore)}</strong>
        </article>
      </section>

      <div class="insight-grid">
        ${renderStoreRows(rows, activeStoreId)}
      </div>

      ${renderStoreManagement(stores, canManageStores)}
    </section>
  `;
}
