export function renderAppHeader(state) {
  const activeServicesCount = state.activeServices.length;
  const activeStore = (state.stores || []).find((store) => store.id === state.activeStoreId) || null;
  const profileOptions = (state.profiles || [])
    .map(
      (profile) => `
        <option value="${profile.id}" ${profile.id === state.activeProfileId ? "selected" : ""}>
          ${profile.name}
        </option>
      `
    )
    .join("");
  const storeOptions = (state.stores || [])
    .map(
      (store) => `
        <option value="${store.id}" ${store.id === state.activeStoreId ? "selected" : ""}>
          ${store.name}
        </option>
      `
    )
    .join("");

  return `
    <header class="app-header">
      <div class="brand-bar">
        <div class="brand">
          <span class="brand__name">${state.brandName}</span>
          <span class="brand__sub">${state.pageTitle}${activeStore ? ` | ${activeStore.name}` : ""}</span>
        </div>
        <div class="brand__meta">
          <span class="summary-pill">${state.waitingList.length} na fila</span>
          <span class="summary-pill ${activeServicesCount > 0 ? "summary-pill--active" : ""}">
            ${activeServicesCount}/${state.settings.maxConcurrentServices} em atendimento
          </span>
          <span class="summary-pill">${state.serviceHistory.length} finalizados</span>
          <label class="summary-select">
            <span style="display: none;">Loja:</span >
            <select data-action="set-active-store" aria-label="Loja ativa">
              ${storeOptions}
            </select>
          </label>
          <label class="summary-select">
            <span style="display: none;">Perfil:</span>
            <select data-action="set-active-profile" aria-label="Perfil de acesso">
              ${profileOptions}
            </select>
          </label>
        </div>
      </div>
    </header>
  `;
}
