<script setup lang="ts">
import { ref, computed, watch } from "vue"
import { isValidAlertHexColor, normalizeAlertHexColor } from "~/utils/alert-colors"

const props = defineProps<{
  modelValue: boolean
  rule?: Record<string, any>
  isEditing?: boolean
}>()

const emit = defineEmits<{
  "update:modelValue": [value: boolean]
  "save": [rule: Record<string, any>]
}>()

function defaultForm() {
  return {
    name: "",
    description: "",
    isActive: true,
    triggerType: "long_open_service",
    thresholdMinutes: 25,
    severity: "critical",
    displayKind: "banner",
    colorTheme: "#F59E0B",
    titleTemplate: "Atendimento em aberto ha {elapsed}",
    bodyTemplate: "O atendimento de {consultant} segue aberto acima do limite de {threshold} min.",
    interactionKind: "none",
    responseOptions: [] as Array<{ value: string; label: string }>,
    isMandatory: false,
    notifyDashboard: true,
    notifyOperationContext: true,
    notifyExternal: false,
    externalChannel: "none"
  }
}

const form = ref(defaultForm())

const triggerTypes = [
  { value: "long_open_service", label: "Atendimento longo" },
  { value: "long_queue_wait", label: "Fila longa" },
  { value: "long_pause", label: "Pausa longa" },
  { value: "idle_store", label: "Loja parada" },
  { value: "outside_business_hours", label: "Fora do horário" }
]

const disabledTriggerTypes = new Set([
  "long_queue_wait",
  "long_pause",
  "idle_store",
  "outside_business_hours"
])
const enabledTriggerValues = new Set(triggerTypes.map((trigger) => trigger.value).filter((value) => !disabledTriggerTypes.has(value)))

function isTriggerDisabled(value: string) {
  return disabledTriggerTypes.has(String(value || "").trim())
}

function triggerOptionLabel(trigger: { value: string; label: string }) {
  return isTriggerDisabled(trigger.value) ? `${trigger.label} - em desenvolvimento` : trigger.label
}

const displayKinds = [
  { value: "card_badge", label: "Badge no card" },
  { value: "banner", label: "Banner" },
  { value: "toast", label: "Notificação" },
  { value: "corner_popup", label: "Popup canto" },
  { value: "center_modal", label: "Modal central" },
  { value: "fullscreen", label: "Tela cheia" }
]

const colorThemes = [
  { value: "amber", label: "Âmbar" },
  { value: "red", label: "Vermelho" },
  { value: "blue", label: "Azul" },
  { value: "green", label: "Verde" },
  { value: "purple", label: "Roxo" },
  { value: "slate", label: "Cinza" }
]

const colorPresetLabels: Record<string, string> = {
  amber: "Ambar",
  red: "Vermelho",
  blue: "Azul",
  green: "Verde",
  purple: "Roxo",
  slate: "Cinza"
}

const colorPresets = colorThemes.map((theme) => ({
  value: normalizeAlertHexColor(theme.value).toUpperCase(),
  label: colorPresetLabels[theme.value] || theme.label
}))

const interactionKinds = [
  { value: "none", label: "Nenhuma" },
  { value: "dismiss", label: "Descartar" },
  { value: "confirm_choice", label: "Confirmação" },
  { value: "select_option", label: "Seleção" }
]

const needsOptions = computed(() =>
  ["confirm_choice", "select_option"].includes(form.value.interactionKind)
)

const colorPreview = computed(() => normalizeAlertHexColor(form.value.colorTheme))
const colorIsValid = computed(() => isValidAlertHexColor(form.value.colorTheme))
const colorPickerValue = computed({
  get: () => colorPreview.value,
  set: (value: string) => {
    form.value.colorTheme = normalizeAlertHexColor(value).toUpperCase()
  }
})

const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value)
})

function normalizeText(value: unknown, fallback = "") {
  const normalized = String(value ?? "").trim()
  return normalized || fallback
}

function normalizeBoolean(value: unknown, fallback = false) {
  return typeof value === "boolean" ? value : fallback
}

function normalizeMinutes(value: unknown, fallback: number) {
  return Math.max(1, Number(value ?? fallback) || fallback)
}

function normalizeTriggerType(value: unknown, fallback = defaultForm().triggerType) {
  const normalized = normalizeText(value, fallback)
  return enabledTriggerValues.has(normalized) ? normalized : fallback
}

function handleColorTextInput(event: Event) {
  const target = event.target as HTMLInputElement
  form.value.colorTheme = String(target?.value || "").trim().toUpperCase()
}

function commitColorTextInput() {
  form.value.colorTheme = normalizeAlertHexColor(form.value.colorTheme).toUpperCase()
}

