<script setup>
import { computed, ref } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import AppToggleSwitch from "~/components/ui/AppToggleSwitch.vue";
import { canManageConsultants, canManageSettings } from "~/domain/utils/permissions";
import SettingsConsultantManager from "~/components/settings/SettingsConsultantManager.vue";
import SettingsOperationTemplateManager from "~/components/settings/SettingsOperationTemplateManager.vue";
import SettingsOptionManager from "~/components/settings/SettingsOptionManager.vue";
import SettingsProductManager from "~/components/settings/SettingsProductManager.vue";
import SettingsTabs from "~/components/settings/SettingsTabs.vue";
import { useAuthStore } from "~/stores/auth";
import { useConsultantsStore } from "~/stores/consultants";
import { useSettingsStore } from "~/stores/settings";
import { useUiStore } from "~/stores/ui";

const tabs = [
  { id: "operacao", label: "Operacao", icon: "tune" },
  { id: "modal", label: "Modal", icon: "edit_note" },
  { id: "produtos", label: "Produtos", icon: "inventory_2" },
  { id: "consultores", label: "Consultores", icon: "group" },
  { id: "motivos", label: "Motivos", icon: "fact_check" },
  { id: "cancelamento", label: "Cancelamento", icon: "undo" },
  { id: "parada", label: "Parada", icon: "pause" },
  { id: "pausas", label: "Pausas", icon: "pause_circle" },
  { id: "motivos-perda", label: "Perdas", icon: "trending_down" },
  { id: "motivos-fora-da-vez", label: "Fora da vez", icon: "bolt" },
  { id: "origens", label: "Origens", icon: "share_location" },
  { id: "profissoes", label: "Profissoes", icon: "badge" },
  { id: "alertas", label: "Alertas", icon: "notifications_active" }
];
// A tela de Parada segue implementada abaixo, mas fica fora do menu por enquanto.
const hiddenSettingsTabs = new Set(["parada"]);
const visibleTabs = computed(() => tabs.filter((tab) => !hiddenSettingsTabs.has(tab.id)));
const fieldSelectionOptions = [
  { value: "single", label: "Escolha unica" },
  { value: "multiple", label: "Multiplas escolhas" }
];
const fieldDetailModeOptions = [
  { value: "off", label: "Sem descricao" },
  { value: "shared", label: "Uma descricao para a selecao" },
  { value: "per-item", label: "Uma descricao por opcao" }
];
const reasonInputModeOptions = [
  { value: "text", label: "Texto livre" },
  { value: "select", label: "Apenas lista" },
  { value: "select-with-other", label: "Lista com Outro" }
];
const modalFinishFlowOptions = [
  { value: "legacy", label: "Modal atual" },
  { value: "erp-reconciliation", label: "Modal conciliacao ERP" }
];

function withFieldJustification(field, baseKey) {
  const normalizedBaseKey = String(baseKey || "").trim();

  if (!normalizedBaseKey) {
    return field;
  }

  const configPrefix = `${normalizedBaseKey.charAt(0).toLowerCase()}${normalizedBaseKey.slice(1)}`;

  return {
    ...field,
    justificationRequiredKey: `require${normalizedBaseKey}Justification`,
    justificationMinCharsKey: `${configPrefix}JustificationMinChars`
  };
}

