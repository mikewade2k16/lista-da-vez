import { getAllowedWorkspaces } from "../utils/permissions.js";

const WORKSPACES = [
  { id: "operacao",      label: "Operacao",     icon: "pending_actions" },
  { id: "consultor",     label: "Consultor",    icon: "person"          },
  { id: "ranking",       label: "Ranking",      icon: "leaderboard"     },
  { id: "dados",         label: "Dados",        icon: "bar_chart"       },
  { id: "inteligencia",  label: "Inteligencia", icon: "psychology"      },
  { id: "relatorios",    label: "Relatorios",   icon: "description"     },
  { id: "campanhas",     label: "Campanhas",    icon: "campaign"        },
  { id: "multiloja",     label: "Multi-loja",   icon: "store"           },
  { id: "configuracoes", label: "Config",       icon: "tune"            }
];

export function renderWorkspaceNav(activeWorkspace, role) {
  const allowedWorkspaces = new Set(getAllowedWorkspaces(role));
  const buttons = WORKSPACES.filter((workspace) => allowedWorkspaces.has(workspace.id)).map((workspace) => {
    const isActive = workspace.id === activeWorkspace;

    return `
      <button
        type="button"
        class="workspace-nav__button ${isActive ? "workspace-nav__button--active" : ""}"
        data-action="set-workspace"
        data-workspace-id="${workspace.id}"
        title="${workspace.label}"
      >
        <span class="material-icons-round workspace-nav__icon">${workspace.icon}</span>
        <span class="workspace-nav__label">${workspace.label}</span>
      </button>
    `;
  }).join("");

  return `
    <nav class="workspace-nav" aria-label="Areas do sistema">
      ${buttons}
    </nav>
  `;
}
