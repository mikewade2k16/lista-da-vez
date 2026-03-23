import { renderAppHeader } from "../components/app-header.js";
import { renderCampaignsPanel } from "../components/admin-campaigns.js";
import { renderConsultantPanel } from "../components/admin-consultant.js";
import { renderIntelligencePanel } from "../components/admin-intelligence.js";
import { renderDataPanel } from "../components/admin-insights.js";
import { renderMultiStorePanel } from "../components/admin-multistore.js";
import { renderRankingPanel } from "../components/admin-ranking.js";
import { renderReportsPanel } from "../components/admin-reports.js";
import { renderSettingsPanel } from "../components/admin-settings.js";
import { renderEmployeeStrip } from "../components/consultant-strip.js";
import { renderFinishModal } from "../components/finish-modal.js";
import { renderQueueColumn } from "../components/queue-column.js";
import { renderWorkspaceNav } from "../components/workspace-nav.js";
import { canManageCampaigns, canManageConsultants, canManageSettings, canManageStores } from "../utils/permissions.js";

function renderLoadingState() {
  return `
    <main class="shell">
      <section class="app-surface app-surface--loading">
        <div class="loading-state">
          <h1 class="loading-state__title">Carregando a fila de atendimento...</h1>
        </div>
      </section>
    </main>
  `;
}

export function renderHomePage(state) {
  if (!state.isReady) {
    return renderLoadingState();
  }

  const activeProfile = state.profiles.find((profile) => profile.id === state.activeProfileId) || state.profiles[0];
  const activeRole = activeProfile?.role || "consultant";
  const reportUiState = state.reportUiState || {};
  const snapshotByStoreId = {
    ...(state.storeSnapshots || {}),
    [state.activeStoreId]: {
      selectedConsultantId: state.selectedConsultantId,
      consultantSimulationAdditionalSales: state.consultantSimulationAdditionalSales,
      waitingList: state.waitingList,
      activeServices: state.activeServices,
      roster: state.roster,
      consultantActivitySessions: state.consultantActivitySessions,
      consultantCurrentStatus: state.consultantCurrentStatus,
      pausedEmployees: state.pausedEmployees,
      serviceHistory: state.serviceHistory
    }
  };

  const modalService = state.activeServices.find((service) => service.id === state.finishModalPersonId) || null;
  const operationWorkspace = `
    <div class="workspace__intro" style="display: none;">
      <h1 class="workspace__title">Lista da vez</h1>
      <p class="workspace__text">
        Toque em um funcionario abaixo para entrar na fila. O atendimento normal sai pelo botao do rodape
        e o fora da vez fica no card do consultor. Ao encerrar, abrimos o fechamento completo no modal.
      </p>
    </div>

    <div class="queue-grid">
      ${renderQueueColumn({
        title: "Lista da vez",
        type: "waiting",
        items: state.waitingList,
        activeServices: state.activeServices,
        maxConcurrentServices: state.settings.maxConcurrentServices
      })}
      ${renderQueueColumn({
        title: "Em atendimento",
        type: "service",
        activeServices: state.activeServices
      })}
    </div>

    ${renderEmployeeStrip({
      employees: state.roster,
      waitingIds: state.waitingList.map((person) => person.id),
      activeServiceIds: state.activeServices.map((service) => service.id),
      pausedEmployees: state.pausedEmployees
    })}
  `;

  const adminWorkspace =
    state.activeWorkspace === "consultor"
      ? renderConsultantPanel({
          roster: state.roster,
          selectedConsultantId: state.selectedConsultantId,
          history: state.serviceHistory,
          simulationAdditionalSales: state.consultantSimulationAdditionalSales
        })
      : state.activeWorkspace === "ranking"
        ? renderRankingPanel({
            history: state.serviceHistory,
            roster: state.roster
          })
        : state.activeWorkspace === "relatorios"
          ? renderReportsPanel({
              history: state.serviceHistory,
              roster: state.roster,
              visitReasonOptions: state.visitReasonOptions,
              customerSourceOptions: state.customerSourceOptions,
              reportFilters: state.reportFilters,
              reportUiState
            })
          : state.activeWorkspace === "multiloja"
            ? renderMultiStorePanel({
                stores: state.stores || [],
                activeStoreId: state.activeStoreId,
                snapshotByStoreId,
                visitReasonOptions: state.visitReasonOptions,
                customerSourceOptions: state.customerSourceOptions,
                settings: state.settings,
                canManageStores: canManageStores(activeRole)
              })
          : state.activeWorkspace === "campanhas"
            ? renderCampaignsPanel({
                campaigns: state.campaigns,
                history: state.serviceHistory,
                visitReasonOptions: state.visitReasonOptions,
                customerSourceOptions: state.customerSourceOptions,
                canManageCampaigns: canManageCampaigns(activeRole)
              })
        : state.activeWorkspace === "dados"
          ? renderDataPanel({
              history: state.serviceHistory,
              visitReasonOptions: state.visitReasonOptions,
              customerSourceOptions: state.customerSourceOptions,
              roster: state.roster,
              waitingList: state.waitingList,
              activeServices: state.activeServices,
              pausedEmployees: state.pausedEmployees,
              consultantCurrentStatus: state.consultantCurrentStatus,
              consultantActivitySessions: state.consultantActivitySessions,
              settings: state.settings
            })
          : state.activeWorkspace === "inteligencia"
            ? renderIntelligencePanel({
                history: state.serviceHistory,
                visitReasonOptions: state.visitReasonOptions,
                customerSourceOptions: state.customerSourceOptions,
                roster: state.roster,
                waitingList: state.waitingList,
                activeServices: state.activeServices,
                pausedEmployees: state.pausedEmployees,
                consultantCurrentStatus: state.consultantCurrentStatus,
                consultantActivitySessions: state.consultantActivitySessions,
                settings: state.settings
              })
          : renderSettingsPanel({
              settings: state.settings,
              modalConfig: state.modalConfig,
              visitReasonOptions: state.visitReasonOptions,
              customerSourceOptions: state.customerSourceOptions,
              professionOptions: state.professionOptions,
              productCatalog: state.productCatalog,
              operationTemplates: state.operationTemplates,
              selectedOperationTemplateId: state.selectedOperationTemplateId,
              roster: state.roster,
              canManageSettings: canManageSettings(activeRole),
              canManageConsultants: canManageConsultants(activeRole)
            });

  return `
    <main class="shell">
      <section class="app-surface">
        ${renderAppHeader(state)}
        <div class="workspace">
          ${renderWorkspaceNav(state.activeWorkspace, activeRole)}
          ${state.activeWorkspace === "operacao" ? operationWorkspace : adminWorkspace}
        </div>
      </section>
      ${renderFinishModal({
        service: modalService,
        visitReasonOptions: state.visitReasonOptions,
        customerSourceOptions: state.customerSourceOptions,
        professionOptions: state.professionOptions,
        productCatalog: state.productCatalog,
        modalConfig: state.modalConfig,
        draft: state.finishModalDraft
      })}
    </main>
  `;
}