const modalFieldSections = [
  {
    id: "customer",
    title: "Dados do cliente",
    description: "Campos basicos do passo 2 para identificar e qualificar o cliente.",
    defaultOpen: true,
    fields: [
      withFieldJustification({
        id: "customer-name",
        label: "Nome do cliente",
        labelKey: "customerNameLabel",
        description: "Campo de texto exibido no topo da secao de cliente.",
        showKey: "showCustomerNameField",
        requiredKey: "requireCustomerNameField",
        requiredDefault: true,
        legacyRequiredKey: "requireCustomerNamePhone"
      }, "CustomerName"),
      withFieldJustification({
        id: "customer-phone",
        label: "Telefone",
        labelKey: "customerPhoneLabel",
        description: "Usado para contato e reaproveitamento do atendimento.",
        showKey: "showCustomerPhoneField",
        requiredKey: "requireCustomerPhoneField",
        requiredDefault: true,
        legacyRequiredKey: "requireCustomerNamePhone"
      }, "CustomerPhone"),
      withFieldJustification({
        id: "customer-email",
        label: "E-mail",
        labelKey: "customerEmailLabel",
        description: "Captura complementar para relacionamento.",
        showKey: "showEmailField",
        requiredKey: "requireEmailField",
        requiredDefault: false
      }, "Email"),
      withFieldJustification({
        id: "customer-profession",
        label: "Profissão",
        labelKey: "customerProfessionLabel",
        description: "Usa o catalogo configurado na aba de profissoes.",
        showKey: "showProfessionField",
        requiredKey: "requireProfessionField",
        requiredDefault: false
      }, "Profession"),
      withFieldJustification({
        id: "existing-customer",
        label: "Já era cliente",
        labelKey: "existingCustomerLabel",
        description: "Vai para o passo 2 para apoiar a busca automatica de cadastro do cliente.",
        showKey: "showExistingCustomerField"
      }, "ExistingCustomer"),
      withFieldJustification({
        id: "notes",
        label: "Observações",
        labelKey: "notesLabel",
        description: "Campo livre para contexto adicional do atendimento.",
        showKey: "showNotesField",
        requiredKey: "requireNotesField",
        requiredDefault: false
      }, "Notes")
    ]
  },
  {
    id: "journey",
    title: "Produtos e jornada",
    description: "Campos principais do atendimento e da origem do cliente.",
    defaultOpen: true,
    fields: [
      withFieldJustification({
        id: "product-closed",
        label: "Compra / Reserva",
        labelKey: "productClosedLabel",
        description: "Aparece primeiro no passo 1 quando o desfecho for compra ou reserva.",
        showKey: "showProductClosedField",
        requiredKey: "requireProductClosedField",
        requiredDefault: true,
        legacyRequiredKey: "requireProduct"
      }, "ProductClosed"),
      withFieldJustification({
        id: "purchase-code",
        label: "Codigo da compra",
        labelKey: "purchaseCodeLabel",
        description: "No fluxo ERP, aparece apenas para compra e guarda a referencia para conciliacao no dia seguinte.",
        showKey: "showPurchaseCodeField",
        requiredKey: "requirePurchaseCodeField",
        requiredDefault: true
      }, "PurchaseCode"),
      withFieldJustification({
        id: "product-seen",
        label: "Interesses do cliente",
        labelKey: "productSeenLabel",
        description: "Aparece no passo 1 para mapear os interesses vistos ou desejados.",
        showKey: "showProductSeenField",
        requiredKey: "requireProductSeenField",
        requiredDefault: true,
        legacyRequiredKey: "requireProduct"
      }, "ProductSeen"),
      withFieldJustification({
        id: "product-seen-notes",
        label: "Observação dos interesses",
        labelKey: "productSeenNotesLabel",
        description: "Campo complementar para contexto, referencia ou justificativa quando nao houver item.",
        showKey: "showProductSeenNotesField",
        requiredKey: "requireProductSeenNotesField",
        requiredDefault: false
      }, "ProductSeenNotes"),
      withFieldJustification({
        id: "visit-reason",
        label: "Motivo da visita",
        labelKey: "visitReasonLabel",
        description: "Ajuda a entender a intencao do cliente na chegada.",
        showKey: "showVisitReasonField",
        requiredKey: "requireVisitReason",
        requiredDefault: true
      }, "VisitReason"),
      withFieldJustification({
        id: "customer-source",
        label: "Origem do cliente",
        labelKey: "customerSourceLabel",
        description: "Relaciona o atendimento ao canal de entrada.",
        showKey: "showCustomerSourceField",
        requiredKey: "requireCustomerSource",
        requiredDefault: true
      }, "CustomerSource")
    ]
  },
  {
    id: "conditional",
    title: "Campos condicionais",
    description: "Campos que so entram em cenarios especificos de encerramento.",
    defaultOpen: false,
    fields: [
      withFieldJustification({
        id: "queue-jump-reason",
        label: "Motivo fora da vez",
        labelKey: "queueJumpReasonLabel",
        description: "Exibido quando o atendimento comeca fora da fila.",
        showKey: "showQueueJumpReasonField",
        requiredKey: "requireQueueJumpReasonField",
        requiredDefault: true
      }, "QueueJumpReason"),
      withFieldJustification({
        id: "loss-reason",
        label: "Motivo da perda",
        labelKey: "lossReasonLabel",
        description: "Exibido quando o desfecho for nao compra.",
        showKey: "showLossReasonField",
        requiredKey: "requireLossReasonField",
        requiredDefault: true
      }, "LossReason")
    ]
  }
];
const modalTextSections = [
  {
    id: "general",
    title: "Textos gerais",
    description: "Titulos base do modal e da secao de cliente.",
    defaultOpen: true,
    fields: [
      { key: "title", label: "Titulo do modal" },
      { key: "customerSectionLabel", label: "Label da secao de cliente" }
    ]
  },
  {
    id: "products",
    title: "Textos de produto",
    description: "Copys exibidas nos blocos de produto visto e produto fechado.",
    defaultOpen: false,
    fields: [
      { key: "productSeenLabel", label: "Label interesses do cliente" },
      { key: "productSeenPlaceholder", label: "Placeholder interesses do cliente" },
      { key: "productClosedLabel", label: "Label fechamento (opcional)" },
      { key: "productClosedPlaceholder", label: "Placeholder compra / reserva" },
      { key: "purchaseCodePlaceholder", label: "Placeholder codigo da compra" }
    ]
  },
  {
    id: "support",
    title: "Textos de apoio",
    description: "Textos auxiliares de observacoes, perda e fora da vez.",
    defaultOpen: false,
    fields: [
      { key: "notesLabel", label: "Label observações" },
      { key: "notesPlaceholder", label: "Placeholder observações" },
      { key: "queueJumpReasonLabel", label: "Label motivo fora da vez" },
      { key: "queueJumpReasonPlaceholder", label: "Placeholder motivo fora da vez" },
      { key: "lossReasonLabel", label: "Label motivo da perda" },
      { key: "lossReasonPlaceholder", label: "Placeholder motivo da perda" },
      { key: "cancelReasonLabel", label: "Label motivo do cancelamento" },
      { key: "cancelReasonPlaceholder", label: "Placeholder motivo do cancelamento" },
      { key: "stopReasonLabel", label: "Label motivo da parada" },
      { key: "stopReasonPlaceholder", label: "Placeholder motivo da parada" }
    ]
  }
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
const auth = useAuthStore();
const runtimeSettingsNotice = computed(() => String(auth.runtimeSettingsNotice || "").trim());
const activeTab = ref("operacao");
const modalConfigState = computed(() => props.state.modalConfig || {});

const activeRole = computed(() => {
  const activeProfile =
    (props.state.profiles || []).find((profile) => profile.id === props.state.activeProfileId) ||
    props.state.profiles?.[0] ||
    null;

  return activeProfile?.role || "consultant";
});
const canEditSettings = computed(() => canManageSettings(auth.role, auth.permissionKeys, auth.permissionsResolved));
const canEditConsultants = computed(() => canManageConsultants(auth.role, auth.permissionKeys, auth.permissionsResolved));
const maxParallelPerConsultantLimit = computed(() =>
  Math.min(5, Math.max(1, Number(props.state.settings?.maxConcurrentServices || 1) || 1))
);

async function updateNumericSetting(settingId, value) {
  const numericValue = Number(value);

  if (settingId === "maxConcurrentServicesPerConsultant") {
    if (!Number.isFinite(numericValue) || numericValue < 1 || numericValue > maxParallelPerConsultantLimit.value) {
      ui.error(`Atendimentos em aberto por consultor deve ficar entre 1 e ${maxParallelPerConsultantLimit.value}.`);
      return;
    }
  }

  const result = await settingsStore.updateSetting(settingId, Number.isFinite(numericValue) ? numericValue : 0);

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

  return result || { ok: true };
}

async function updateModalConfigNumberValue(configKey, value, minimum = 0) {
  const normalizedValue = Math.max(minimum, Math.trunc(Number(value) || 0));
  return updateModalConfigValue(configKey, normalizedValue);
}

function getModalNumberValue(configKey, fallback = 0, minimum = 0) {
  const normalizedFallback = Math.max(minimum, Math.trunc(Number(fallback) || 0));

  if (!configKey) {
    return normalizedFallback;
  }

  const numericValue = Math.trunc(Number(modalConfigState.value?.[configKey]));

  if (!Number.isFinite(numericValue) || numericValue < minimum) {
    return normalizedFallback;
  }

  return numericValue;
}

function getModalTextValue(configKey, fallback = "") {
  if (!configKey) {
    return fallback;
  }

  const configuredValue = String(modalConfigState.value?.[configKey] || "").trim();
  return configuredValue || fallback;
}

function getModalBooleanValue(configKey, fallback = false, legacyConfigKey = "") {
  const directValue = modalConfigState.value?.[configKey];

  if (typeof directValue === "boolean") {
    return directValue;
  }

  if (legacyConfigKey) {
    const legacyValue = modalConfigState.value?.[legacyConfigKey];

    if (typeof legacyValue === "boolean") {
      return legacyValue;
    }
  }

  return fallback;
}

function getFinishFlowMode() {
  const configuredValue = String(modalConfigState.value?.finishFlowMode || "").trim();
  return configuredValue === "erp-reconciliation" ? "erp-reconciliation" : "legacy";
}

function isModalFieldVisible(field) {
  return getModalBooleanValue(field.showKey, field.showDefault ?? true, field.legacyShowKey || "");
}

function isModalFieldRequired(field) {
  if (!field.requiredKey) {
    return false;
  }

  return getModalBooleanValue(field.requiredKey, field.requiredDefault ?? false, field.legacyRequiredKey || "");
}

async function handleModalFieldVisibilityChange(field, nextValue) {
  await updateModalConfigValue(field.showKey, nextValue);

  if (!nextValue && field.requiredKey) {
    await updateModalConfigValue(field.requiredKey, false);
  }

  if (!nextValue && field.justificationRequiredKey) {
    await updateModalConfigValue(field.justificationRequiredKey, false);
  }
}

async function handleModalFieldRequiredChange(field, nextValue) {
  if (!field.requiredKey || !isModalFieldVisible(field)) {
    return;
  }

  await updateModalConfigValue(field.requiredKey, nextValue);
}

function isModalFieldJustificationRequired(field) {
  if (!field.justificationRequiredKey) {
    return false;
  }

  return getModalBooleanValue(field.justificationRequiredKey, false);
}

function getModalFieldJustificationMinChars(field) {
  return getModalNumberValue(field.justificationMinCharsKey, 20, 1);
}

async function handleModalFieldJustificationChange(field, nextValue) {
  if (!field.justificationRequiredKey || !isModalFieldVisible(field)) {
    return;
  }

  await updateModalConfigValue(field.justificationRequiredKey, nextValue);
}

async function handleModalFieldLabelChange(field, value) {
  if (!field.labelKey) {
    return;
  }

  const normalizedValue = String(value || "").trim();
  await updateModalConfigValue(field.labelKey, normalizedValue || field.label);
}

function getModalFieldSectionSummary(section) {
  const visibleCount = section.fields.filter((field) => isModalFieldVisible(field)).length;
  const requiredCount = section.fields.filter((field) => isModalFieldVisible(field) && isModalFieldRequired(field)).length;

  return `${visibleCount}/${section.fields.length} visiveis · ${requiredCount} obrigatorios`;
}

function getModalTextSectionSummary(section) {
  const filledCount = section.fields.filter((field) => String(modalConfigState.value?.[field.key] || "").trim()).length;

  return `${filledCount}/${section.fields.length} preenchidos`;
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

  if (group === "pause-reason") {
    handleMutationResult(await settingsStore.addPauseReasonOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
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

  if (group === "cancel-reason") {
    handleMutationResult(await settingsStore.addCancelReasonOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
    return;
  }

  if (group === "stop-reason") {
    handleMutationResult(await settingsStore.addStopReasonOption(label), "Opcao adicionada.", "Nao foi possivel adicionar a opcao.");
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

  if (group === "pause-reason") {
    handleMutationResult(await settingsStore.updatePauseReasonOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
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

  if (group === "cancel-reason") {
    handleMutationResult(await settingsStore.updateCancelReasonOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
    return;
  }

  if (group === "stop-reason") {
    handleMutationResult(await settingsStore.updateStopReasonOption(optionId, label), "Opcao atualizada.", "Nao foi possivel atualizar a opcao.");
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

  if (group === "pause-reason") {
    handleMutationResult(await settingsStore.removePauseReasonOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
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

  if (group === "cancel-reason") {
    handleMutationResult(await settingsStore.removeCancelReasonOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
    return;
  }

  if (group === "stop-reason") {
    handleMutationResult(await settingsStore.removeStopReasonOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
    return;
  }

  handleMutationResult(await settingsStore.removeProfessionOption(optionId), "Opcao removida.", "Nao foi possivel remover a opcao.");
}

async function reorderOption(group, optionIds) {
  if (group === "visit-reason") {
    handleMutationResult(await settingsStore.reorderVisitReasonOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
    return;
  }

  if (group === "customer-source") {
    handleMutationResult(await settingsStore.reorderCustomerSourceOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
    return;
  }

  if (group === "pause-reason") {
    handleMutationResult(await settingsStore.reorderPauseReasonOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
    return;
  }

  if (group === "queue-jump-reason") {
    handleMutationResult(await settingsStore.reorderQueueJumpReasonOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
    return;
  }

  if (group === "loss-reason") {
    handleMutationResult(await settingsStore.reorderLossReasonOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
    return;
  }

  if (group === "cancel-reason") {
    handleMutationResult(await settingsStore.reorderCancelReasonOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
    return;
  }

  if (group === "stop-reason") {
    handleMutationResult(await settingsStore.reorderStopReasonOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
    return;
  }

  handleMutationResult(await settingsStore.reorderProfessionOptions(optionIds), "", "Nao foi possivel atualizar a ordem.");
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
      <p class="admin-panel__subtitle settings-tenant-scope-banner">
        <span class="material-icons-round" aria-hidden="true">domain</span>
        Estas configuracoes valem para todas as lojas do tenant. Alteracoes feitas aqui afetam toda a operacao.
      </p>
      <p v-if="runtimeSettingsNotice" class="admin-panel__subtitle settings-runtime-warning">
        <span class="material-icons-round" aria-hidden="true">warning</span>
        {{ runtimeSettingsNotice }}
      </p>
    </header>

    <SettingsTabs :tabs="visibleTabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

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
          <label class="settings-field"><span>Atendimentos em aberto por consultor</span><input :value="Number(state.settings.maxConcurrentServicesPerConsultant || 1)" type="number" min="1" :max="maxParallelPerConsultantLimit" :disabled="!canEditSettings" @change="updateNumericSetting('maxConcurrentServicesPerConsultant', $event.target.value)"></label>
          <p class="settings-card__text">Quantos atendimentos cada consultor pode manter em aberto antes de encerrar os anteriores. Limite atual: 1 a {{ maxParallelPerConsultantLimit }}.</p>
          <label class="settings-field"><span>Fechamento rapido (min)</span><input :value="Number(state.settings.timingFastCloseMinutes || 5)" type="number" min="1" max="120" :disabled="!canEditSettings" @change="updateNumericSetting('timingFastCloseMinutes', $event.target.value)"></label>
          <label class="settings-field"><span>Atendimento demorado (min)</span><input :value="Number(state.settings.timingLongServiceMinutes || 25)" type="number" min="1" max="240" :disabled="!canEditSettings" @change="updateNumericSetting('timingLongServiceMinutes', $event.target.value)"></label>
          <label class="settings-field"><span>Venda baixa (R$)</span><input :value="Number(state.settings.timingLowSaleAmount || 1200)" type="number" min="1" step="1" :disabled="!canEditSettings" @change="updateNumericSetting('timingLowSaleAmount', $event.target.value)"></label>
          <label class="settings-field"><span>Janela de cancelamento (seg)</span><input :value="Number(state.settings.serviceCancelWindowSeconds || 30)" type="number" min="0" max="300" :disabled="!canEditSettings" @change="updateNumericSetting('serviceCancelWindowSeconds', $event.target.value)"></label>
          <p class="settings-card__text">Dentro dessa janela, o botão principal troca para cancelar atendimento e desfaz o início sem encerrar o fluxo completo.</p>
          <label class="settings-toggle"><input :checked="Boolean(state.settings.testModeEnabled)" type="checkbox" :disabled="!canEditSettings" @change="updateBooleanSetting('testModeEnabled', $event.target.checked)"><span>Modo teste</span></label>
          <label class="settings-toggle"><input :checked="Boolean(state.settings.autoFillFinishModal)" type="checkbox" :disabled="!canEditSettings" @change="updateBooleanSetting('autoFillFinishModal', $event.target.checked)"><span>Preencher modal automaticamente</span></label>
        </article>
      </div>
    </div>

    <div v-if="activeTab === 'cancelamento'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Campo de cancelamento</h3>
          <p class="settings-card__text">Define como a justificativa aparece quando o atendimento ainda esta dentro da janela de cancelamento.</p>
        </header>
        <AppSelectField
          class="settings-field"
          label="Modo do campo"
          :model-value="state.modalConfig.cancelReasonInputMode || 'text'"
          :options="reasonInputModeOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('cancelReasonInputMode', $event)"
        />
        <label class="settings-field"><span>Label</span><input :value="state.modalConfig.cancelReasonLabel || 'Motivo do cancelamento'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('cancelReasonLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder</span><input :value="state.modalConfig.cancelReasonPlaceholder || 'Informe ou selecione o motivo do cancelamento'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('cancelReasonPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label do outro</span><input :value="state.modalConfig.cancelReasonOtherLabel || 'Detalhe do cancelamento'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('cancelReasonOtherLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder do outro</span><input :value="state.modalConfig.cancelReasonOtherPlaceholder || 'Explique por que o atendimento foi cancelado'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('cancelReasonOtherPlaceholder', $event.target.value)"></label>
      </article>

      <SettingsOptionManager
        title="Motivos de cancelamento"
        description="Opcoes exibidas quando o campo estiver configurado como lista ou lista com outro."
        :items="state.cancelReasonOptions || []"
        :disabled="!canEditSettings"
        add-placeholder="Adicionar novo motivo de cancelamento"
        testid="settings-cancel-reasons"
        @add="addOption('cancel-reason', $event)"
        @update="(optionId, label) => updateOption('cancel-reason', optionId, label)"
        @remove="removeOption('cancel-reason', $event)"
        @reorder="reorderOption('cancel-reason', $event)"
      />
    </div>

    <div v-if="activeTab === 'parada'" class="settings-grid">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Campo de parada</h3>
          <p class="settings-card__text">A parada sempre exige justificativa. Aqui voce escolhe apenas como ela sera coletada e exibida.</p>
        </header>
        <AppSelectField
          class="settings-field"
          label="Modo do campo"
          :model-value="state.modalConfig.stopReasonInputMode || 'text'"
          :options="reasonInputModeOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('stopReasonInputMode', $event)"
        />
        <label class="settings-field"><span>Label</span><input :value="state.modalConfig.stopReasonLabel || 'Motivo da parada'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('stopReasonLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder</span><input :value="state.modalConfig.stopReasonPlaceholder || 'Informe ou selecione o motivo da parada'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('stopReasonPlaceholder', $event.target.value)"></label>
        <label class="settings-field"><span>Label do outro</span><input :value="state.modalConfig.stopReasonOtherLabel || 'Detalhe da parada'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('stopReasonOtherLabel', $event.target.value)"></label>
        <label class="settings-field"><span>Placeholder do outro</span><input :value="state.modalConfig.stopReasonOtherPlaceholder || 'Explique por que o atendimento foi parado'" type="text" :disabled="!canEditSettings" @change="updateModalConfigValue('stopReasonOtherPlaceholder', $event.target.value)"></label>
      </article>

      <SettingsOptionManager
        title="Motivos de parada"
        description="Opcoes exibidas quando a parada estiver configurada como lista ou lista com outro."
        :items="state.stopReasonOptions || []"
        :disabled="!canEditSettings"
        add-placeholder="Adicionar novo motivo de parada"
        testid="settings-stop-reasons"
        @add="addOption('stop-reason', $event)"
        @update="(optionId, label) => updateOption('stop-reason', optionId, label)"
        @remove="removeOption('stop-reason', $event)"
        @reorder="reorderOption('stop-reason', $event)"
      />
    </div>

    <div v-if="activeTab === 'modal'" class="settings-grid settings-grid--modal">
      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Fluxo de fechamento</h3>
          <p class="settings-card__text">Escolha entre o modal atual e o fluxo novo para conciliacao ERP, sem perder compatibilidade com o formulario legado.</p>
        </header>

        <AppSelectField
          class="settings-field"
          label="Modo do modal"
          :model-value="getFinishFlowMode()"
          :options="modalFinishFlowOptions"
          :disabled="!canEditSettings"
          @update:model-value="updateModalConfigValue('finishFlowMode', $event)"
        />

        <label class="settings-field">
          <span>Placeholder do codigo da compra</span>
          <input
            :value="getModalTextValue('purchaseCodePlaceholder', 'Informe o codigo da compra para conciliacao posterior')"
            type="text"
            :disabled="!canEditSettings || getFinishFlowMode() !== 'erp-reconciliation'"
            @change="updateModalConfigValue('purchaseCodePlaceholder', $event.target.value)"
          >
        </label>
      </article>

      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Campos e validacoes</h3>
          <p class="settings-card__text">Cada bloco agora concentra os campos do modal com switches de exibicao, obrigatoriedade e justificativa.</p>
        </header>

        <div class="settings-modal-section-list">
          <details
            v-for="section in modalFieldSections"
            :key="section.id"
            class="settings-collapse"
            :open="section.defaultOpen"
          >
            <summary class="settings-collapse__summary">
              <div class="settings-collapse__title-wrap">
                <strong class="settings-collapse__title">{{ section.title }}</strong>
                <span class="settings-collapse__text">{{ section.description }}</span>
              </div>
              <span class="settings-collapse__meta">{{ getModalFieldSectionSummary(section) }}</span>
              <span class="material-icons-round settings-collapse__icon">expand_more</span>
            </summary>

            <div class="settings-collapse__body settings-modal-field-list">
              <article
                v-for="field in section.fields"
                :key="field.id"
                class="settings-modal-field-row"
              >
                <div class="settings-modal-field-row__copy">
                  <input
                    v-if="field.labelKey"
                    class="settings-modal-field-row__title-input"
                    :value="getModalTextValue(field.labelKey, field.label)"
                    type="text"
                    :disabled="!canEditSettings"
                    @change="handleModalFieldLabelChange(field, $event.target.value)"
                  >
                  <strong v-else class="settings-modal-field-row__title">{{ field.label }}</strong>
                  <span class="settings-modal-field-row__hint">{{ field.description }}</span>
                </div>

                <div class="settings-modal-field-row__switches">
                  <div class="settings-modal-field-row__switch">
                    <span class="settings-modal-field-row__switch-label">Mostrar</span>
                    <AppToggleSwitch
                      :model-value="isModalFieldVisible(field)"
                      :disabled="!canEditSettings"
                      compact
                      @change="handleModalFieldVisibilityChange(field, $event)"
                    />
                  </div>

                  <div class="settings-modal-field-row__switch">
                    <span class="settings-modal-field-row__switch-label">Obrigatorio</span>
                    <AppToggleSwitch
                      :model-value="isModalFieldRequired(field)"
                      :disabled="!canEditSettings || !field.requiredKey || !isModalFieldVisible(field)"
                      compact
                      @change="handleModalFieldRequiredChange(field, $event)"
                    />
                  </div>

                  <div class="settings-modal-field-row__switch">
                    <span class="settings-modal-field-row__switch-label">Justificativa</span>
                    <AppToggleSwitch
                      :model-value="isModalFieldJustificationRequired(field)"
                      :disabled="!canEditSettings || !field.justificationRequiredKey || !isModalFieldVisible(field)"
                      compact
                      @change="handleModalFieldJustificationChange(field, $event)"
                    />
                  </div>

                  <label
                    v-if="field.justificationMinCharsKey && isModalFieldJustificationRequired(field)"
                    class="settings-modal-field-row__switch settings-modal-field-row__switch--number"
                  >
                    <span class="settings-modal-field-row__switch-label">Min. sem espacos</span>
                    <input
                      class="settings-modal-field-row__number-input"
                      :value="getModalFieldJustificationMinChars(field)"
                      type="number"
                      min="1"
                      max="500"
                      :disabled="!canEditSettings || !isModalFieldVisible(field) || !isModalFieldJustificationRequired(field)"
                      @change="updateModalConfigNumberValue(field.justificationMinCharsKey, $event.target.value, 1)"
                    >
                  </label>
                </div>
              </article>
            </div>
          </details>
        </div>
      </article>

      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Regras de interesses</h3>
          <p class="settings-card__text">Aqui ficam as regras do campo de interesses do cliente e da justificativa quando nao houver item selecionado.</p>
        </header>

        <div class="settings-modal-rules">
          <div class="settings-modal-rule">
            <div class="settings-modal-rule__copy">
              <strong class="settings-modal-rule__title">Permitir opcao "nenhum"</strong>
              <span class="settings-modal-rule__hint">Libera no modal a escolha de nenhum interesse identificado para aquele atendimento.</span>
            </div>

            <AppToggleSwitch
              :model-value="getModalBooleanValue('allowProductSeenNone', true)"
              :disabled="!canEditSettings || !getModalBooleanValue('showProductSeenField', true)"
              @change="updateModalConfigValue('allowProductSeenNone', $event)"
            />
          </div>

          <div class="settings-modal-rule">
            <div class="settings-modal-rule__copy">
              <strong class="settings-modal-rule__title">Exigir justificativa ao marcar nenhum</strong>
              <span class="settings-modal-rule__hint">Quando o consultor escolher nenhum interesse, obriga o preenchimento do texto complementar.</span>
            </div>

            <AppToggleSwitch
              :model-value="getModalBooleanValue('requireProductSeenNotesWhenNone', true)"
              :disabled="!canEditSettings || !getModalBooleanValue('showProductSeenNotesField', true) || !getModalBooleanValue('allowProductSeenNone', true)"
              @change="updateModalConfigValue('requireProductSeenNotesWhenNone', $event)"
            />
          </div>

          <label class="settings-field">
            <span>Título dos detalhes</span>
            <input
              :value="getModalTextValue('productSeenNotesLabel', 'Observação dos interesses')"
              type="text"
              :disabled="!canEditSettings || !getModalBooleanValue('showProductSeenNotesField', true)"
              @change="updateModalConfigValue('productSeenNotesLabel', $event.target.value)"
            >
          </label>

          <label class="settings-field">
            <span>Placeholder dos detalhes</span>
            <input
              :value="getModalTextValue('productSeenNotesPlaceholder', 'Descreva referência, pedido específico, contexto do cliente ou justificativa quando não houver interesse identificado.')"
              type="text"
              :disabled="!canEditSettings || !getModalBooleanValue('showProductSeenNotesField', true)"
              @change="updateModalConfigValue('productSeenNotesPlaceholder', $event.target.value)"
            >
          </label>

          <label class="settings-field">
            <span>Mínimo de caracteres da justificativa</span>
            <input
              :value="getModalNumberValue('productSeenNotesMinChars', 20, 1)"
              type="number"
              min="1"
              max="500"
              :disabled="!canEditSettings || !getModalBooleanValue('showProductSeenNotesField', true)"
              @change="updateModalConfigNumberValue('productSeenNotesMinChars', $event.target.value, 1)"
            >
          </label>
        </div>
      </article>

      <article class="settings-card">
        <header class="settings-card__header">
          <h3 class="settings-card__title">Textos do modal</h3>
          <p class="settings-card__text">Os textos ficam organizados em blocos separados, depois da matriz de switches.</p>
        </header>

        <div class="settings-modal-section-list">
          <details
            v-for="section in modalTextSections"
            :key="section.id"
            class="settings-collapse"
            :open="section.defaultOpen"
          >
            <summary class="settings-collapse__summary">
              <div class="settings-collapse__title-wrap">
                <strong class="settings-collapse__title">{{ section.title }}</strong>
                <span class="settings-collapse__text">{{ section.description }}</span>
              </div>
              <span class="settings-collapse__meta">{{ getModalTextSectionSummary(section) }}</span>
              <span class="material-icons-round settings-collapse__icon">expand_more</span>
            </summary>

            <div class="settings-collapse__body settings-modal-text-grid">
              <label
                v-for="field in section.fields"
                :key="field.key"
                class="settings-field settings-modal-text-field"
              >
                <span>{{ field.label }}</span>
                <input
                  :value="modalConfigState[field.key] || ''"
                  type="text"
                  :disabled="!canEditSettings"
                  @change="updateModalConfigValue(field.key, $event.target.value)"
                >
              </label>
            </div>
          </details>
        </div>
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
        :disabled="!canEditSettings"
        add-placeholder="Adicionar nova opcao"
        testid="settings-motivos"
        @add="addOption('visit-reason', $event)"
        @update="(optionId, label) => updateOption('visit-reason', optionId, label)"
        @remove="removeOption('visit-reason', $event)"
        @reorder="reorderOption('visit-reason', $event)"
      />
    </div>

    <div v-if="activeTab === 'pausas'">
      <SettingsOptionManager
        title="Motivos de pausa"
        description="Opcoes exibidas ao pausar consultor no painel de operacao."
        :items="state.pauseReasonOptions || []"
        :disabled="!canEditSettings"
        add-placeholder="Adicionar novo motivo de pausa"
        testid="settings-pausas"
        @add="addOption('pause-reason', $event)"
        @update="(optionId, label) => updateOption('pause-reason', optionId, label)"
        @remove="removeOption('pause-reason', $event)"
        @reorder="reorderOption('pause-reason', $event)"
      />
    </div>

    <div v-if="activeTab === 'motivos-fora-da-vez'">
      <SettingsOptionManager
        title="Motivo fora da vez"
        description="Opcoes obrigatorias exibidas quando o atendimento for encerrado fora da vez."
        :items="state.queueJumpReasonOptions || []"
        :disabled="!canEditSettings"
        add-placeholder="Adicionar novo motivo fora da vez"
        testid="settings-fora-da-vez"
        @add="addOption('queue-jump-reason', $event)"
        @update="(optionId, label) => updateOption('queue-jump-reason', optionId, label)"
        @remove="removeOption('queue-jump-reason', $event)"
        @reorder="reorderOption('queue-jump-reason', $event)"
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
        :disabled="!canEditSettings"
        add-placeholder="Adicionar novo motivo da perda"
        testid="settings-motivos-perda"
        @add="addOption('loss-reason', $event)"
        @update="(optionId, label) => updateOption('loss-reason', optionId, label)"
        @remove="removeOption('loss-reason', $event)"
        @reorder="reorderOption('loss-reason', $event)"
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
        :disabled="!canEditSettings"
        add-placeholder="Adicionar nova opcao"
        testid="settings-origens"
        @add="addOption('customer-source', $event)"
        @update="(optionId, label) => updateOption('customer-source', optionId, label)"
        @remove="removeOption('customer-source', $event)"
        @reorder="reorderOption('customer-source', $event)"
      />
    </div>

    <div v-if="activeTab === 'profissoes'">
      <SettingsOptionManager
        title="Profissoes"
        description="Lista usada no modal. Se nao existir, tambem pode cadastrar na hora no fechamento."
        :items="state.professionOptions || []"
        :disabled="!canEditSettings"
        add-placeholder="Adicionar nova profissao"
        testid="settings-profissoes"
        @add="addOption('profession', $event)"
        @update="(optionId, label) => updateOption('profession', optionId, label)"
        @remove="removeOption('profession', $event)"
        @reorder="reorderOption('profession', $event)"
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

<style scoped>
.settings-runtime-warning {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  margin-top: 0.85rem;
  padding: 0.8rem 0.95rem;
  border-radius: 14px;
  border: 1px solid rgba(245, 158, 11, 0.28);
  background: rgba(245, 158, 11, 0.08);
  color: #fbbf24;
}

.settings-runtime-warning .material-icons-round {
  font-size: 1rem;
  margin-top: 0.1rem;
}
</style>
