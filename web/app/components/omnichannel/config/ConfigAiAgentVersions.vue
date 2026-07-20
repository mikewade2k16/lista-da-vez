<script setup lang="ts">
import { computed, reactive } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useUiStore } from '~/stores/ui'
import type { OmniAgentVersion, OmniAgentVersionInput } from '~/domain/omnichannel/config-types'

// Versões do agente: lista + criar rascunho + publish/rollback. Versão publicada é
// IMUTÁVEL (editar = criar nova versão; rollback = repontar a ativa). O componente é
// presentacional: as chamadas ao back e o reload ficam no card pai (fonte única).
const props = defineProps<{
  versions: OmniAgentVersion[]
  activeVersionId: string | null
  disabled?: boolean
}>()
const emit = defineEmits<{
  create: [payload: OmniAgentVersionInput]
  publish: [version: number]
  rollback: [versionId: string]
}>()

const ui = useUiStore()

const form = reactive({ provider: '', model: '', temperature: 0.2, layers: '[]' })

const createBlockedReason = computed(() => {
  if (!form.provider.trim()) return 'Informe o provider do modelo.'
  if (!form.model.trim()) return 'Informe o modelo.'
  return ''
})

function isActive(v: OmniAgentVersion): boolean {
  return props.activeVersionId === v.id
}

function submitDraft(): void {
  if (createBlockedReason.value) return
  let layers: unknown
  try {
    layers = JSON.parse(form.layers || '[]')
  } catch {
    ui.error('As camadas do prompt precisam ser um JSON válido.')
    return
  }
  emit('create', {
    provider: form.provider.trim(),
    model: form.model.trim(),
    temperature: Number(form.temperature) || 0,
    layers,
  })
  form.layers = '[]'
}
</script>

<template>
  <div class="cfg-vers">
    <span class="cfg-field__label">Versões</span>
    <p v-if="versions.length === 0" class="cfg-vers__hint">
      Nenhuma versão ainda. Crie um rascunho abaixo e publique quando estiver pronto.
    </p>
    <ul v-else class="cfg-vers__list">
      <li v-for="v in versions" :key="v.id" class="cfg-vers__item">
        <div class="cfg-vers__meta">
          <strong>v{{ v.version }}</strong>
          <span class="cfg-vers__tag" :class="`is-${v.status}`">{{ v.status }}</span>
          <span v-if="isActive(v)" class="cfg-vers__tag is-active">ativa</span>
          <span class="cfg-vers__prov">{{ v.provider }}/{{ v.model }}</span>
        </div>
        <div class="cfg-vers__actions">
          <AppPanelButton
            v-if="v.status !== 'published'"
            variant="secondary"
            :disabled="disabled"
            @click="emit('publish', v.version)"
          >
            Publicar
          </AppPanelButton>
          <AppPanelButton
            v-else-if="!isActive(v)"
            variant="ghost"
            :disabled="disabled"
            @click="emit('rollback', v.id)"
          >
            Reverter para esta
          </AppPanelButton>
        </div>
      </li>
    </ul>

    <details class="cfg-vers__new">
      <summary>Nova versão (rascunho)</summary>
      <div class="cfg-vers__form">
        <div class="cfg-grid">
          <label class="cfg-field">
            <span class="cfg-field__label">Provider do modelo *</span>
            <input
              v-model="form.provider"
              class="cfg-input"
              type="text"
              placeholder="gemini / openai / glm"
              :disabled="disabled"
            />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Modelo *</span>
            <input v-model="form.model" class="cfg-input" type="text" :disabled="disabled" />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Temperatura</span>
            <input
              v-model.number="form.temperature"
              class="cfg-input"
              type="number"
              step="0.1"
              min="0"
              max="2"
              :disabled="disabled"
            />
          </label>
        </div>
        <label class="cfg-field">
          <span class="cfg-field__label">Camadas do prompt (JSON)</span>
          <textarea
            v-model="form.layers"
            class="cfg-input cfg-textarea"
            rows="4"
            :disabled="disabled"
          ></textarea>
        </label>
        <div class="cfg-vers__foot">
          <span v-if="createBlockedReason" class="cfg-vers__hint">{{ createBlockedReason }}</span>
          <AppPanelButton
            variant="primary"
            :disabled="disabled || !!createBlockedReason"
            @click="submitDraft"
          >
            Criar rascunho
          </AppPanelButton>
        </div>
      </div>
    </details>
  </div>
</template>

<style scoped>
.cfg-vers {
  display: grid;
  gap: 0.5rem;
}

.cfg-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.cfg-vers__hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.cfg-vers__list {
  display: grid;
  gap: 0.4rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.cfg-vers__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.45rem 0.6rem;
  border: 1px solid rgb(var(--border) / 0.7);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.5);
}

.cfg-vers__meta {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  font-size: 0.82rem;
  color: rgb(var(--text));
}

.cfg-vers__tag {
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 700;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
}

.cfg-vers__tag.is-published {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cfg-vers__tag.is-active {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.cfg-vers__prov {
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.6rem;
}

.cfg-field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.cfg-input {
  min-height: 36px;
  padding: 0 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.cfg-textarea {
  padding: 0.5rem 0.75rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  line-height: 1.4;
  resize: vertical;
}

.cfg-input:focus {
  outline: none;
  border-color: rgb(var(--primary) / 0.6);
}

.cfg-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.cfg-vers__new {
  margin-top: 0.4rem;
}

.cfg-vers__new summary {
  cursor: pointer;
  font-size: 0.8rem;
  color: rgb(var(--primary));
}

.cfg-vers__form {
  display: grid;
  gap: 0.6rem;
  margin-top: 0.5rem;
}

.cfg-vers__foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
}
</style>
