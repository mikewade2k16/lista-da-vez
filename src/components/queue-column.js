import { formatClock, formatDuration } from "../utils/time.js";

function renderWaitingCard(person, index, isLimitReached) {
  const isNext = index === 0;
  const skippedCount = index;
  const actionHint = `Passa na frente de ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`;

  return `
    <article class="queue-card ${isNext ? "queue-card--next" : ""}">
      <span class="queue-card__position">${index + 1}</span>
      <span class="queue-card__avatar" style="--avatar-accent: ${person.color}">
        ${person.initials}
      </span>
      <span class="queue-card__content">
        <strong class="queue-card__name">${person.name}</strong>
        <span class="queue-card__role">${person.role}</span>
        <span class="queue-card__note">${isNext ? "Aguardando" : "Aguardando na fila"}</span>
      </span>
      <div class="queue-card__actions">
        ${isNext ? '<span class="queue-card__badge">Na vez</span>' : ""}
        ${!isNext
          ? `
            <div class="queue-card__action-wrap">
              <button
                class="queue-card__action"
                type="button"
                title="Atender fora da vez"
                data-action="start-service"
                data-person-id="${person.id}"
                ${isLimitReached ? "disabled" : ""}
              >
                <span class="material-icons-round">bolt</span>
              </button>
              <span class="queue-card__action-hint">${actionHint}</span>
            </div>
          `
          : ""}
      </div>
    </article>
  `;
}

// MODO APRESENTACAO: botao "Atender primeiro da fila" movido para cima da lista
// (renderizado como .queue-column__action-bar, fixo no topo da coluna)
// Para reverter: mover o bloco <div class="queue-column__footer"> de volta para
// dentro de renderWaitingContent() e remover renderWaitingActionBar()
function renderWaitingActionBar(items, activeServicesCount, maxConcurrentServices) {
  if (items.length === 0) return "";

  const isLimitReached = activeServicesCount >= maxConcurrentServices;

  return `
    <div class="queue-column__action-bar">
      <button
        class="column-action column-action--primary"
        type="button"
        data-action="start-service"
        ${isLimitReached ? "disabled" : ""}
      >
        ${isLimitReached ? `Limite de ${maxConcurrentServices} atendimentos ativos` : "Atender primeiro da fila"}
      </button>
    </div>
  `;
}

function renderWaitingContent(items, activeServicesCount, maxConcurrentServices) {
  if (items.length === 0) {
    return `
      <div class="queue-empty">
        <span class="queue-empty__icon">!</span>
        <strong class="queue-empty__title">Fila vazia</strong>
        <span class="queue-empty__text">Use a barra de Consultores abaixo para colocar alguem na lista.</span>
      </div>
    `;
  }

  const isLimitReached = activeServicesCount >= maxConcurrentServices;

  return items.map((person, index) => renderWaitingCard(person, index, isLimitReached)).join("");
}

function renderServiceCard(service) {
  const elapsedMs = Date.now() - service.serviceStartedAt;
  const skippedCount = service.skippedPeople.length;
  const serviceTypeLabel = service.startMode === "queue-jump" ? "Fora da vez" : "Na vez";

  return `
    <article class="service-card">
      <div class="service-card__header">
        <span class="service-card__eyebrow">Atendimento em andamento</span>
        <span class="queue-card__note">Iniciado as ${formatClock(service.serviceStartedAt)}</span>
        <span class="queue-card__note">ID ${service.serviceId}</span>
      </div>
      <div class="service-card__body">
        <span class="queue-card__avatar queue-card__avatar--large" style="--avatar-accent: ${service.color}">
          ${service.initials}
        </span>
        <div class="service-card__content">
          <strong class="queue-card__name">${service.name}</strong>
          <span class="queue-card__role">${service.role}</span>
          <span class="queue-card__note">
            ${serviceTypeLabel}${skippedCount > 0 ? `, passou ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}` : ""}
          </span>
        </div>
        <strong class="service-card__timer" data-timer-start="${service.serviceStartedAt}">${formatDuration(elapsedMs)}</strong>
      </div>
      <button class="column-action column-action--secondary" type="button" data-action="open-finish-modal" data-person-id="${service.id}">
        Encerrar atendimento
      </button>
    </article>
  `;
}

function renderServiceContent(services) {
  if (services.length === 0) {
    return `
      <div class="queue-empty">
        <span class="queue-empty__icon">...</span>
        <strong class="queue-empty__title">Nenhum atendimento em andamento</strong>
        <span class="queue-empty__text">Quando iniciar um atendimento, o tempo passa a ser contado aqui.</span>
      </div>
    `;
  }

  return services.map((service) => renderServiceCard(service)).join("");
}

export function renderQueueColumn({
  title,
  type,
  items = [],
  activeServices = [],
  maxConcurrentServices = 10
}) {
  const activeServicesCount = activeServices.length;
  const actionBar = type === "waiting"
    ? renderWaitingActionBar(items, activeServicesCount, maxConcurrentServices)
    : "";
  const content =
    type === "waiting"
      ? renderWaitingContent(items, activeServicesCount, maxConcurrentServices)
      : renderServiceContent(activeServices);

  return `
    <section class="queue-column">
      <header class="queue-column__header">${title}</header>
      ${actionBar}
      <div class="queue-column__body queue-column__body--${type}">
        ${content}
      </div>
    </section>
  `;
}
