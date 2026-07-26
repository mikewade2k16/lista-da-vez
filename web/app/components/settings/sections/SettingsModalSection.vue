<script setup>
import { computed, ref } from 'vue'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'

const props = defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})

const panels = [
  {
    id: 'flow',
    label: 'Fluxo',
    icon: 'account_tree',
    title: 'Fluxo de fechamento',
    description: 'Modal atual ou conciliacao ERP.',
  },
  {
    id: 'fields',
    label: 'Campos e validacoes',
    icon: 'fact_check',
    title: 'Campos e validacoes',
    description: 'Exibicao, obrigatoriedade e justificativa por campo.',
  },
  {
    id: 'interests',
    label: 'Interesses',
    icon: 'interests',
    title: 'Regras de interesses',
    description: 'Campo de interesses do cliente e justificativa.',
  },
  {
    id: 'texts',
    label: 'Textos',
    icon: 'text_fields',
    title: 'Textos do modal',
    description: 'Titulos, labels e placeholders organizados por bloco.',
  },
]

const activePanelId = ref(panels[0].id)
const activePanel = computed(
  () => panels.find((panel) => panel.id === activePanelId.value) || panels[0],
)

const fieldSectionsMeta = computed(() => {
  const sections = props.ctx.modalFieldSections || []
  const fields = sections.flatMap((section) => section.fields || [])
  const visible = fields.filter((field) => props.ctx.isModalFieldVisible(field)).length
  return `${visible}/${fields.length} campos visiveis`
})

const textSectionsMeta = computed(() => {
  const sections = props.ctx.modalTextSections || []
  const fields = sections.flatMap((section) => section.fields || [])
  const filled = fields.filter((field) =>
    String(props.ctx.modalConfigState?.[field.key] || '').trim(),
  ).length
  return `${filled}/${fields.length} textos preenchidos`
})

function panelMeta(panelId) {
  if (panelId === 'flow') {
    const mode = props.ctx.getFinishFlowMode()
    return (
      (props.ctx.modalFinishFlowOptions || []).find((option) => option.value === mode)?.label ||
      'Modal atual'
    )
  }
  if (panelId === 'fields') return fieldSectionsMeta.value
  if (panelId === 'interests') {
    return props.ctx.getModalBooleanValue('allowProductSeenNone', true)
      ? 'Opcao nenhum liberada'
      : 'Opcao nenhum bloqueada'
  }
  return textSectionsMeta.value
}
</script>

