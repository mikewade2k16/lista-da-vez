<script setup>
import { computed, ref } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { canManageConsultants, canManageSettings } from "~/domain/utils/permissions";
import SettingsConsultantManager from "~/components/settings/SettingsConsultantManager.vue";
import SettingsOperationTemplateManager from "~/components/settings/SettingsOperationTemplateManager.vue";
import SettingsOptionManager from "~/components/settings/SettingsOptionManager.vue";
import SettingsProductManager from "~/components/settings/SettingsProductManager.vue";
import SettingsTabs from "~/components/settings/SettingsTabs.vue";
import { useConsultantsStore } from "~/stores/consultants";
import { useSettingsStore } from "~/stores/settings";
import { useUiStore } from "~/stores/ui";

const tabs = [
  { id: "operacao", label: "Operacao", icon: "tune" },
  { id: "modal", label: "Modal", icon: "edit_note" },
  { id: "produtos", label: "Produtos", icon: "inventory_2" },
  { id: "consultores", label: "Consultores", icon: "group" },
  { id: "motivos", label: "Motivos", icon: "fact_check" },
  { id: "motivos-perda", label: "Perdas", icon: "trending_down" },
  { id: "motivos-fora-da-vez", label: "Fora da vez", icon: "bolt" },
  { id: "origens", label: "Origens", icon: "share_location" },
  { id: "profissoes", label: "Profissoes", icon: "badge" },
  { id: "alertas", label: "Alertas", icon: "notifications_active" }
];
const fieldSelectionOptions = [
  { value: "single", label: "Escolha unica" },
  { value: "multiple", label: "Multiplas escolhas" }
];
const fieldDetailModeOptions = [
  { value: "off", label: "Sem descricao" },
  { value: "shared", label: "Uma descricao para a selecao" },
  { value: "per-item", label: "Uma descricao por opcao" }
];

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const settingsStore = useSettingsStore();
const consultantsStore = useConsultantsStore();
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

async function updateNumericSetting(settingId, value) {
  const result = await settingsStore.updateSetting(settingId, Number(value) || 0);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel salvar a configuracao.");
  }
}

async function updateBooleanSetting(settingId, value) {
  const result = await settingsStore.updateSetting(settingId, Boolean(value));

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel salvar a configuracao.");
  }
}

async function updateModalConfigValue(configKey, value) {
  const result = await settingsStore.updateModalConfig(configKey, value);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel salvar a configuracao do modal.");
  }
}

async function applyTemplate(templateId) {
  const result = await settingsStore.applyOperationTemplate(templateId);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel aplicar o template.");
  }
}

function handleMutationResult(result, successMessage, fallbackErrorMessage) {
  if (result?.ok === false) {
    ui.error(result.message || fallbackErrorMessage);
    return false;
  }

  if (successMessage) {
    ui.success(successMessage);
  }

  return true;
}

