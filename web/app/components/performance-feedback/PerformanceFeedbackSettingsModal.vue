<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import type {
  PerformanceFeedbackSection,
  PerformanceFeedbackSettings,
} from '~/types/performance-feedback'

type DrawerMode = 'side' | 'center' | 'fullscreen'

const props = defineProps<{
  open: boolean
  settings: PerformanceFeedbackSettings | null
  loading: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void
  (
    event: 'save',
    value: {
      cadence: PerformanceFeedbackSettings['cadence']
      defaultSections: PerformanceFeedbackSection[]
      expectedVersion: number
    },
  ): void
}>()

const mode = ref<DrawerMode>('center')
const cadence = ref<PerformanceFeedbackSettings['cadence']>('monthly')
const sections = ref<PerformanceFeedbackSection[]>([])
let sectionSequence = 0

const canSave = computed(
  () =>
    Boolean(props.settings) &&
    sections.value.length > 0 &&
    sections.value.length <= 20 &&
    sections.value.every((section) => Boolean(section.title.trim())),
)

watch(
  () => [props.open, props.settings] as const,
  ([open, settings]) => {
    if (!open || !settings) return
    cadence.value = settings.cadence
    sections.value = settings.defaultSections.map((section) => ({
      ...section,
      contentHtml: '',
    }))
  },
  { immediate: true },
)

function addSection(): void {
  if (sections.value.length >= 20) return
  sectionSequence += 1
  sections.value.push({
    id: `default-${Date.now()}-${sectionSequence}`,
    title: 'Novo tópico',
    contentHtml: '',
  })
}

function removeSection(index: number): void {
  if (sections.value.length <= 1) return
  sections.value = sections.value.filter((_, currentIndex) => currentIndex !== index)
}

function save(): void {
  if (!canSave.value || !props.settings) return
  emit('save', {
    cadence: cadence.value,
    defaultSections: sections.value.map((section) => ({
      ...section,
      title: section.title.trim(),
      contentHtml: '',
    })),
    expectedVersion: props.settings.version,
  })
}
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    preference-key="performance-feedback-settings"
    title="Padrão dos feedbacks"
    subtitle="Defina a frequência e os tópicos usados em novos ciclos."
    @update:model-value="emit('update:open', $event)"
  >
    <div v-if="loading && !settings" class="pf-settings-drawer__loading">
      <USkeleton class="h-24 w-full" />
      <USkeleton class="h-52 w-full" />
    </div>

    <div v-else-if="settings" class="pf-settings-drawer__body">
      <section class="pf-settings-drawer__section">
        <div class="pf-settings-drawer__section-copy">
          <h3>Frequência</h3>
          <p>Semanal libera as quatro semanas; mensal trabalha o mês completo.</p>
        </div>
        <div class="pf-settings-drawer__cadence" role="group" aria-label="Frequência do feedback">
          <button
            type="button"
            :class="{ 'is-active': cadence === 'monthly' }"
            @click="cadence = 'monthly'"
          >
            Mensal
          </button>
          <button
            type="button"
            :class="{ 'is-active': cadence === 'weekly' }"
            @click="cadence = 'weekly'"
          >
            Semanal
          </button>
        </div>
      </section>

      <section class="pf-settings-drawer__section">
        <header class="pf-settings-drawer__section-header">
          <div class="pf-settings-drawer__section-copy">
            <h3>Tópicos padrão</h3>
            <p>Mantenha apenas um tópico ou adicione até vinte.</p>
          </div>
          <UButton
            icon="i-lucide-plus"
            label="Adicionar"
            color="neutral"
            variant="soft"
            size="sm"
            :disabled="sections.length >= 20"
            @click="addSection"
          />
        </header>

        <div class="pf-settings-drawer__topics">
          <div v-for="(section, index) in sections" :key="section.id">
            <UIcon name="i-lucide-grip-vertical" />
            <UInput
              v-model="section.title"
              class="pf-settings-drawer__topic-input"
              maxlength="160"
              placeholder="Título do tópico"
              :aria-label="`Título do tópico ${index + 1}`"
            />
            <UButton
              icon="i-lucide-trash-2"
              :aria-label="`Remover tópico ${index + 1}`"
              color="error"
              variant="ghost"
              size="sm"
              :disabled="sections.length <= 1"
              @click="removeSection(index)"
            />
          </div>
        </div>
      </section>
    </div>

    <div v-else class="pf-settings-drawer__empty">
      Não foi possível carregar a configuração deste tenant.
    </div>

    <template #footer>
      <UButton
        label="Cancelar"
        color="neutral"
        variant="ghost"
        size="sm"
        @click="emit('update:open', false)"
      />
      <UButton
        label="Salvar"
        icon="i-lucide-check"
        size="sm"
        :loading="saving"
        :disabled="!canSave"
        @click="save"
      />
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.pf-settings-drawer__loading,
.pf-settings-drawer__body,
.pf-settings-drawer__topics {
  display: grid;
  gap: 0.75rem;
}

.pf-settings-drawer__section {
  display: grid;
  gap: 0.85rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: var(--radius-md);
  background: rgb(var(--surface) / 0.65);
}

.pf-settings-drawer__section-header,
.pf-settings-drawer__topics > div {
  display: flex;
  align-items: center;
}

.pf-settings-drawer__section-header {
  justify-content: space-between;
  gap: 1rem;
}

.pf-settings-drawer__section-copy {
  display: grid;
  gap: 0.2rem;
}

.pf-settings-drawer__section-copy h3,
.pf-settings-drawer__section-copy p {
  margin: 0;
}

.pf-settings-drawer__section-copy h3 {
  font-size: 0.9rem;
}

.pf-settings-drawer__section-copy p,
.pf-settings-drawer__empty {
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.pf-settings-drawer__cadence {
  display: inline-flex;
  width: fit-content;
  padding: 0.2rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.7);
}

.pf-settings-drawer__cadence button {
  padding: 0.45rem 0.8rem;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: rgb(var(--muted));
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
}

.pf-settings-drawer__cadence button.is-active {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.pf-settings-drawer__topics > div {
  gap: 0.45rem;
}

.pf-settings-drawer__topic-input {
  flex: 1;
  min-width: 0;
}

.pf-settings-drawer__topics > div > svg {
  color: rgb(var(--muted) / 0.7);
}
</style>
