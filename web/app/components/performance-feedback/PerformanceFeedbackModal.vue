<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import OmniEditor from '~/components/omni/OmniEditor.vue'
import AppGoalPeriodFilter from '~/components/ui/AppGoalPeriodFilter.vue'
import { goalWeekCount } from '~/utils/goal-periods'
import { usePerformanceFeedback } from '~/composables/usePerformanceFeedback'
import type {
  PerformanceFeedbackSection,
  PerformanceFeedbackTarget,
} from '~/types/performance-feedback'
import PerformanceFeedbackHistory from './PerformanceFeedbackHistory.vue'
import PerformanceFeedbackMetrics from './PerformanceFeedbackMetrics.vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    target: PerformanceFeedbackTarget | null
    periodControllable?: boolean
    periodPending?: boolean
  }>(),
  {
    periodControllable: false,
    periodPending: false,
  },
)

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void
  (event: 'change-period', value: { month: string; week: number }): void
}>()

const feedback = usePerformanceFeedback()
let sectionSequence = 0
const openSectionId = ref<string | null>(null)

const statusLabels: Record<string, string> = {
  draft: 'Rascunho',
  shared: 'Aguardando consultor',
  acknowledged: 'Devolutiva registrada',
}

const canSave = computed(
  () =>
    feedback.canManage.value &&
    feedback.feedbackSections.value.length <= 20 &&
    feedback.feedbackSections.value.every((section) => Boolean(section.title.trim())),
)

const displayedMetrics = computed(() => props.target?.metrics ?? null)
const weeklyCadence = computed(() => feedback.context.value?.settings.cadence === 'weekly')

watch(
  [() => props.open, () => props.target?.storeId, () => props.target?.consultantId],
  ([open]) => {
    if (!open || !props.target) return
    void openFeedbackTarget(props.target)
  },
)

async function openFeedbackTarget(target: PerformanceFeedbackTarget): Promise<void> {
  await feedback.openFor(target)
  const requestedWeek = Math.min(
    goalWeekCount(feedback.month.value),
    Math.max(0, Math.trunc(Number(target.week || 0))),
  )
  if (feedback.week.value === requestedWeek) return
  emit('change-period', { month: feedback.month.value, week: feedback.week.value })
}

watch(
  () => feedback.feedbackSections.value.map((section) => section.id),
  (sectionIds) => {
    if (openSectionId.value && sectionIds.includes(openSectionId.value)) return
    openSectionId.value = sectionIds[0] ?? null
  },
  { immediate: true },
)

function updateSections(sections: PerformanceFeedbackSection[]): void {
  feedback.feedbackSections.value = sections
}

function addSection(): void {
  if (!feedback.canManage.value || feedback.feedbackSections.value.length >= 20) return
  sectionSequence += 1
  const sectionId = `custom-${Date.now()}-${sectionSequence}`
  updateSections([
    ...feedback.feedbackSections.value,
    {
      id: sectionId,
      title: 'Novo tópico',
      contentHtml: '',
    },
  ])
  openSectionId.value = sectionId
}

function updateSectionTitle(index: number, value: unknown): void {
  updateSections(
    feedback.feedbackSections.value.map((section, currentIndex) =>
      currentIndex === index ? { ...section, title: String(value ?? '') } : section,
    ),
  )
}

function updateSectionContent(index: number, value: unknown): void {
  updateSections(
    feedback.feedbackSections.value.map((section, currentIndex) =>
      currentIndex === index ? { ...section, contentHtml: String(value ?? '') } : section,
    ),
  )
}

function removeSection(index: number): void {
  if (!feedback.canManage.value) return
  const removedSection = feedback.feedbackSections.value[index]
  const remainingSections = feedback.feedbackSections.value.filter(
    (_, currentIndex) => currentIndex !== index,
  )
  updateSections(remainingSections)

  if (removedSection?.id === openSectionId.value) {
    openSectionId.value = remainingSections[index]?.id ?? remainingSections[index - 1]?.id ?? null
  }
}

function toggleSection(sectionId: string): void {
  openSectionId.value = openSectionId.value === sectionId ? null : sectionId
}

function handleSectionHeaderClick(sectionId: string, event: MouseEvent): void {
  const target = event.target
  if (target instanceof Element && target.closest('button, input, textarea, [contenteditable]')) {
    return
  }
  toggleSection(sectionId)
}