<template>
  <div class="settings-modal-workspace">
    <aside
      class="settings-card settings-modal-sidebar"
      aria-label="Topicos de configuracao do modal"
    >
      <span class="settings-modal-sidebar__title">Configurar modal</span>
      <nav class="settings-modal-sidebar__nav">
        <button
          v-for="panel in panels"
          :key="panel.id"
          class="settings-modal-sidebar__item"
          :class="{ 'is-active': activePanelId === panel.id }"
          type="button"
          :aria-current="activePanelId === panel.id ? 'page' : undefined"
          @click="activePanelId = panel.id"
        >
          <span class="material-icons-round" aria-hidden="true">{{ panel.icon }}</span>
          <span>{{ panel.label }}</span>
        </button>
      </nav>
    </aside>

    <section class="settings-card settings-modal-panel">
      <header class="settings-card__header settings-modal-panel__header">
        <div>
          <h3 class="settings-card__title">{{ activePanel.title }}</h3>
          <p class="settings-card__text">{{ activePanel.description }}</p>
        </div>
        <span class="settings-collapse__meta settings-modal-panel__meta">
          {{ panelMeta(activePanel.id) }}
        </span>
      </header>

      <div class="settings-modal-panel__body">
        <div v-if="activePanelId === 'flow'" class="settings-modal-flow-grid">
          <AppSelectField
            class="settings-field"
            label="Modo do modal"
            :model-value="ctx.getFinishFlowMode()"
            :options="ctx.modalFinishFlowOptions"
            :disabled="!ctx.canEditSettings"
            @update:model-value="ctx.updateModalConfigValue('finishFlowMode', $event)"
          />

          <label class="settings-field">
            <span>Placeholder do codigo da compra</span>
            <input
              :value="
                ctx.getModalTextValue(
                  'purchaseCodePlaceholder',
                  'Informe o codigo da compra para conciliacao posterior',
                )
              "
              type="text"
              :disabled="!ctx.canEditSettings || ctx.getFinishFlowMode() !== 'erp-reconciliation'"
              @change="ctx.updateModalConfigValue('purchaseCodePlaceholder', $event.target.value)"
            />
          </label>
        </div>

        <div v-else-if="activePanelId === 'fields'" class="settings-modal-accordion-list">
          <details
            v-for="section in ctx.modalFieldSections"
            :key="section.id"
            class="settings-collapse settings-modal-accordion"
          >
            <summary class="settings-collapse__summary settings-modal-accordion__summary">
              <span class="settings-collapse__title-wrap">
                <strong class="settings-collapse__title">{{ section.title }}</strong>
                <small class="settings-collapse__text">{{ section.description }}</small>
              </span>
              <span class="settings-collapse__meta settings-modal-accordion__meta">
                {{ ctx.getModalFieldSectionSummary(section) }}
              </span>
              <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
                expand_more
              </span>
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
                    :value="ctx.getModalTextValue(field.labelKey, field.label)"
                    type="text"
                    :disabled="!ctx.canEditSettings"
                    @change="ctx.handleModalFieldLabelChange(field, $event.target.value)"
                  />
                  <strong v-else class="settings-modal-field-row__title">{{ field.label }}</strong>
                  <span class="settings-modal-field-row__hint">{{ field.description }}</span>
                </div>

                <div class="settings-modal-field-row__switches">
                  <div class="settings-modal-field-row__switch">
                    <span class="settings-modal-field-row__switch-label">Mostrar</span>
                    <AppToggleSwitch
                      :model-value="ctx.isModalFieldVisible(field)"
                      :disabled="!ctx.canEditSettings"
                      compact
                      @change="ctx.handleModalFieldVisibilityChange(field, $event)"
                    />
                  </div>

                  <div class="settings-modal-field-row__switch">
                    <span class="settings-modal-field-row__switch-label">Obrigatorio</span>
                    <AppToggleSwitch
                      :model-value="ctx.isModalFieldRequired(field)"
                      :disabled="
                        !ctx.canEditSettings ||
                        !field.requiredKey ||
                        !ctx.isModalFieldVisible(field)
                      "
                      compact
                      @change="ctx.handleModalFieldRequiredChange(field, $event)"
                    />
                  </div>

                  <div class="settings-modal-field-row__switch">
                    <span class="settings-modal-field-row__switch-label">Justificativa</span>
                    <AppToggleSwitch
                      :model-value="ctx.isModalFieldJustificationRequired(field)"
                      :disabled="
                        !ctx.canEditSettings ||
                        !field.justificationRequiredKey ||
                        !ctx.isModalFieldVisible(field)
                      "
                      compact
                      @change="ctx.handleModalFieldJustificationChange(field, $event)"
                    />
                  </div>

                  <label
                    v-if="
                      field.justificationMinCharsKey && ctx.isModalFieldJustificationRequired(field)
                    "
                    class="settings-modal-field-row__switch settings-modal-field-row__switch--number"
                  >
                    <span class="settings-modal-field-row__switch-label">Min. sem espacos</span>
                    <input
                      class="settings-modal-field-row__number-input"
                      :value="ctx.getModalFieldJustificationMinChars(field)"
                      type="number"
                      min="1"
                      max="500"
                      :disabled="
                        !ctx.canEditSettings ||
                        !ctx.isModalFieldVisible(field) ||
                        !ctx.isModalFieldJustificationRequired(field)
                      "
                      @change="
                        ctx.updateModalConfigNumberValue(
                          field.justificationMinCharsKey,
                          $event.target.value,
                          1,
                        )
                      "
                    />
                  </label>
                </div>
              </article>
            </div>
          </details>
        </div>

        <div v-else-if="activePanelId === 'interests'" class="settings-modal-interest-grid">
          <div class="settings-modal-rule">
            <div class="settings-modal-rule__copy">
              <strong class="settings-modal-rule__title">Permitir opcao "nenhum"</strong>
              <span class="settings-modal-rule__hint">
                Libera a escolha de nenhum interesse identificado.
              </span>
            </div>
            <AppToggleSwitch
              :model-value="ctx.getModalBooleanValue('allowProductSeenNone', true)"
              :disabled="
                !ctx.canEditSettings || !ctx.getModalBooleanValue('showProductSeenField', true)
              "
              @change="ctx.updateModalConfigValue('allowProductSeenNone', $event)"
            />
          </div>

          <div class="settings-modal-rule">
            <div class="settings-modal-rule__copy">
              <strong class="settings-modal-rule__title">
                Exigir justificativa ao marcar nenhum
              </strong>
              <span class="settings-modal-rule__hint">
                Obriga o preenchimento do texto complementar.
              </span>
            </div>
            <AppToggleSwitch
              :model-value="ctx.getModalBooleanValue('requireProductSeenNotesWhenNone', true)"
              :disabled="
                !ctx.canEditSettings ||
                !ctx.getModalBooleanValue('showProductSeenNotesField', true) ||
                !ctx.getModalBooleanValue('allowProductSeenNone', true)
              "
              @change="ctx.updateModalConfigValue('requireProductSeenNotesWhenNone', $event)"
            />
          </div>

          <label class="settings-field">
            <span>Titulo dos detalhes</span>
            <input
              :value="ctx.getModalTextValue('productSeenNotesLabel', 'Observacao dos interesses')"
              type="text"
              :disabled="
                !ctx.canEditSettings || !ctx.getModalBooleanValue('showProductSeenNotesField', true)
              "
              @change="ctx.updateModalConfigValue('productSeenNotesLabel', $event.target.value)"
            />
          </label>

          <label class="settings-field">
            <span>Placeholder dos detalhes</span>
            <input
              :value="
                ctx.getModalTextValue(
                  'productSeenNotesPlaceholder',
                  'Descreva referencia, pedido especifico, contexto do cliente ou justificativa quando nao houver interesse identificado.',
                )
              "
              type="text"
              :disabled="
                !ctx.canEditSettings || !ctx.getModalBooleanValue('showProductSeenNotesField', true)
              "
              @change="
                ctx.updateModalConfigValue('productSeenNotesPlaceholder', $event.target.value)
              "
            />
          </label>

          <label class="settings-field">
            <span>Minimo de caracteres da justificativa</span>
            <input
              :value="ctx.getModalNumberValue('productSeenNotesMinChars', 20, 1)"
              type="number"
              min="1"
              max="500"
              :disabled="
                !ctx.canEditSettings || !ctx.getModalBooleanValue('showProductSeenNotesField', true)
              "
              @change="
                ctx.updateModalConfigNumberValue('productSeenNotesMinChars', $event.target.value, 1)
              "
            />
          </label>
        </div>

        <div v-else class="settings-modal-accordion-list">
          <details
            v-for="section in ctx.modalTextSections"
            :key="section.id"
            class="settings-collapse settings-modal-accordion"
          >
            <summary class="settings-collapse__summary settings-modal-accordion__summary">
              <span class="settings-collapse__title-wrap">
                <strong class="settings-collapse__title">{{ section.title }}</strong>
                <small class="settings-collapse__text">{{ section.description }}</small>
              </span>
              <span class="settings-collapse__meta settings-modal-accordion__meta">
                {{ ctx.getModalTextSectionSummary(section) }}
              </span>
              <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
                expand_more
              </span>
            </summary>

            <div class="settings-collapse__body settings-modal-text-grid">
              <label
                v-for="field in section.fields"
                :key="field.key"
                class="settings-field settings-modal-text-field"
              >
                <span>{{ field.label }}</span>
                <input
                  :value="ctx.modalConfigState[field.key] || ''"
                  type="text"
                  :disabled="!ctx.canEditSettings"
                  @change="ctx.updateModalConfigValue(field.key, $event.target.value)"
                />
              </label>
            </div>
          </details>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped src="./settings-modal-section.css"></style>