function cloneResponseOptions(value: unknown) {
  if (!Array.isArray(value)) {
    return []
  }

  return value.map((option) => ({
    value: normalizeText(option?.value),
    label: normalizeText(option?.label)
  }))
}

function initForm() {
  const defaults = defaultForm()

  if (!props.rule || !props.isEditing) {
    form.value = defaults
    return
  }

  form.value = {
    name: normalizeText(props.rule.name, defaults.name),
    description: normalizeText(props.rule.description, defaults.description),
    isActive: normalizeBoolean(props.rule.isActive, defaults.isActive),
    triggerType: normalizeTriggerType(props.rule.triggerType, defaults.triggerType),
    thresholdMinutes: normalizeMinutes(props.rule.thresholdMinutes, defaults.thresholdMinutes),
    severity: normalizeText(props.rule.severity, defaults.severity),
    displayKind: normalizeText(props.rule.displayKind, defaults.displayKind),
    colorTheme: normalizeAlertHexColor(props.rule.colorTheme, defaults.colorTheme).toUpperCase(),
    titleTemplate: normalizeText(props.rule.titleTemplate, defaults.titleTemplate),
    bodyTemplate: normalizeText(props.rule.bodyTemplate, defaults.bodyTemplate),
    interactionKind: normalizeText(props.rule.interactionKind, defaults.interactionKind),
    responseOptions: cloneResponseOptions(props.rule.responseOptions),
    isMandatory: normalizeBoolean(props.rule.isMandatory, defaults.isMandatory),
    notifyDashboard: normalizeBoolean(props.rule.notifyDashboard, defaults.notifyDashboard),
    notifyOperationContext: normalizeBoolean(props.rule.notifyOperationContext, defaults.notifyOperationContext),
    notifyExternal: normalizeBoolean(props.rule.notifyExternal, defaults.notifyExternal),
    externalChannel: normalizeText(props.rule.externalChannel, defaults.externalChannel)
  }
}

function addOption() {
  form.value.responseOptions.push({ value: "", label: "" })
}

function removeOption(index: number) {
  form.value.responseOptions.splice(index, 1)
}

function save() {
  const payload = {
    ...form.value,
    triggerType: normalizeTriggerType(form.value.triggerType),
    thresholdMinutes: normalizeMinutes(form.value.thresholdMinutes, defaultForm().thresholdMinutes),
    colorTheme: normalizeAlertHexColor(form.value.colorTheme).toUpperCase(),
    responseOptions: cloneResponseOptions(form.value.responseOptions)
  }

  if (!["confirm_choice", "select_option"].includes(payload.interactionKind)) {
    payload.isMandatory = false
    payload.responseOptions = []
  }

  emit("save", payload)
}

function handleOpenChange(open: boolean) {
  isOpen.value = open
  if (open) {
    initForm()
  }
}

watch(
  () => [props.modelValue, props.rule?.id],
  ([open]) => {
    if (open) {
      initForm()
    }
  },
  { immediate: true }
)

watch(
  () => props.rule,
  () => {
    if (props.modelValue) {
      initForm()
    }
  },
  { deep: true }
)
</script>