async function addOption(group, label) {
  if (group === "visit-reason") {
    handleMutationResult(await settingsStore.addVisitReasonOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
    return;
  }

  if (group === "customer-source") {
    handleMutationResult(await settingsStore.addCustomerSourceOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
    return;
  }

  if (group === "queue-jump-reason") {
    handleMutationResult(await settingsStore.addQueueJumpReasonOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
    return;
  }

  if (group === "loss-reason") {
    handleMutationResult(await settingsStore.addLossReasonOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
    return;
  }

  handleMutationResult(await settingsStore.addProfessionOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
}

async function updateOption(group, optionId, label) {
  if (group === "visit-reason") {
    handleMutationResult(await settingsStore.updateVisitReasonOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
    return;
  }

  if (group === "customer-source") {
    handleMutationResult(await settingsStore.updateCustomerSourceOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
    return;
  }

  if (group === "queue-jump-reason") {
    handleMutationResult(await settingsStore.updateQueueJumpReasonOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
    return;
  }

  if (group === "loss-reason") {
    handleMutationResult(await settingsStore.updateLossReasonOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
    return;
  }

  handleMutationResult(await settingsStore.updateProfessionOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
}

async function removeOption(group, optionId) {
  if (group === "visit-reason") {
    handleMutationResult(await settingsStore.removeVisitReasonOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
    return;
  }

  if (group === "customer-source") {
    handleMutationResult(await settingsStore.removeCustomerSourceOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
    return;
  }

  if (group === "queue-jump-reason") {
    handleMutationResult(await settingsStore.removeQueueJumpReasonOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
    return;
  }

  if (group === "loss-reason") {
    handleMutationResult(await settingsStore.removeLossReasonOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
    return;
  }

  handleMutationResult(await settingsStore.removeProfessionOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
}

async function addProduct(payload) {
  handleMutationResult(
    await settingsStore.addCatalogProduct(payload.name, payload.category, payload.basePrice, payload.code),
    "Produto adicionado.",
    "Nao foi possivel adicionar o produto."
  );
}

async function updateProduct(productId, payload) {
  handleMutationResult(
    await settingsStore.updateCatalogProduct(productId, payload),
    "Produto atualizado.",
    "Nao foi possivel atualizar o produto."
  );
}

async function removeProduct(productId) {
  handleMutationResult(
    await settingsStore.removeCatalogProduct(productId),
    "Produto removido.",
    "Nao foi possivel remover o produto."
  );
}

async function addConsultant(payload) {
  const result = await consultantsStore.createConsultantProfile(payload);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel criar consultor.");
    return;
  }

  const accessEmail = String(result?.access?.email || "").trim();
  const initialPassword = String(result?.access?.initialPassword || "").trim();

  if (accessEmail && initialPassword) {
    await ui.prompt({
      title: "Acesso do consultor criado",
      message: `Login padrao: ${accessEmail}\nSenha inicial: ${initialPassword}\nOriente o consultor a trocar a senha em Perfil no primeiro acesso.`,
      inputLabel: "Acesso",
      initialValue: `${accessEmail} | ${initialPassword}`,
      confirmLabel: "Fechar"
    });
  }

  ui.success("Consultor criado com acesso vinculado.");
}

async function updateConsultant(consultantId, payload) {
  const result = await consultantsStore.updateConsultantProfile(consultantId, payload);

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

  const result = await consultantsStore.archiveConsultantProfile(consultantId);

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
          <label class="settings-field"><span>Atendimentos simultaneos</span><input :value="Number(state.settings.maxConcurrentServices || 10)" type="number" min="1" max="100" :disabled="!canEditSettings" @change="updateNumericSetting('maxConcurrentServices', $event.target.value)"></label>
          <label class="settings-field"><span>Fechamento rapido (min)</span><input :value="Number(state.settings.timingFastCloseMinutes || 5)" type="number" min="1" max="120" :disabled="!canEditSettings" @change="updateNumericSetting('timingFastCloseMinutes', $event.target.value)"></label>
          <label class="settings-field"><span>Atendimento demorado (min)</span><input :value="Number(state.settings.timingLongServiceMinutes || 25)" type="number" min="1" max="240" :disabled="!canEditSettings" @change="updateNumericSetting('timingLongServiceMinutes', $event.target.value)"></label>
          <label class="settings-field"><span>Venda baixa (R$)</span><input :value="Number(state.settings.timingLowSaleAmount || 1200)" type="number" min="1" step="1" :disabled="!canEditSettings" @change="updateNumericSetting('timingLowSaleAmount', $event.target.value)"></label>
          <label class="settings-toggle"><input :checked="Boolean(state.settings.testModeEnabled)" type="checkbox" :disabled="!canEditSettings" @change="updateBooleanSetting('testModeEnabled', $event.target.checked)"><span>Modo teste</span></label>
          <label class="settings-toggle"><input :checked="Boolean(state.settings.autoFillFinishModal)" type="checkbox" :disabled="!canEditSettings" @change="updateBooleanSetting('autoFillFinishModal', $event.target.checked)"><span>Preencher modal automaticamente</span></label>
        </article>
      </div>
    </div>

    <div v-if="activeTab === 'modal'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header"><h3 class="settings-card__title">Textos</h3></header>
        <label class="settings-field"><span>Titulo do modal</span><input :value="state.modalConfig.title" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('title', $event.target.value)"></label>
        <label class="settings-field"><span>Label da secao de cliente</span><input :value="state.modalConfig.customerSectionLabel" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('customerSectionLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Label observacoes</span><input :value="state.modalConfig.notesLabel" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('notesLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder observacoes</span><input :value="state.modalConfig.notesPlaceholder" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('notesPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label motivo fora da vez</span><input :value="state.modalConfig.queueJumpReasonLabel" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('queueJumpReasonLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder busca motivo fora da vez</span><input :value="state.modalConfig.queueJumpReasonPlaceholder" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('queueJumpReasonPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label motivo da perda</span><input :value="state.modalConfig.lossReasonLabel" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('lossReasonLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder busca motivo da perda</span><input :value="state.modalConfig.lossReasonPlaceholder" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('lossReasonPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label produto visto</span><input :value="state.modalConfig.productSeenLabel" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('productSeenLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder produto visto</span><input :value="state.modalConfig.productSeenPlaceholder" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('productSeenPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label produto fechado</span><input :value="state.modalConfig.productClosedLabel" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('productClosedLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder produto fechado</span><input :value="state.modalConfig.productClosedPlaceholder" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('productClosedPlaceholder', $event.target.value)"></label>
      </article>

      <article class="settings-card">
        <header class="settings-card__header"><h3 class="settings-card__title">Campos e validacoes</h3></header>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showEmailField)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showEmailField', $event.target.checked)"><span>Mostrar email</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showProfessionField)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showProfessionField', $event.target.checked)"><span>Mostrar profissao</span></label>
        <label class="settings-toggle"><input :checked="Boolean(state.modalConfig.showNotesField)" type="checkbox" :disabled="!canEditSettings" @change="updateModalConfigValue('showNotesField', $event.target.checked)"><span>Mostrar observacoes</span></label>
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

    <div v-if="activeTab === 'motivos'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Comportamento do campo</h3>
          <p class="settings-card__text">Defina aqui como o campo aparece no modal antes de cadastrar as opcoes.</p>
        </header>
        <AppSelectField
          class="settings-field"
          label="Selecao"
          :model-value="state.modalConfig.visitReasonSelectionMode || 'multiple'"
          :options="fieldSelectionOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('visitReasonSelectionMode', $event)"
        />
        <AppSelectField
          class="settings-field"
          label="Descricao"
          :model-value="state.modalConfig.visitReasonDetailMode || 'shared'"
          :options="fieldDetailModeOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('visitReasonDetailMode', $event)"
        />
      </article>

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

    <div v-if="activeTab === 'motivos-fora-da-vez'">
      <SettingsOptionManager
        title="Motivo fora da vez"
        description="Opcoes obrigatorias exibidas quando o atendimento for encerrado fora da vez."
        :items="state.queueJumpReasonOptions || []"
        add-placeholder="Adicionar novo motivo fora da vez"
        testid="settings-fora-da-vez"
        @add="addOption('queue-jump-reason', $event)"
        @update="(optionId, label) => updateOption('queue-jump-reason', optionId, label)"
        @remove="removeOption('queue-jump-reason', $event)"
      />
    </div>

    <div v-if="activeTab === 'motivos-perda'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Comportamento do campo</h3>
          <p class="settings-card__text">Defina aqui como o campo aparece quando o atendimento termina sem venda.</p>
        </header>
        <AppSelectField
          class="settings-field"
          label="Selecao"
          :model-value="state.modalConfig.lossReasonSelectionMode || 'single'"
          :options="fieldSelectionOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('lossReasonSelectionMode', $event)"
        />
        <AppSelectField
          class="settings-field"
          label="Descricao"
          :model-value="state.modalConfig.lossReasonDetailMode || 'off'"
          :options="fieldDetailModeOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('lossReasonDetailMode', $event)"
        />
      </article>

      <SettingsOptionManager
        title="Motivo da perda"
        description="Opcoes exibidas quando o atendimento termina sem venda."
        :items="state.lossReasonOptions || []"
        add-placeholder="Adicionar novo motivo da perda"
        testid="settings-motivos-perda"
        @add="addOption('loss-reason', $event)"
        @update="(optionId, label) => updateOption('loss-reason', optionId, label)"
        @remove="removeOption('loss-reason', $event)"
      />
    </div>

    <div v-if="activeTab === 'origens'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Comportamento do campo</h3>
          <p class="settings-card__text">Defina aqui como o campo aparece no modal antes de cadastrar as opcoes.</p>
        </header>
        <AppSelectField
          class="settings-field"
          label="Selecao"
          :model-value="state.modalConfig.customerSourceSelectionMode || 'single'"
          :options="fieldSelectionOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('customerSourceSelectionMode', $event)"
        />
        <AppSelectField
          class="settings-field"
          label="Descricao"
          :model-value="state.modalConfig.customerSourceDetailMode || 'shared'"
          :options="fieldDetailModeOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('customerSourceDetailMode', $event)"
        />
      </article>

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
          <input :value="Number(state.settings.alertMinConversionRate || 0)" type="number" min="0" max="100" step="1" :disabled="!canEditSettings" @change="updateNumericSetting('alertMinConversionRate', $event.target.value)">
        </label>
        <label class="settings-field">
          <span>Fora da vez maximo (%)</span>
          <input :value="Number(state.settings.alertMaxQueueJumpRate || 0)" type="number" min="0" max="100" step="1" :disabled="!canEditSettings" @change="updateNumericSetting('alertMaxQueueJumpRate', $event.target.value)">
        </label>
        <label class="settings-field">
          <span>P.A. minimo</span>
          <input :value="Number(state.settings.alertMinPaScore || 0)" type="number" min="0" step="0.1" :disabled="!canEditSettings" @change="updateNumericSetting('alertMinPaScore', $event.target.value)">
        </label>
        <label class="settings-field">
          <span>Ticket medio minimo (R$)</span>
          <input :value="Number(state.settings.alertMinTicketAverage || 0)" type="number" min="0" step="100" :disabled="!canEditSettings" @change="updateNumericSetting('alertMinTicketAverage', $event.target.value)">
        </label>
      </article>
    </div>
  </section>
</template>
