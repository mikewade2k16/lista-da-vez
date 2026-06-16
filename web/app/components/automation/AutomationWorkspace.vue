<script setup lang="ts">
import { useAutomation } from '~/composables/useAutomation'
import { useKnowledgeDocs } from '~/composables/useKnowledgeDocs'

const {
  qr,
  connecting,
  savingEnabled,
  errorMessage,
  connected,
  enabled,
  load,
  connect,
  disconnect,
  setEnabled,
  personaName,
  personaPrompt,
  personaLoading,
  savingPersona,
  personaSavedAt,
  loadPersona,
  savePersona,
} = useAutomation()

const { docs, loadDocs } = useKnowledgeDocs()

const ctxPreview = ref<{ refresh: () => void } | null>(null)

type SectionId = 'behavior' | 'sources' | 'models' | 'knowledge' | 'context'
interface Section {
  id: SectionId
  label: string
  icon: string
}

const SECTIONS: Section[] = [
  { id: 'behavior', label: 'Comportamento', icon: 'i-lucide-user' },
  { id: 'sources', label: 'Fontes de conhecimento', icon: 'i-lucide-link' },
  { id: 'models', label: 'Modelos de IA', icon: 'i-lucide-cpu' },
  { id: 'knowledge', label: 'Conhecimento', icon: 'i-lucide-book-open' },
  { id: 'context', label: 'Previa do contexto', icon: 'i-lucide-code' },
]

const active = ref<SectionId>('behavior')

function onToggleEnabled() {
  if (savingEnabled.value) return
  void setEnabled(!enabled.value)
}

function onKnowledgeChange() {
  ctxPreview.value?.refresh()
  void loadDocs()
}

onMounted(() => {
  void load()
  void loadPersona()
  void loadDocs()
})
</script>

<template>
  <section class="aw">
    <AutomationStatusBar
      :enabled="enabled"
      :saving-enabled="savingEnabled"
      :connected="connected"
      :connecting="connecting"
      :docs-count="docs.length"
      @toggle-enabled="onToggleEnabled"
      @connect="connect"
      @disconnect="disconnect"
    />

    <div v-if="qr" class="aw-qr">
      <img
        :src="qr"
        alt="QR code para conectar o WhatsApp"
        width="190"
        height="190"
        class="aw-qr__img"
      />
      <div class="aw-qr__text">
        <p class="aw-qr__title">Escaneie para conectar</p>
        <p class="aw-qr__hint">
          No celular: WhatsApp &gt; Aparelhos conectados &gt; Conectar aparelho.
        </p>
      </div>
    </div>

    <p v-if="errorMessage" class="aw__error">{{ errorMessage }}</p>

    <div class="aw-body">
      <nav class="aw-nav" aria-label="Secoes da automacao">
        <p class="aw-nav__head">Configuracao</p>
        <button
          v-for="s in SECTIONS"
          :key="s.id"
          type="button"
          class="aw-nav__item"
          :class="{ 'aw-nav__item--active': active === s.id }"
          @click="active = s.id"
        >
          <UIcon :name="s.icon" class="aw-nav__icon" aria-hidden="true" />
          <span class="aw-nav__label">{{ s.label }}</span>
          <span v-if="s.id === 'knowledge' && docs.length" class="aw-nav__badge">
            {{ docs.length }}
          </span>
        </button>
      </nav>

      <div class="aw-panel">
        <AutomationBehaviorCard
          v-if="active === 'behavior'"
          :persona-name="personaName"
          :persona-prompt="personaPrompt"
          :persona-loading="personaLoading"
          :saving-persona="savingPersona"
          :persona-saved-at="personaSavedAt"
          @update:persona-name="personaName = $event"
          @update:persona-prompt="personaPrompt = $event"
          @save="savePersona"
        />
        <AutomationSourcesCard v-else-if="active === 'sources'" />
        <AutomationModelsCard v-else-if="active === 'models'" @change="ctxPreview?.refresh()" />
        <AutomationKnowledgeCard v-else-if="active === 'knowledge'" @change="onKnowledgeChange" />
        <AutomationContextPreview v-else-if="active === 'context'" ref="ctxPreview" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.aw {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.5rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.aw__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.6rem 0.85rem;
  border-radius: 0.5rem;
  font-size: 0.9rem;
}

/* QR inline (so aparece quando ha um QR para escanear) */
.aw-qr {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  padding: 1.1rem 1.25rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
}

.aw-qr__img {
  border-radius: 0.5rem;
  background: rgb(255 255 255);
  padding: 0.4rem;
  flex-shrink: 0;
}

.aw-qr__title {
  font-weight: 600;
}

.aw-qr__hint {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-top: 0.2rem;
}

/* Corpo: sidebar + painel */
.aw-body {
  display: grid;
  grid-template-columns: 244px minmax(0, 1fr);
  gap: 1.5rem;
  align-items: start;
  flex: 1;
  min-height: 0;
}

.aw-nav {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  position: sticky;
  top: 0;
}

.aw-nav__head {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  padding: 0 0.85rem;
  margin-bottom: 0.5rem;
}

.aw-nav__item {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  padding: 0.7rem 0.85rem;
  border-radius: 0.7rem;
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.92rem;
  font-weight: 500;
  text-align: left;
  transition:
    background 0.12s ease,
    color 0.12s ease;
}

.aw-nav__item:hover {
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
}

.aw-nav__item--active {
  background: rgb(var(--primary) / 0.15);
  color: var(--text-main);
  font-weight: 600;
}

.aw-nav__item--active .aw-nav__icon {
  color: rgb(var(--primary));
}

.aw-nav__icon {
  width: 1.1rem;
  height: 1.1rem;
  flex-shrink: 0;
}

.aw-nav__label {
  flex: 1;
  min-width: 0;
}

.aw-nav__badge {
  font-size: 0.72rem;
  font-weight: 700;
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
  flex-shrink: 0;
}

.aw-panel {
  min-width: 0;
}

@media (max-width: 880px) {
  .aw-body {
    grid-template-columns: 1fr;
  }

  .aw-nav {
    position: static;
    flex-direction: row;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .aw-nav__head {
    display: none;
  }

  .aw-nav__item {
    width: auto;
    white-space: nowrap;
  }
}
</style>