<template>
  <div v-if="isOpen" class="alert-rule-editor-overlay" @click.self="handleOpenChange(false)">
    <div class="alert-rule-editor-modal">
      <div class="editor-header">
        <h2>{{ isEditing ? "Editar regra" : "Nova regra" }}</h2>
        <button class="close-btn" @click="handleOpenChange(false)">✕</button>
      </div>

      <div class="editor-content">
        <!-- Seção 1: Identificação -->
        <section class="editor-section">
          <h3>Identificação</h3>
          <div class="form-group">
            <label>Nome da regra *</label>
            <input v-model="form.name" type="text" placeholder="ex: Atendimento longo" />
          </div>
          <div class="form-group">
            <label>Descrição</label>
            <input v-model="form.description" type="text" placeholder="ex: Alerta para atendimentos que excedem o limite" />
          </div>
          <div class="form-group checkbox">
            <input v-model="form.isActive" type="checkbox" id="is-active" />
            <label for="is-active">Ativa</label>
          </div>
        </section>

        <!-- Seção 2: Trigger -->
        <section class="editor-section">
          <h3>Gatilho</h3>
          <div class="form-group">
            <label>Tipo de gatilho *</label>
            <select v-model="form.triggerType">
              <option v-for="t in triggerTypes" :key="t.value" :value="t.value" :disabled="isTriggerDisabled(t.value)">
                {{ triggerOptionLabel(t) }}
              </option>
            </select>
          </div>
          <div class="form-group">
            <label>Limite (minutos) *</label>
            <input v-model.number="form.thresholdMinutes" type="number" min="1" />
          </div>
          <div class="form-group">
            <label>Severidade</label>
            <select v-model="form.severity">
              <option value="info">Informação</option>
              <option value="attention">Atenção</option>
              <option value="critical">Crítica</option>
            </select>
          </div>
        </section>

        <!-- Seção 3: Apresentação -->
        <section class="editor-section">
          <h3>Apresentação</h3>
          <div class="form-group">
            <label>Tipo de display *</label>
            <select v-model="form.displayKind">
              <option v-for="d in displayKinds" :key="d.value" :value="d.value">
                {{ d.label }}
              </option>
            </select>
          </div>
          <div class="form-group">
            <label>Cor do tema</label>
            <div class="color-picker">
              <div class="color-picker__controls">
                <label class="color-picker__swatch" :style="{ '--selected-color': colorPreview }" aria-label="Escolher cor hexadecimal">
                  <input v-model="colorPickerValue" type="color" />
                </label>
                <input
                  :value="form.colorTheme"
                  type="text"
                  maxlength="7"
                  placeholder="#F59E0B"
                  spellcheck="false"
                  :aria-invalid="!colorIsValid"
                  @input="handleColorTextInput"
                  @blur="commitColorTextInput"
                />
              </div>
              <div class="color-picker__presets" aria-label="Cores sugeridas">
                <button
                  v-for="preset in colorPresets"
                  :key="preset.value"
                  class="color-picker__preset"
                  type="button"
                  :class="{ 'is-active': colorPreview.toUpperCase() === preset.value }"
                  :style="{ '--preset-color': preset.value }"
                  :title="preset.label"
                  @click="form.colorTheme = preset.value"
                >
                  <span class="color-picker__preset-swatch" aria-hidden="true"></span>
                  <span>{{ preset.label }}</span>
                </button>
              </div>
              <p v-if="!colorIsValid" class="color-picker__error">Informe uma cor hexadecimal valida. Ex.: #F59E0B</p>
            </div>
          </div>
          <div class="form-group">
            <label>Título do alerta *</label>
            <input v-model="form.titleTemplate" type="text" placeholder="{consultant} atendimento longo" />
          </div>
          <div class="form-group">
            <label>Corpo do alerta</label>
            <input v-model="form.bodyTemplate" type="text" placeholder="{consultant} há {elapsed}" />
          </div>
        </section>

        <!-- Seção 4: Interação -->
        <section class="editor-section">
          <h3>Interação</h3>
          <div class="form-group">
            <label>Tipo de interação *</label>
            <select v-model="form.interactionKind">
              <option v-for="i in interactionKinds" :key="i.value" :value="i.value">
                {{ i.label }}
              </option>
            </select>
          </div>

          <div v-if="needsOptions" class="form-group">
            <label>Opções de resposta (mín. 2)</label>
            <div class="options-list">
              <div v-for="(opt, idx) in form.responseOptions" :key="idx" class="option-row">
                <input v-model="opt.value" placeholder="valor" />
                <input v-model="opt.label" placeholder="rótulo" />
                <button @click="removeOption(idx)" class="btn-remove">✕</button>
              </div>
            </div>
            <button @click="addOption" class="btn-secondary">+ Adicionar opção</button>
          </div>

          <div class="form-group checkbox">
            <input v-model="form.isMandatory" type="checkbox" id="is-mandatory" :disabled="form.interactionKind === 'none'" />
            <label for="is-mandatory">Resposta obrigatória</label>
          </div>
        </section>

        <!-- Seção 5: Notificação -->
        <section class="editor-section">
          <h3>Notificações</h3>
          <div class="form-group checkbox">
            <input v-model="form.notifyDashboard" type="checkbox" id="notify-dashboard" />
            <label for="notify-dashboard">Notificar dashboard</label>
          </div>
          <div class="form-group checkbox">
            <input v-model="form.notifyOperationContext" type="checkbox" id="notify-context" />
            <label for="notify-context">Notificar contexto operacional</label>
          </div>
          <div class="form-group checkbox">
            <input v-model="form.notifyExternal" type="checkbox" id="notify-external" />
            <label for="notify-external">Notificar externo</label>
          </div>
          <div v-if="form.notifyExternal" class="form-group">
            <label>Canal externo</label>
            <select v-model="form.externalChannel">
              <option value="none">Nenhum</option>
              <option value="whatsapp">WhatsApp</option>
              <option value="email">E-mail</option>
            </select>
          </div>
        </section>
      </div>

      <div class="editor-footer">
        <button class="btn-secondary" @click="handleOpenChange(false)">Cancelar</button>
        <button class="btn-primary" @click="save">Salvar</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.alert-rule-editor-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.alert-rule-editor-modal {
  background: rgba(15, 23, 42, 0.98);
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  max-height: 90vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
}

.editor-header {
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.15);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(30, 41, 59, 0.6);
}

.editor-header h2 {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 600;
  color: #e2e8f0;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  color: rgba(148, 163, 184, 0.7);
  line-height: 1;
  padding: 0.25rem;
  transition: color 0.15s;
}

