<script setup>
import { computed, ref } from "vue";
import { canManageConsultants, canManageSettings } from "@core/utils/permissions";
import SettingsConsultantManager from "~/components/settings/SettingsConsultantManager.vue";
import SettingsOperationTemplateManager from "~/components/settings/SettingsOperationTemplateManager.vue";
import SettingsOptionManager from "~/components/settings/SettingsOptionManager.vue";
import SettingsProductManager from "~/components/settings/SettingsProductManager.vue";
import SettingsTabs from "~/components/settings/SettingsTabs.vue";
import { useDashboardStore } from "~/stores/dashboard";
import { useUiStore } from "~/stores/ui";

const tabs = [
  { id: "operacao", label: "Operacao", icon: "tune" },
  { id: "modal", label: "Modal", icon: "edit_note" },
  { id: "produtos", label: "Produtos", icon: "inventory_2" },
  { id: "consultores", label: "Consultores", icon: "group" },
  { id: "motivos", label: "Motivos", icon: "fact_check" },
  { id: "origens", label: "Origens", icon: "share_location" },
  { id: "profissoes", label: "Profissoes", icon: "badge" },
  { id: "alertas", label: "Alertas", icon: "notifications_active" }
];

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const dashboard = useDashboardStore();
const ui = useUiStore();
const activeTab = ref("operacao");

const activeRole = computed(() => {
  const activeProfile =
    (props.state.profiles || []).find((profile) => profile.id === props.state.activeProfileId) ||
    props.state.profiles?.[0] ||
    null;

  return activeProfile?.role || "consultant";
});
const canEditSettings = computed(() => canManageSettings(activeRole.value));
const canEditConsultants = computed(() => canManageConsultants(activeRole.value));

function updateNumericSetting(settingId, value) {
  void dashboard.updateSetting(settingId, Number(value) || 0);
}

function updateBooleanSetting(settingId, value) {
  void dashboard.updateSetting(settingId, Boolean(value));
}

function updateModalConfigValue(configKey, value) {
  void dashboard.updateModalConfig(configKey, value);
}

function applyTemplate(templateId) {
  void dashboard.applyOperationTemplate(templateId);
}

function addOption(group, label) {
  if (group === "visit-reason") {
    void dashboard.addVisitReasonOption(label);
    return;
  }

  if (group === "customer-source") {
    void dashboard.addCustomerSourceOption(label);
    return;
  }

  void dashboard.addProfessionOption(label);
}

function updateOption(group, optionId, label) {
  if (group === "visit-reason") {
    void dashboard.updateVisitReasonOption(optionId, label);
    return;
  }

  if (group === "customer-source") {
    void dashboard.updateCustomerSourceOption(optionId, label);
    return;
  }

  void dashboard.updateProfessionOption(optionId, label);
}

function removeOption(group, optionId) {
  if (group === "visit-reason") {
    void dashboard.removeVisitReasonOption(optionId);
    return;
  }

  if (group === "customer-source") {
    void dashboard.removeCustomerSourceOption(optionId);
    return;
  }

  void dashboard.removeProfessionOption(optionId);
}

function addProduct(payload) {
  void dashboard.addCatalogProduct(payload.name, payload.category, payload.basePrice);
}

function updateProduct(productId, payload) {
  void dashboard.updateCatalogProduct(productId, payload);
}

function removeProduct(productId) {
  void dashboard.removeCatalogProduct(productId);
}

async function addConsultant(payload) {
  const result = await dashboard.createConsultantProfile(payload);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel criar consultor.");
    return;
  }

  ui.success("Consultor criado.");
}

async function updateConsultant(consultantId, payload) {
  const result = await dashboard.updateConsultantProfile(consultantId, payload);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel atualizar consultor.");
    return;
  }

  ui.success("Consultor atualizado.");
}

async function archiveConsultant(consultantId) {
  const { confirmed } = await ui.confirm({
    title: "Arquivar consultor",
    message: "O consultor sera removido da escala ativa. Deseja continuar?",
    confirmLabel: "Arquivar"
  });

  if (!confirmed) {
    return;
  }

  const result = await dashboard.archiveConsultantProfile(consultantId);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel arquivar consultor.");
    return;
  }

  ui.success("Consultor arquivado.");
}
</script>

