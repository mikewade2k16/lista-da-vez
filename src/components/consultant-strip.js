function renderEmployeeButton(employee, status, pauseInfo) {
  const statusLabel =
    status === "service"
      ? "Em atendimento"
      : status === "queue"
        ? "Na fila"
        : status === "paused"
          ? "Pausado"
          : "Disponivel";
  const actionMarkup =
    status === "available"
      ? `
        <div class="employee__actions">
          <button class="employee__action employee__action--primary" type="button" data-action="add-to-queue" data-person-id="${employee.id}" title="Entrar na fila">
            <span class="material-icons-round">login</span>
          </button>
          <button class="employee__action employee__action--secondary" type="button" data-action="pause-employee" data-person-id="${employee.id}" title="Pausar">
            <span class="material-icons-round">pause</span>
          </button>
        </div>
      `
      : status === "queue"
        ? `
          <div class="employee__actions">
            <button class="employee__action employee__action--secondary" type="button" data-action="pause-employee" data-person-id="${employee.id}" title="Pausar">
              <span class="material-icons-round">pause</span>
            </button>
          </div>
        `
        : status === "paused"
          ? `
            <div class="employee__actions">
              <button class="employee__action employee__action--primary" type="button" data-action="resume-employee" data-person-id="${employee.id}" title="Retomar">
                <span class="material-icons-round">play_arrow</span>
              </button>
            </div>
          `
          : "";

  return `
    <div class="employee employee--${status}">
      <span class="employee__avatar" style="--avatar-accent: ${employee.color}">
        ${employee.initials}
      </span>
      <div class="employee__info">
        <span class="employee__name">${employee.name}</span>
        <span class="employee__status">${statusLabel}</span>
        ${pauseInfo ? `<span class="employee__pause-reason">${pauseInfo.reason}</span>` : ""}
      </div>
      ${actionMarkup}
    </div>
  `;
}

export function renderEmployeeStrip({ employees, waitingIds, activeServiceIds, pausedEmployees }) {
  const employeeMarkup = employees
    .map((employee) => {
      const pauseInfo = pausedEmployees.find((item) => item.personId === employee.id) || null;
      const status =
        activeServiceIds.includes(employee.id)
          ? "service"
          : pauseInfo
            ? "paused"
            : waitingIds.includes(employee.id)
              ? "queue"
              : "available";

      return renderEmployeeButton(employee, status, pauseInfo);
    })
    .join("");

  return `
    <footer class="employee-strip">
      <div class="employee-strip__header">
        <strong class="employee-strip__title">Consultores</strong>
        <span class="employee-strip__text">Entrar na fila, pausar e retomar ficam por aqui</span>
      </div>
      <div class="employee-strip__list">
        ${employeeMarkup}
      </div>
    </footer>
  `;
}
