import { computed, reactive, ref } from 'vue'

import {
  fieldDetailModeOptions,
  fieldSelectionOptions,
  hiddenSettingsTabs,
  modalFieldSections,
  modalFinishFlowOptions,
  modalTextSections,
  optionTabConfigs,
  reasonInputModeOptions,
  reasonInputSectionConfigs,
  settingsTabs,
} from '~/domain/utils/settings-workspace-data'
import {
  DEFAULT_CRM_GOAL_PAYOUT_POLICY,
  DEFAULT_CRM_LIST_USAGE_TIERS,
  normalizeCrmGoalPayoutPolicy,
  normalizeCrmListUsageMinOrders,
  normalizeCrmListUsageTiers,
} from '~/domain/utils/crm-performance-policy'
import {
  canManageCrmCommercialPolicy,
  canManageConsultants,
  canManageSettings,
} from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useConsultantsStore } from '~/stores/consultants'
import { useSettingsStore } from '~/stores/settings'
import { useUiStore } from '~/stores/ui'

export function useSettingsWorkspace(props) {
  const settingsStore = useSettingsStore()
  const consultantsStore = useConsultantsStore()
  const ui = useUiStore()
  const auth = useAuthStore()
  const scoreWeightSettingIds = new Set([
    'scoreWeightConversion',
    'scoreWeightSoldValue',
    'scoreWeightQuality',
    'scoreWeightPa',
    'scoreWeightQueueDiscipline',
  ])

  const activeTab = ref('operacao')
  const state = computed(() => props.state || {})
  const modalConfigState = computed(() => state.value.modalConfig || {})
  const visibleTabs = computed(() => settingsTabs.filter((tab) => !hiddenSettingsTabs.has(tab.id)))
  const runtimeSettingsNotice = computed(() => String(auth.runtimeSettingsNotice || '').trim())
  const canEditSettings = computed(() =>
    canManageSettings(auth.role, auth.permissionKeys, auth.permissionsResolved),
  )
  const canEditCrmCommercialPolicy = computed(() =>
    canManageCrmCommercialPolicy(auth.role, auth.permissionKeys, auth.permissionsResolved),
  )
  const canEditConsultants = computed(() =>
    canManageConsultants(auth.role, auth.permissionKeys, auth.permissionsResolved),
  )
  const crmListUsageTiers = computed(() =>
    normalizeCrmListUsageTiers(state.value.settings?.crmListUsageTiers),
  )
  const crmListUsageMinOrdersForHighlight = computed(() =>
    normalizeCrmListUsageMinOrders(state.value.settings?.crmListUsageMinOrdersForHighlight),
  )
  const crmGoalPayoutPolicy = computed(() =>
    normalizeCrmGoalPayoutPolicy(state.value.settings?.crmGoalPayoutPolicy),
  )
  const maxParallelPerConsultantLimit = computed(() =>
    Math.min(5, Math.max(1, Number(state.value.settings?.maxConcurrentServices || 1) || 1)),
  )

  async function updateNumericSetting(settingId, value) {
    const numericValue = Number(value)
    if (settingId === 'maxConcurrentServicesPerConsultant') {
      const isInvalid =
        !Number.isFinite(numericValue) ||
        numericValue < 1 ||
        numericValue > maxParallelPerConsultantLimit.value
      if (isInvalid) {
        ui.error(
          `Atendimentos em aberto por consultor deve ficar entre 1 e ${maxParallelPerConsultantLimit.value}.`,
        )
        return
      }
    }

    const normalizedValue = Number.isFinite(numericValue) ? numericValue : 0
    const sanitizedValue = scoreWeightSettingIds.has(settingId)
      ? Math.max(0, normalizedValue)
      : normalizedValue

    const result = await settingsStore.updateSetting(settingId, sanitizedValue)
    if (result?.ok === false) ui.error(result.message || 'Nao foi possivel salvar a configuracao.')
  }

  async function updateBooleanSetting(settingId, value) {
    const result = await settingsStore.updateSetting(settingId, Boolean(value))
    if (result?.ok === false) ui.error(result.message || 'Nao foi possivel salvar a configuracao.')
  }

  async function updateCrmCommercialPolicy(patch) {
    const nextPayload = {
      crmListUsageTiers: crmListUsageTiers.value,
      crmListUsageMinOrdersForHighlight: crmListUsageMinOrdersForHighlight.value,
      crmGoalPayoutPolicy: crmGoalPayoutPolicy.value,
      ...patch,
    }
    const result = await settingsStore.updateCrmCommercialPolicy(nextPayload)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel salvar a politica comercial do CRM.')
    }
    return result || { ok: true }
  }

  async function updateCrmListUsageTier(index, field, value) {
    const tiers = crmListUsageTiers.value.map((tier) => ({ ...tier }))
    if (!tiers[index]) return
    tiers[index][field] = field === 'minRate' ? Number(value) : String(value || '').trim()
    await updateCrmCommercialPolicy({ crmListUsageTiers: normalizeCrmListUsageTiers(tiers) })
  }

  async function addCrmListUsageTier() {
    const tiers = [
      ...crmListUsageTiers.value,
      { id: `faixa-${Date.now()}`, label: 'Nova faixa', minRate: 0 },
    ]
    await updateCrmCommercialPolicy({ crmListUsageTiers: normalizeCrmListUsageTiers(tiers) })
  }

  async function removeCrmListUsageTier(index) {
    const tiers = crmListUsageTiers.value.filter((_, itemIndex) => itemIndex !== index)
    await updateCrmCommercialPolicy({
      crmListUsageTiers: normalizeCrmListUsageTiers(
        tiers.length ? tiers : DEFAULT_CRM_LIST_USAGE_TIERS,
      ),
    })
  }

  async function updateCrmListUsageMinOrders(value) {
    await updateCrmCommercialPolicy({
      crmListUsageMinOrdersForHighlight: normalizeCrmListUsageMinOrders(value),
    })
  }

  async function updateCrmGoalPayoutRule(group, index, field, value) {
    const policy = normalizeCrmGoalPayoutPolicy(crmGoalPayoutPolicy.value)
    const rules = Array.isArray(policy[group]) ? policy[group].map((rule) => ({ ...rule })) : []
    if (!rules[index]) return
    rules[index][field] = field === 'mode' ? String(value || 'percent') : Number(value)
    await updateCrmCommercialPolicy({
      crmGoalPayoutPolicy: normalizeCrmGoalPayoutPolicy({
        ...policy,
        [group]: rules,
      }),
    })
  }

  async function addCrmGoalPayoutRule(group) {
    const policy = normalizeCrmGoalPayoutPolicy(crmGoalPayoutPolicy.value)
    const fallback = DEFAULT_CRM_GOAL_PAYOUT_POLICY[group]?.[0] || {
      threshold: 80,
      value: 1,
      mode: 'percent',
    }
    await updateCrmCommercialPolicy({
      crmGoalPayoutPolicy: normalizeCrmGoalPayoutPolicy({
        ...policy,
        [group]: [...(policy[group] || []), { ...fallback }],
      }),
    })
  }

  async function removeCrmGoalPayoutRule(group, index) {
    const policy = normalizeCrmGoalPayoutPolicy(crmGoalPayoutPolicy.value)
    const rules = (policy[group] || []).filter((_, itemIndex) => itemIndex !== index)
    await updateCrmCommercialPolicy({
      crmGoalPayoutPolicy: normalizeCrmGoalPayoutPolicy({
        ...policy,
        [group]: rules.length ? rules : DEFAULT_CRM_GOAL_PAYOUT_POLICY[group],
      }),
    })
  }

  async function updateModalConfigValue(configKey, value) {
    const result = await settingsStore.updateModalConfig(configKey, value)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel salvar a configuracao do modal.')
    }
    return result || { ok: true }
  }

  async function updateModalConfigNumberValue(configKey, value, minimum = 0) {
    const normalizedValue = Math.max(minimum, Math.trunc(Number(value) || 0))
    return updateModalConfigValue(configKey, normalizedValue)
  }

  function getModalNumberValue(configKey, fallback = 0, minimum = 0) {
    const normalizedFallback = Math.max(minimum, Math.trunc(Number(fallback) || 0))
    if (!configKey) return normalizedFallback
    const numericValue = Math.trunc(Number(modalConfigState.value?.[configKey]))
    if (!Number.isFinite(numericValue) || numericValue < minimum) return normalizedFallback
    return numericValue
  }

  function getModalTextValue(configKey, fallback = '') {
    if (!configKey) return fallback
    const configuredValue = String(modalConfigState.value?.[configKey] || '').trim()
    return configuredValue || fallback
  }

  function getModalBooleanValue(configKey, fallback = false, legacyConfigKey = '') {
    const directValue = modalConfigState.value?.[configKey]
    if (typeof directValue === 'boolean') return directValue
    const legacyValue = legacyConfigKey ? modalConfigState.value?.[legacyConfigKey] : undefined
    return typeof legacyValue === 'boolean' ? legacyValue : fallback
  }

  function getFinishFlowMode() {
    const configuredValue = String(modalConfigState.value?.finishFlowMode || '').trim()
    return configuredValue === 'erp-reconciliation' ? 'erp-reconciliation' : 'legacy'
  }

  function isModalFieldVisible(field) {
    return getModalBooleanValue(field.showKey, field.showDefault ?? true, field.legacyShowKey || '')
  }

  function isModalFieldRequired(field) {
    if (!field.requiredKey) return false
    return getModalBooleanValue(
      field.requiredKey,
      field.requiredDefault ?? false,
      field.legacyRequiredKey || '',
    )
  }

  async function handleModalFieldVisibilityChange(field, nextValue) {
    await updateModalConfigValue(field.showKey, nextValue)
    if (!nextValue && field.requiredKey) await updateModalConfigValue(field.requiredKey, false)
    if (!nextValue && field.justificationRequiredKey) {
      await updateModalConfigValue(field.justificationRequiredKey, false)
    }
  }

  async function handleModalFieldRequiredChange(field, nextValue) {
    if (!field.requiredKey || !isModalFieldVisible(field)) return
    await updateModalConfigValue(field.requiredKey, nextValue)
  }

  function isModalFieldJustificationRequired(field) {
    if (!field.justificationRequiredKey) return false
    return getModalBooleanValue(field.justificationRequiredKey, false)
  }

  function getModalFieldJustificationMinChars(field) {
    return getModalNumberValue(field.justificationMinCharsKey, 20, 1)
  }

  async function handleModalFieldJustificationChange(field, nextValue) {
    if (!field.justificationRequiredKey || !isModalFieldVisible(field)) return
    await updateModalConfigValue(field.justificationRequiredKey, nextValue)
  }

  async function handleModalFieldLabelChange(field, value) {
    if (!field.labelKey) return
    const normalizedValue = String(value || '').trim()
    await updateModalConfigValue(field.labelKey, normalizedValue || field.label)
  }

  function getModalFieldSectionSummary(section) {
    const visibleCount = section.fields.filter((field) => isModalFieldVisible(field)).length
    const requiredCount = section.fields.filter(
      (field) => isModalFieldVisible(field) && isModalFieldRequired(field),
    ).length
    return `${visibleCount}/${section.fields.length} visiveis - ${requiredCount} obrigatorios`
  }

  function getModalTextSectionSummary(section) {
    const filledCount = section.fields.filter((field) =>
      String(modalConfigState.value?.[field.key] || '').trim(),
    ).length
    return `${filledCount}/${section.fields.length} preenchidos`
  }

  async function applyTemplate(templateId) {
    const result = await settingsStore.applyOperationTemplate(templateId)
    if (result?.ok === false) ui.error(result.message || 'Nao foi possivel aplicar o template.')
  }

  function handleMutationResult(result, successMessage, fallbackErrorMessage) {
    if (result?.ok === false) {
      ui.error(result.message || fallbackErrorMessage)
      return false
    }
    if (successMessage) ui.success(successMessage)
    return true
  }

  const optionActions = {
    'cancel-reason': {
      add: (label) => settingsStore.addCancelReasonOption(label),
      remove: (id) => settingsStore.removeCancelReasonOption(id),
      reorder: (ids) => settingsStore.reorderCancelReasonOptions(ids),
      update: (id, label) => settingsStore.updateCancelReasonOption(id, label),
    },
    'customer-source': {
      add: (label) => settingsStore.addCustomerSourceOption(label),
      remove: (id) => settingsStore.removeCustomerSourceOption(id),
      reorder: (ids) => settingsStore.reorderCustomerSourceOptions(ids),
      update: (id, label) => settingsStore.updateCustomerSourceOption(id, label),
    },
    'loss-reason': {
      add: (label) => settingsStore.addLossReasonOption(label),
      remove: (id) => settingsStore.removeLossReasonOption(id),
      reorder: (ids) => settingsStore.reorderLossReasonOptions(ids),
      update: (id, label) => settingsStore.updateLossReasonOption(id, label),
    },
    'pause-reason': {
      add: (label) => settingsStore.addPauseReasonOption(label),
      remove: (id) => settingsStore.removePauseReasonOption(id),
      reorder: (ids) => settingsStore.reorderPauseReasonOptions(ids),
      update: (id, label) => settingsStore.updatePauseReasonOption(id, label),
    },
    profession: {
      add: (label) => settingsStore.addProfessionOption(label),
      remove: (id) => settingsStore.removeProfessionOption(id),
      reorder: (ids) => settingsStore.reorderProfessionOptions(ids),
      update: (id, label) => settingsStore.updateProfessionOption(id, label),
    },
    'queue-jump-reason': {
      add: (label) => settingsStore.addQueueJumpReasonOption(label),
      remove: (id) => settingsStore.removeQueueJumpReasonOption(id),
      reorder: (ids) => settingsStore.reorderQueueJumpReasonOptions(ids),
      update: (id, label) => settingsStore.updateQueueJumpReasonOption(id, label),
    },
    'stop-reason': {
      add: (label) => settingsStore.addStopReasonOption(label),
      remove: (id) => settingsStore.removeStopReasonOption(id),
      reorder: (ids) => settingsStore.reorderStopReasonOptions(ids),
      update: (id, label) => settingsStore.updateStopReasonOption(id, label),
    },
    'visit-reason': {
      add: (label) => settingsStore.addVisitReasonOption(label),
      remove: (id) => settingsStore.removeVisitReasonOption(id),
      reorder: (ids) => settingsStore.reorderVisitReasonOptions(ids),
      update: (id, label) => settingsStore.updateVisitReasonOption(id, label),
    },
  }

  function getOptionActions(group) {
    return optionActions[group] || optionActions.profession
  }

  async function addOption(group, label) {
    handleMutationResult(
      await getOptionActions(group).add(label),
      'Opcao adicionada.',
      'Nao foi possivel adicionar a opcao.',
    )
  }

  async function updateOption(group, optionId, label) {
    handleMutationResult(
      await getOptionActions(group).update(optionId, label),
      'Opcao atualizada.',
      'Nao foi possivel atualizar a opcao.',
    )
  }

  async function removeOption(group, optionId) {
    handleMutationResult(
      await getOptionActions(group).remove(optionId),
      'Opcao removida.',
      'Nao foi possivel remover a opcao.',
    )
  }

  async function reorderOption(group, optionIds) {
    handleMutationResult(
      await getOptionActions(group).reorder(optionIds),
      '',
      'Nao foi possivel atualizar a ordem.',
    )
  }

  async function addProduct(payload) {
    handleMutationResult(
      await settingsStore.addCatalogProduct(
        payload.name,
        payload.category,
        payload.basePrice,
        payload.code,
      ),
      'Produto adicionado.',
      'Nao foi possivel adicionar o produto.',
    )
  }

  async function updateProduct(productId, payload) {
    handleMutationResult(
      await settingsStore.updateCatalogProduct(productId, payload),
      'Produto atualizado.',
      'Nao foi possivel atualizar o produto.',
    )
  }

  async function removeProduct(productId) {
    handleMutationResult(
      await settingsStore.removeCatalogProduct(productId),
      'Produto removido.',
      'Nao foi possivel remover o produto.',
    )
  }

  async function addConsultant(payload) {
    const result = await consultantsStore.createConsultantProfile(payload)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel criar consultor.')
      return
    }

    const accessEmail = String(result?.access?.email || '').trim()
    const initialPassword = String(result?.access?.initialPassword || '').trim()
    if (accessEmail && initialPassword) {
      await ui.prompt({
        title: 'Acesso do consultor criado',
        message: `Login padrao: ${accessEmail}\nSenha inicial: ${initialPassword}\nOriente o consultor a trocar a senha em Perfil no primeiro acesso.`,
        inputLabel: 'Acesso',
        initialValue: `${accessEmail} | ${initialPassword}`,
        confirmLabel: 'Fechar',
      })
    }

    ui.success('Consultor criado com acesso vinculado.')
  }

  async function updateConsultant(consultantId, payload) {
    const result = await consultantsStore.updateConsultantProfile(consultantId, payload)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel atualizar consultor.')
      return
    }
    ui.success('Consultor atualizado.')
  }

  async function archiveConsultant(consultantId) {
    const { confirmed } = await ui.confirm({
      title: 'Arquivar consultor',
      message: 'O consultor sera removido da escala ativa. Deseja continuar?',
      confirmLabel: 'Arquivar',
    })
    if (!confirmed) return

    const result = await consultantsStore.archiveConsultantProfile(consultantId)
    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel arquivar consultor.')
      return
    }
    ui.success('Consultor arquivado.')
  }

  return reactive({
    activeTab,
    addCrmGoalPayoutRule,
    addCrmListUsageTier,
    addConsultant,
    addOption,
    addProduct,
    applyTemplate,
    archiveConsultant,
    canEditConsultants,
    canEditCrmCommercialPolicy,
    canEditSettings,
    crmGoalPayoutPolicy,
    crmListUsageMinOrdersForHighlight,
    crmListUsageTiers,
    fieldDetailModeOptions,
    fieldSelectionOptions,
    getFinishFlowMode,
    getModalBooleanValue,
    getModalFieldJustificationMinChars,
    getModalFieldSectionSummary,
    getModalNumberValue,
    getModalTextSectionSummary,
    getModalTextValue,
    handleModalFieldJustificationChange,
    handleModalFieldLabelChange,
    handleModalFieldRequiredChange,
    handleModalFieldVisibilityChange,
    isModalFieldJustificationRequired,
    isModalFieldRequired,
    isModalFieldVisible,
    maxParallelPerConsultantLimit,
    modalConfigState,
    modalFieldSections,
    modalFinishFlowOptions,
    modalTextSections,
    optionTabConfigs,
    reasonInputModeOptions,
    reasonInputSectionConfigs,
    removeCrmGoalPayoutRule,
    removeCrmListUsageTier,
    removeOption,
    removeProduct,
    reorderOption,
    runtimeSettingsNotice,
    state,
    updateBooleanSetting,
    updateConsultant,
    updateCrmCommercialPolicy,
    updateCrmGoalPayoutRule,
    updateCrmListUsageMinOrders,
    updateCrmListUsageTier,
    updateModalConfigNumberValue,
    updateModalConfigValue,
    updateNumericSetting,
    updateOption,
    updateProduct,
    visibleTabs,
  })
}