function isSectionFilled(contentHtml: string): boolean {
  return (
    contentHtml
      .replace(/<[^>]*>/g, '')
      .replace(/&nbsp;|&#160;/gi, '')
      .trim().length > 0
  )
}

async function chooseWeek(value: number): Promise<void> {
  feedback.week.value = value
  emit('change-period', { month: feedback.month.value, week: value })
  await feedback.load()
}

function selectedGoalPeriod(): string {
  return feedback.week.value === 0 ? 'month' : `p${feedback.week.value}`
}

function chooseGoalPeriod(value: string): void {
  const week = value === 'month' ? 0 : Number(value.replace(/^p/, ''))
  if (!Number.isInteger(week) || week < 0 || week > goalWeekCount(feedback.month.value)) return
  void chooseWeek(week)
}

async function applySelectedPeriod(): Promise<void> {
  emit('change-period', { month: feedback.month.value, week: feedback.week.value })
  await feedback.load()
}

function saveFeedback(): void {
  if (!displayedMetrics.value) return
  void feedback.saveManager(displayedMetrics.value)
}
</script>

<template>
  <UModal :open="open" :ui="{ content: 'max-w-6xl' }" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard
        class="pf-modal-card"
        :ui="{ header: 'px-5 py-3 sm:px-5', body: 'px-5 py-3 sm:p-5 sm:py-3' }"
      >
        <template #header>
          <div class="pf-modal-card__header">
            <div class="pf-modal-card__identity">
              <span class="pf-section-eyebrow">Feedback de desempenho</span>
              <h2>{{ target?.consultantName || 'Consultor' }}</h2>
              <p>
                {{ target?.storeName || feedback.context.value?.store.name || 'Loja selecionada' }}
              </p>
            </div>
            <section class="pf-modal-filters" aria-label="Período do feedback">
              <label v-if="periodControllable" class="pf-field">
                <span class="sr-only">Mês de referência</span>
                <input
                  v-model="feedback.month.value"
                  type="month"
                  :disabled="periodPending"
                  @change="applySelectedPeriod"
                />
              </label>
              <div v-if="periodControllable && weeklyCadence" class="pf-period-filter">
                <span class="sr-only">Período</span>
                <AppGoalPeriodFilter
                  :month="selectedMonth"
                  :model-value="selectedGoalPeriod()"
                  :disabled="periodPending"
                  aria-label="Período do feedback"
                  @update:model-value="chooseGoalPeriod"
                />
              </div>
              <UButton
                v-if="periodControllable"
                icon="i-lucide-refresh-cw"
                aria-label="Atualizar indicadores"
                color="neutral"
                variant="ghost"
                size="sm"
                :loading="feedback.pending.value || periodPending"
                @click="applySelectedPeriod"
              />
              <div v-if="feedback.context.value" class="pf-modal-period">
                <UIcon name="i-lucide-calendar-range" />
                <strong>{{ feedback.context.value.period.label }}</strong>
                <small>
                  {{ feedback.context.value.period.dateFrom }} a
                  {{ feedback.context.value.period.dateTo }}
                </small>
              </div>
            </section>
            <div class="pf-modal-card__header-actions">
              <span
                v-if="feedback.selectedReview.value"
                class="pf-status"
                :data-status="feedback.selectedReview.value.status"
              >
                {{ statusLabels[feedback.selectedReview.value.status] }}
              </span>
              <UButton
                icon="i-lucide-x"
                aria-label="Fechar feedback"
                color="neutral"
                variant="ghost"
                @click="emit('update:open', false)"
              />
            </div>
          </div>
        </template>

        <div class="pf-modal-card__body">
          <div v-if="feedback.pending.value && !feedback.context.value" class="pf-loading">
            <USkeleton class="h-28 w-full" />
            <USkeleton class="h-52 w-full" />
          </div>

          <div v-else-if="feedback.errorMessage.value" class="pf-state pf-state--error">
            <UIcon name="i-lucide-circle-alert" />
            <div>
              <strong>Não foi possível abrir este feedback</strong>
              <p>{{ feedback.errorMessage.value }}</p>
            </div>
            <UButton
              label="Tentar novamente"
              color="neutral"
              variant="soft"
              @click="feedback.load()"
            />
          </div>

          <template v-else-if="feedback.context.value?.selectedConsultant">
            <PerformanceFeedbackMetrics v-if="displayedMetrics" :metrics="displayedMetrics" />

            <div
              v-if="!feedback.canManage.value && !feedback.selectedReview.value"
              class="pf-state"
            >
              <UIcon name="i-lucide-message-square-dashed" />
              <div>
                <strong>Nenhum feedback compartilhado neste período</strong>
                <p>Quando a gestão publicar o registro, ele aparecerá aqui.</p>
              </div>
            </div>

            <section v-else class="pf-dynamic-editors">
              <header class="pf-dynamic-editors__header">
                <div>
                  <h3>Tópicos do feedback</h3>
                  <p>Crie, renomeie ou remova blocos conforme a necessidade desta conversa.</p>
                </div>
                <div v-if="feedback.canManage.value" class="pf-dynamic-editors__tools">
                  <UButton
                    icon="i-lucide-plus"
                    label="Adicionar"
                    color="neutral"
                    variant="soft"
                    size="sm"
                    :disabled="feedback.feedbackSections.value.length >= 20"
                    @click="addSection"
                  />
                </div>
              </header>

              <div v-if="feedback.feedbackSections.value.length" class="pf-dynamic-editors__list">
                <article
                  v-for="(section, index) in feedback.feedbackSections.value"
                  :key="section.id"
                  class="pf-dynamic-editor"
                  :class="{ 'is-open': openSectionId === section.id }"
                >
                  <header
                    class="pf-dynamic-editor__header"
                    @click="handleSectionHeaderClick(section.id, $event)"
                  >
                    <UButton
                      :icon="
                        openSectionId === section.id
                          ? 'i-lucide-chevron-up'
                          : 'i-lucide-chevron-down'
                      "
                      :aria-label="
                        openSectionId === section.id ? 'Recolher tópico' : 'Expandir tópico'
                      "
                      :aria-expanded="openSectionId === section.id"
                      :aria-controls="`feedback-editor-${section.id}`"
                      color="neutral"
                      variant="ghost"
                      size="sm"
                      @click="toggleSection(section.id)"
                    />
                    <UInput
                      v-if="feedback.canManage.value"
                      :model-value="section.title"
                      class="pf-dynamic-editor__title-input"
                      size="sm"
                      maxlength="160"
                      aria-label="Título do tópico"
                      @focus="openSectionId = section.id"
                      @update:model-value="updateSectionTitle(index, $event)"
                    />
                    <button
                      v-else
                      type="button"
                      class="pf-dynamic-editor__title"
                      :aria-expanded="openSectionId === section.id"
                      :aria-controls="`feedback-editor-${section.id}`"
                      @click="toggleSection(section.id)"
                    >
                      {{ section.title }}
                    </button>
                    <span
                      class="pf-dynamic-editor__state"
                      :class="{ 'is-filled': isSectionFilled(section.contentHtml) }"
                    >
                      {{ isSectionFilled(section.contentHtml) ? 'Preenchido' : 'Em branco' }}
                    </span>
                    <UButton
                      v-if="feedback.canManage.value"
                      icon="i-lucide-trash-2"
                      aria-label="Remover tópico"
                      color="error"
                      variant="ghost"
                      @click="removeSection(index)"
                    />
                  </header>
                  <div
                    v-show="openSectionId === section.id"
                    :id="`feedback-editor-${section.id}`"
                    class="pf-dynamic-editor__body"
                  >
                    <OmniEditor
                      :model-value="section.contentHtml"
                      content-type="html"
                      :editable="feedback.canManage.value"
                      :people="[]"
                      :clients="[]"
                      :tasks="[]"
                      min-height="320px"
                      max-height="58vh"
                      compact
                      placeholder="Escreva o que precisa ser registrado neste tópico..."
                      @update:model-value="updateSectionContent(index, $event)"
                    />
                  </div>
                </article>
              </div>
              <div v-else class="pf-dynamic-editors__empty">
                <UIcon name="i-lucide-notebook-pen" />
                <span>Adicione um editor para começar o feedback.</span>
              </div>

              <footer v-if="feedback.canManage.value" class="pf-dynamic-editors__actions">
                <UButton
                  label="Salvar feedback"
                  icon="i-lucide-check"
                  size="sm"
                  :loading="feedback.saving.value"
                  :disabled="!canSave"
                  @click="saveFeedback"
                />
              </footer>
            </section>

            <section
              v-if="
                feedback.selectedReview.value && feedback.selectedReview.value.status !== 'draft'
              "
              class="pf-consultant-response"
            >
              <header>
                <span class="pf-section-eyebrow">Devolutiva do consultor</span>
                <h3>Percepção e compromissos</h3>
              </header>
              <OmniEditor
                v-model="feedback.consultantNotesHtml.value"
                content-type="html"
                :editable="feedback.canRespond.value"
                :people="[]"
                :clients="[]"
                :tasks="[]"
                min-height="118px"
                max-height="240px"
                compact
                placeholder="Registre como recebeu o feedback e os compromissos para o próximo ciclo..."
              />
              <footer v-if="feedback.canRespond.value">
                <UButton
                  label="Registrar minha devolutiva"
                  icon="i-lucide-check-circle-2"
                  :loading="feedback.saving.value"
                  @click="feedback.saveConsultant()"
                />
              </footer>
            </section>

            <PerformanceFeedbackHistory :items="feedback.context.value.history" />
          </template>
        </div>
      </UCard>
    </template>
  </UModal>
</template>

<style src="./performance-feedback.css"></style>

<style scoped>
.pf-modal-card__header,
.pf-modal-card__header-actions,
.pf-modal-filters,
.pf-modal-period,
.pf-dynamic-editors__header,
.pf-dynamic-editors__tools,
.pf-dynamic-editor__header,
.pf-dynamic-editors__actions,
.pf-consultant-response footer {
  display: flex;
  align-items: center;
}

.pf-modal-card__header {
  justify-content: space-between;
  gap: 0.75rem;
}

.pf-modal-card__identity {
  min-width: 11rem;
  flex: none;
}

.pf-modal-card__header h2 {
  font-size: 1rem;
  line-height: 1.3;
}

.pf-modal-card__header h2,
.pf-modal-card__header p,
.pf-dynamic-editors h3,
.pf-dynamic-editors p,
.pf-dynamic-editor h4,
.pf-consultant-response h3 {
  margin: 0;
}

.pf-modal-card__header p,
.pf-dynamic-editors p {
  color: rgb(var(--muted) / 0.92);
  font-size: 0.78rem;
}

.pf-modal-card__header-actions {
  gap: 0.5rem;
}

.pf-modal-card__body {
  display: grid;
  gap: 0.7rem;
  max-height: min(80vh, 52rem);
  overflow-y: auto;
  padding-right: 0.2rem;
}

.pf-modal-filters {
  min-width: 0;
  flex: 1;
  flex-wrap: wrap;
  justify-content: center;
  gap: 0.4rem;
}

.pf-modal-filters .pf-field,
.pf-modal-filters .pf-period-filter {
  gap: 0;
}

.pf-modal-period {
  flex-wrap: wrap;
  gap: 0.32rem;
  color: rgb(var(--muted) / 0.94);
  font-size: 0.7rem;
}

.pf-modal-period strong {
  color: rgb(var(--text) / 0.96);
}

.pf-dynamic-editors,
.pf-dynamic-editors__list,
.pf-consultant-response {
  display: grid;
  gap: 0.55rem;
}

.pf-dynamic-editors__header {
  justify-content: space-between;
  gap: 1rem;
}

.pf-dynamic-editors__tools {
  gap: 0.3rem;
}

.pf-dynamic-editor,
.pf-consultant-response {
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: 0.9rem;
  background: rgb(var(--surface) / 0.72);
}

.pf-consultant-response {
  padding: 0.5rem;
}

.pf-dynamic-editor {
  overflow: hidden;
  transition:
    border-color 160ms ease,
    background-color 160ms ease;
}

.pf-dynamic-editor.is-open {
  border-color: rgb(var(--primary) / 0.48);
  background: rgb(var(--surface) / 0.9);
}

.pf-dynamic-editor__header {
  min-height: 3rem;
  gap: 0.4rem;
  padding: 0.4rem 0.5rem;
  cursor: pointer;
}

.pf-dynamic-editor__header :deep(input) {
  cursor: text;
}

.pf-dynamic-editor__header :deep(button) {
  cursor: pointer;
}

.pf-dynamic-editor__title-input,
.pf-dynamic-editor__title {
  flex: 1 1 auto;
  width: 100%;
  min-width: 0;
}

.pf-dynamic-editor__title-input :deep(input) {
  width: 100%;
}

.pf-dynamic-editor__title {
  border: 0;
  background: transparent;
  color: rgb(var(--text));
  font: inherit;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
}

.pf-dynamic-editor__state {
  flex: none;
  padding: 0.18rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.1);
  color: rgb(var(--muted) / 0.9);
  font-size: 0.68rem;
  line-height: 1.2;
}

.pf-dynamic-editor__state.is-filled {
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success) / 0.98);
}

.pf-dynamic-editor__body {
  border-top: 1px solid rgb(var(--border) / 0.65);
}

.pf-dynamic-editors__actions,
.pf-consultant-response footer {
  justify-content: flex-end;
  gap: 0.5rem;
}

.pf-dynamic-editors__empty {
  display: grid;
  justify-items: center;
  gap: 0.5rem;
  padding: 1.25rem;
  border: 1px dashed rgb(var(--border) / 0.9);
  border-radius: 0.9rem;
  color: rgb(var(--muted) / 0.92);
}

@media (max-width: 720px) {
  .pf-modal-card__header,
  .pf-dynamic-editors__header {
    align-items: flex-start;
  }

  .pf-modal-card__header {
    flex-wrap: wrap;
  }

  .pf-modal-filters {
    width: 100%;
    flex-basis: 100%;
    justify-content: flex-start;
    order: 3;
  }

  .pf-modal-filters {
    align-items: stretch;
  }

  .pf-modal-period {
    width: 100%;
  }
}
</style>