<template>
  <section class="admin-panel" data-testid="settings-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Configuracoes</h2>
    </header>

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <div v-if="activeTab === 'operacao'">
      <SettingsOperationTemplateManager
        :templates="state.operationTemplates || []"
        :selected-operation-template-id="state.selectedOperationTemplateId"
        :disabled="!canEditSettings"
        @apply="applyTemplate"
      />

      <div class="settings-grid" style="margin-top: 16px;">
        <article class="settings-card">
          <header class="settings-card__header">
            <h3 class="settings-card__title">Limites e timings</h3>
          </header>
          <label class="settings-field"><span>Atendimentos simultaneos</span><input :value="Number(state.settings.maxConcurrentServices || 10)" type="number" min="1" max="100" :disabled="!canEditSettings" @input="updateNumericSetting('maxConcurrentServices', $event.target.value)"></label>
          <label class="settings-field"><span>Fechamento rapido (min)</span><input :value="Number(state.settings.timingFastCloseMinutes || 5)" type="number" min="1" max="120" :disabled="!canEditSettings" @input="updateNumericSetting('timingFastCloseMinutes', $event.target.value)"></label>
          <label class="settings-field"><span>Atendimento demorado (min)</span><input :value="Number(state.settings.timingLongServiceMinutes || 25)" type="number" min="1" max="240" :disabled="!canEditSettings" @input="updateNumericSetting('timingLongServiceMinutes', $event.target.value)"></label>
          <label class="settings-field"><span>Venda baixa (R$)</span><input :value="Number(state.settings.timingLowSaleAmount || 1200)" type="number" min="1" step="1" :disabled="!canEditSettings" @input="updateNumericSetting('timingLowSaleAmount', $event.target.value)"></label>
          <label class="settings-toggle"><input :checked="Boolean(state.settings.testModeEnabled)" type="checkbox" :disabled="!canEditSettings" @change="updateBooleanSetting('testModeEnabled', $event.target.checked)"><span>Modo teste</span></label>
          <label class="settings-toggle"><input :checked="Boolean(state.settings.autoFillFinishModal)" type="checkbox" :disabled="!canEditSettings" @change="updateBooleanSetting('autoFillFinishModal', $event.target.checked)"><span>Preencher modal automaticamente</span></label>
        </article>
      </div>
    </div>

    <div v-if="activeTab === 'modal'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header"><h3 class="settings-card__title">Textos</h3></header>
        <label class="settings-field"><span>Titulo do modal</span><input :value="state.modalConfig.title" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('title', $event.target.value)"></label>
        <label class="settings-field"><span>Label da secao de cliente</span><input :value="state.modalConfig.customerSectionLabel" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('customerSectionLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Label observacoes</span><input :value="state.modalConfig.notesLabel" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('notesLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder observacoes</span><input :value="state.modalConfig.notesPlaceholder" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('notesPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label motivo fora da vez</span><input :value="state.modalConfig.queueJumpReasonLabel" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('queueJumpReasonLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder motivo fora da vez</span><input :value="state.modalConfig.queueJumpReasonPlaceholder" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('queueJumpReasonPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label produto visto</span><input :value="state.modalConfig.productSeenLabel" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('productSeenLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder produto visto</span><input :value="state.modalConfig.productSeenPlaceholder" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('productSeenPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label produto fechado</span><input :value="state.modalConfig.productClosedLabel" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('productClosedLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder produto fechado</span><input :value="state.modalConfig.productClosedPlaceholder" type="text" :disabled="!canEditSettings" @input="updateModalConfigValue('productClosedPlaceholder', $event.target.value)"></label>
      </article>

      <article class="settings-card">
        <header class="settings-card__header"><h3 class="settings-card__title">Campos e validacoes</h3></header>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showEmailField)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showEmailField', $event.target.checked)"><span>Mostrar email</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showProfessionField)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showProfessionField', $event.target.checked)"><span>Mostrar profissao</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showNotesField)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showNotesField', $event.target.checked)"><span>Mostrar observacoes</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showVisitReasonDetails)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showVisitReasonDetails', $event.target.checked)"><span>Detalhe por motivo de visita</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showCustomerSourceDetails)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showCustomerSourceDetails', $event.target.checked)"><span>Detalhe por origem</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.requireProduct)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('requireProduct', $event.target.checked)"><span>Exigir produto</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.requireVisitReason)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('requireVisitReason', $event.target.checked)"><span>Exigir motivo da visita</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.requireCustomerSource)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('requireCustomerSource', $event.target.checked)"><span>Exigir origem do cliente</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.requireCustomerNamePhone)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('requireCustomerNamePhone', $event.target.checked)"><span>Exigir nome e telefone</span></label>
      </article>
    </div>

    <div v-if="activeTab === 'produtos'">
      <SettingsProductManager
        :products="state.productCatalog || []"
        @add="addProduct"
        @update="updateProduct"
        @remove="removeProduct"
      />
    </div>

    <div v-if="activeTab === 'consultores'">
      <SettingsConsultantManager
        :consultants="state.roster || []"
        :disabled="!canEditConsultants"
        @add="addConsultant"
        @update="updateConsultant"
        @archive="archiveConsultant"
      />
    </div>

    <div v-if="activeTab === 'motivos'">
      <SettingsOptionManager
        title="Motivo da visita"
        description="Opcoes exibidas no modal de fechamento."
        :items="state.visitReasonOptions || []"
        add-placeholder="Adicionar nova opcao"
        testid="settings-motivos"
        @add="addOption('visit-reason', $event)"
        @update="(optionId, label) => updateOption('visit-reason', optionId, label)"
        @remove="removeOption('visit-reason', $event)"
      />
    </div>

    <div v-if="activeTab === 'origens'">
      <SettingsOptionManager
        title="Origem do cliente"
        description="Opcoes exibidas no modal de fechamento."
        :items="state.customerSourceOptions || []"
        add-placeholder="Adicionar nova opcao"
        testid="settings-origens"
        @add="addOption('customer-source', $event)"
        @update="(optionId, label) => updateOption('customer-source', optionId, label)"
        @remove="removeOption('customer-source', $event)"
      />
    </div>

    <div v-if="activeTab === 'profissoes'">
      <SettingsOptionManager
        title="Profissoes"
        description="Lista usada no modal. Se nao existir, tambem pode cadastrar na hora no fechamento."
        :items="state.professionOptions || []"
        add-placeholder="Adicionar nova profissao"
        testid="settings-profissoes"
        @add="addOption('profession', $event)"
        @update="(optionId, label) => updateOption('profession', optionId, label)"
        @remove="removeOption('profession', $event)"
      />
    </div>

    <div v-if="activeTab === 'alertas'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Limites de desempenho</h3>
          <p class="settings-card__text">
            Consultores que ficarem abaixo (ou acima) desses limites no mes atual aparecem como alertas em /ranking.
            Deixe em 0 para desativar.
          </p>
        </header>
        <label class="settings-field">
          <span>Conversao minima (%)</span>
          <input :value="Number(state.settings.alertMinConversionRate || 0)" type="number" min="0" max="100" step="1" :disabled="!canEditSettings" @input="updateNumericSetting('alertMinConversionRate', $event.target.value)">
        </label>
        <label class="settings-field">
          <span>Fora da vez maximo (%)</span>
          <input :value="Number(state.settings.alertMaxQueueJumpRate || 0)" type="number" min="0" max="100" step="1" :disabled="!canEditSettings" @input="updateNumericSetting('alertMaxQueueJumpRate', $event.target.value)">
        </label>
        <label class="settings-field">
          <span>P.A. minimo</span>
          <input :value="Number(state.settings.alertMinPaScore || 0)" type="number" min="0" step="0.1" :disabled="!canEditSettings" @input="updateNumericSetting('alertMinPaScore', $event.target.value)">
        </label>
        <label class="settings-field">
          <span>Ticket medio minimo (R$)</span>
          <input :value="Number(state.settings.alertMinTicketAverage || 0)" type="number" min="0" step="100" :disabled="!canEditSettings" @input="updateNumericSetting('alertMinTicketAverage', $event.target.value)">
        </label>
      </article>
    </div>
  </section>
</template>