.close-btn:hover {
  color: #e2e8f0;
}

.editor-content {
  flex: 1;
  padding: 1.5rem;
  overflow-y: auto;
}

.editor-section {
  margin-bottom: 1.75rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.1);
}

.editor-section:last-child {
  border-bottom: none;
  margin-bottom: 0;
}

.editor-section h3 {
  margin: 0 0 1rem 0;
  font-size: 0.85rem;
  font-weight: 600;
  color: rgba(148, 163, 184, 0.7);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.form-group {
  margin-bottom: 0.875rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.35rem;
  font-weight: 500;
  color: #cbd5e1;
  font-size: 0.9rem;
}

.form-group input[type="text"],
.form-group input[type="number"],
.form-group select {
  width: 100%;
  padding: 0.55rem 0.75rem;
  background: rgba(30, 41, 59, 0.7);
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 4px;
  font-size: 0.9rem;
  color: #e2e8f0;
  outline: none;
  transition: border-color 0.15s;
  box-sizing: border-box;
}

.form-group input[type="text"]:focus,
.form-group input[type="number"]:focus,
.form-group select:focus {
  border-color: rgba(59, 130, 246, 0.6);
}

.form-group input::placeholder {
  color: rgba(148, 163, 184, 0.4);
}

.form-group select option {
  background: #1e293b;
  color: #e2e8f0;
}

.color-picker {
  display: grid;
  gap: 0.65rem;
}

.color-picker__controls {
  display: grid;
  grid-template-columns: 3rem minmax(0, 1fr);
  gap: 0.65rem;
  align-items: center;
}

.color-picker__swatch {
  position: relative;
  width: 3rem;
  height: 2.45rem;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 6px;
  background: var(--selected-color, #f59e0b);
  cursor: pointer;
  box-shadow: inset 0 0 0 2px rgba(15, 23, 42, 0.38);
}

.color-picker__swatch input {
  position: absolute;
  inset: -0.4rem;
  width: calc(100% + 0.8rem);
  height: calc(100% + 0.8rem);
  border: 0;
  padding: 0;
  opacity: 0;
  cursor: pointer;
}

.color-picker__presets {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.color-picker__preset {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.34rem 0.5rem;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 6px;
  background: rgba(15, 23, 42, 0.42);
  color: #cbd5e1;
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.color-picker__preset:hover,
.color-picker__preset.is-active {
  border-color: var(--preset-color, #f59e0b);
  color: #f8fafc;
}

.color-picker__preset-swatch {
  width: 0.78rem;
  height: 0.78rem;
  border-radius: 999px;
  background: var(--preset-color, #f59e0b);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.22);
}

.color-picker__error {
  margin: -0.15rem 0 0;
  color: #fca5a5;
  font-size: 0.78rem;
}

.form-group.checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.form-group.checkbox label {
  margin-bottom: 0;
  cursor: pointer;
}

.form-group.checkbox input[type="checkbox"] {
  width: auto;
  margin: 0;
  accent-color: #3b82f6;
  cursor: pointer;
}

.options-list {
  background: rgba(30, 41, 59, 0.5);
  border: 1px solid rgba(148, 163, 184, 0.15);
  border-radius: 4px;
  padding: 0.75rem;
  margin-bottom: 0.75rem;
}

.option-row {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.option-row:last-child {
  margin-bottom: 0;
}

.option-row input {
  flex: 1;
  padding: 0.45rem 0.6rem;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 4px;
  font-size: 0.875rem;
  color: #e2e8f0;
  outline: none;
}

.option-row input::placeholder {
  color: rgba(148, 163, 184, 0.35);
}

.btn-remove {
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #fca5a5;
  cursor: pointer;
  border-radius: 4px;
  width: 2rem;
  font-size: 0.85rem;
  transition: background 0.15s;
}

.btn-remove:hover {
  background: rgba(239, 68, 68, 0.25);
}

.editor-footer {
  padding: 1.25rem 1.5rem;
  border-top: 1px solid rgba(148, 163, 184, 0.15);
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  background: rgba(30, 41, 59, 0.4);
}

.btn-primary,
.btn-secondary {
  padding: 0.55rem 1.25rem;
  border-radius: 4px;
  font-weight: 500;
  font-size: 0.9rem;
  cursor: pointer;
  transition: opacity 0.15s;
  border: none;
}

.btn-primary {
  background: #3b82f6;
  color: white;
}

.btn-primary:hover {
  opacity: 0.9;
}

.btn-secondary {
  background: rgba(100, 116, 139, 0.3);
  border: 1px solid rgba(148, 163, 184, 0.25);
  color: #cbd5e1;
}

.btn-secondary:hover {
  background: rgba(100, 116, 139, 0.45);
}
</style>
