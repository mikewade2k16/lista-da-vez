<script setup>
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'

defineProps({
  ctx: {
    type: Object,
    required: true,
  },
})
</script>

<template>
  <div class="settings-grid settings-grid--modal">
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Fluxo de fechamento</h3>
        <p class="settings-card__text">
          Escolha entre o modal atual e o fluxo novo para conciliacao ERP, sem perder
          compatibilidade com o formulario legado.
        </p>
      </header>

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
    </article>

    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Campos e validacoes</h3>
        <p class="settings-card__text">
          Cada bloco agora concentra os campos do modal com switches de exibicao, obrigatoriedade e
          justificativa.
        </p>
      </header>

      <div class="settings-modal-section-list">
        <details
          v-for="section in ctx.modalFieldSections"
          :key="section.id"
          class="settings-collapse"
          :open="section.defaultOpen"
        >
          <summary class="settings-collapse__summary">
            <div class="settings-collapse__title-wrap">
              <strong class="settings-collapse__title">{{ section.title }}</strong>
              <span class="settings-collapse__text">{{ section.description }}</span>
            </div>
            <span class="settings-collapse__meta">
              {{ ctx.getModalFieldSectionSummary(section) }}
            </span>
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
                      !ctx.canEditSettings || !field.requiredKey || !ctx.isModalFieldVisible(field)
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
    </article>

    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Regras de interesses</h3>
        <p class="settings-card__text">
          Aqui ficam as regras do campo de interesses do cliente e da justificativa quando nao
          houver item selecionado.
        </p>
      </header>

      <div class="settings-modal-rules">
        <div class="settings-modal-rule">
          <div class="settings-modal-rule__copy">
            <strong class="settings-modal-rule__title">Permitir opcao "nenhum"</strong>
            <span class="settings-modal-rule__hint">
              Libera no modal a escolha de nenhum interesse identificado para aquele atendimento.
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
              Quando o consultor escolher nenhum interesse, obriga o preenchimento do texto
              complementar.
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
            @change="ctx.updateModalConfigValue('productSeenNotesPlaceholder', $event.target.value)"
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
    </article>

    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Textos do modal</h3>
        <p class="settings-card__text">
          Os textos ficam organizados em blocos separados, depois da matriz de switches.
        </p>
      </header>

      <div class="settings-modal-section-list">
        <details
          v-for="section in ctx.modalTextSections"
          :key="section.id"
          class="settings-collapse"
          :open="section.defaultOpen"
        >
          <summary class="settings-collapse__summary">
            <div class="settings-collapse__title-wrap">
              <strong class="settings-collapse__title">{{ section.title }}</strong>
              <span class="settings-collapse__text">{{ section.description }}</span>
            </div>
            <span class="settings-collapse__meta">
              {{ ctx.getModalTextSectionSummary(section) }}
            </span>
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
                :value="ctx.modalConfigState[field.key] || ''"
                type="text"
                :disabled="!ctx.canEditSettings"
                @change="ctx.updateModalConfigValue(field.key, $event.target.value)"
              />
            </label>
          </div>
        </details>
      </div>
    </article>
  </div>
</template>
