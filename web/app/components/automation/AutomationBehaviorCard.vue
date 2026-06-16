<script setup lang="ts">
interface Props {
  personaName: string
  personaPrompt: string
  personaLoading: boolean
  savingPersona: boolean
  personaSavedAt: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:personaName': [value: string]
  'update:personaPrompt': [value: string]
  save: []
}>()

const personaSaved = computed(() => props.personaSavedAt > 0)

// Tom de voz ainda nao tem campo no backend — visual por enquanto.
const tone = ref('professional-friendly')
const TONES = [
  { value: 'professional-friendly', label: 'Profissional e amigavel' },
  { value: 'professional', label: 'Profissional' },
  { value: 'casual', label: 'Casual e descontraido' },
  { value: 'direct', label: 'Direto e objetivo' },
]
</script>

<template>
  <article class="abh">
    <header class="abh__head">
      <div class="abh__head-text">
        <h2 class="abh__title">Comportamento</h2>
        <p class="abh__subtitle">Nome e instrucoes do assistente</p>
      </div>
      <div class="abh__head-actions">
        <span v-if="personaSaved" class="abh__saved">Salvo</span>
        <button
          type="button"
          class="abh__save"
          :disabled="savingPersona || personaLoading"
          @click="emit('save')"
        >
          {{ savingPersona ? 'Salvando...' : 'Salvar comportamento' }}
        </button>
      </div>
    </header>

    <hr class="abh__divider" />

    <div class="abh__fields">
      <label class="abh__field">
        <span class="abh__label">Nome do assistente</span>
        <input
          :value="personaName"
          type="text"
          class="abh__input"
          :disabled="personaLoading"
          @input="emit('update:personaName', ($event.target as HTMLInputElement).value)"
        />
      </label>

      <label class="abh__field">
        <span class="abh__label">
          Tom de voz
          <span class="abh__tag">visual</span>
        </span>
        <select v-model="tone" class="abh__input abh__select">
          <option v-for="t in TONES" :key="t.value" :value="t.value">{{ t.label }}</option>
        </select>
      </label>
    </div>

    <label class="abh__field abh__field--full">
      <span class="abh__label">Instrucoes (comportamento)</span>
      <textarea
        :value="personaPrompt"
        class="abh__textarea"
        rows="14"
        spellcheck="false"
        :disabled="personaLoading"
        @input="emit('update:personaPrompt', ($event.target as HTMLTextAreaElement).value)"
      ></textarea>
    </label>

    <p class="abh__hint">
      Comportamento, tom e personalidade. Conhecimento (catalogo, precos, FAQs) vai nos documentos.
      Guardrails de WhatsApp sao aplicados automaticamente no runtime. (Tom de voz ainda e visual —
      sem backend.)
    </p>
  </article>
</template>

<style scoped>
.abh {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.abh__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.abh__title {
  font-size: 1.5rem;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.abh__subtitle {
  font-size: 0.9rem;
  color: var(--text-muted);
  margin-top: 0.2rem;
}

.abh__head-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-shrink: 0;
}

.abh__saved {
  font-size: 0.82rem;
  font-weight: 600;
  color: rgb(var(--success));
}

.abh__save {
  font-size: 0.9rem;
  font-weight: 600;
  padding: 0.6rem 1.3rem;
  border-radius: 0.55rem;
  cursor: pointer;
  border: 1px solid transparent;
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.abh__save:disabled {
  opacity: 0.6;
  cursor: progress;
}

.abh__divider {
  border: none;
  border-top: 1px solid var(--line-soft);
  margin: 0;
}

.abh__fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1.25rem;
}

.abh__field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
}

.abh__field--full {
  width: 100%;
}

.abh__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.abh__tag {
  font-size: 0.6rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 0.05rem 0.35rem;
  border-radius: 0.3rem;
  background: rgb(var(--surface-2) / 0.9);
  border: 1px solid var(--line-soft);
  color: var(--text-muted);
  text-transform: none;
}

.abh__input,
.abh__textarea {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.6rem;
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font: inherit;
}

.abh__input:focus,
.abh__textarea:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.abh__select {
  cursor: pointer;
  appearance: none;
  background-image:
    linear-gradient(45deg, transparent 50%, var(--text-muted) 50%),
    linear-gradient(135deg, var(--text-muted) 50%, transparent 50%);
  background-position:
    calc(100% - 18px) 50%,
    calc(100% - 13px) 50%;
  background-size:
    5px 5px,
    5px 5px;
  background-repeat: no-repeat;
}

.abh__textarea {
  resize: vertical;
  min-height: 230px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.88rem;
  line-height: 1.6;
  white-space: pre-wrap;
}

.abh__hint {
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.5;
}

@media (max-width: 720px) {
  .abh__fields {
    grid-template-columns: 1fr;
  }
}
</style>
