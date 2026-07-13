<script setup lang="ts">
import { computed } from 'vue'
import {
  ROADMAP_PHASES,
  ROADMAP_GROUPS,
  type PhaseStatus,
  type RoadmapPhase,
} from '~/components/roadmap/roadmap-data'

const STATUS_LABEL: Record<PhaseStatus, string> = {
  pending: 'Pendente',
  in_progress: 'Em andamento',
  done: 'Concluido',
  blocked: 'Bloqueado',
}

const STATUS_ICON: Record<PhaseStatus, string> = {
  pending: 'schedule',
  in_progress: 'autorenew',
  done: 'check_circle',
  blocked: 'report',
}

const phases = computed<RoadmapPhase[]>(() => ROADMAP_PHASES)

const groupedPhases = computed(() => {
  const DEFAULT_GROUP = 'multi-tenant'
  const groups = ROADMAP_GROUPS.map((g) => ({
    ...g,
    phases: phases.value.filter((p) => (p.group ?? DEFAULT_GROUP) === g.id),
  }))
  return groups.filter((g) => g.phases.length > 0)
})

const anchorItems = computed(() => {
  const items: Array<{
    id: string
    code: string
    title: string
    status: PhaseStatus
    progress: number
    groupLabel?: string
  }> = []
  const DEFAULT_GROUP = 'multi-tenant'
  let lastGroup = ''
  for (const phase of phases.value) {
    const group = phase.group ?? DEFAULT_GROUP
    if (group !== lastGroup) {
      const groupDef = ROADMAP_GROUPS.find((g) => g.id === group)
      if (groupDef)
        items.push({
          id: `group-${group}`,
          code: '',
          title: groupDef.label,
          status: 'pending',
          progress: 0,
          groupLabel: groupDef.label,
        })
      lastGroup = group
    }
    items.push({
      id: phase.id,
      code: phase.code,
      title: phase.title,
      status: phase.status,
      progress: phaseProgress(phase),
    })
  }
  return items
})

const totals = computed(() => {
  const counters = { total: 0, pending: 0, in_progress: 0, done: 0, blocked: 0 }
  for (const phase of phases.value) {
    counters.total += 1
    counters[phase.status] += 1
  }
  return counters
})

const overallProgress = computed(() => {
  const totalTasks = phases.value.reduce((acc, phase) => acc + phase.tasks.length, 0)
  const doneTasks = phases.value.reduce(
    (acc, phase) => acc + phase.tasks.filter((task) => task.done).length,
    0,
  )
  if (totalTasks === 0) return 0
  return Math.round((doneTasks / totalTasks) * 100)
})

function phaseProgress(phase: RoadmapPhase) {
  if (phase.tasks.length === 0) return 0
  const done = phase.tasks.filter((task) => task.done).length
  return Math.round((done / phase.tasks.length) * 100)
}

function scrollToPhase(phaseId: string) {
  const target = document.getElementById(phaseId)
  if (!target) return
  target.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<template>
  <div class="roadmap-timeline-view">
    <header class="roadmap-stats-header">
      <div class="roadmap-stats">
        <div class="roadmap-stat">
          <span class="roadmap-stat__value">{{ totals.total }}</span>
          <span class="roadmap-stat__label">Fases</span>
        </div>
        <div class="roadmap-stat roadmap-stat--done">
          <span class="roadmap-stat__value">{{ totals.done }}</span>
          <span class="roadmap-stat__label">Concluidas</span>
        </div>
        <div class="roadmap-stat roadmap-stat--in-progress">
          <span class="roadmap-stat__value">{{ totals.in_progress }}</span>
          <span class="roadmap-stat__label">Em andamento</span>
        </div>
        <div class="roadmap-stat roadmap-stat--pending">
          <span class="roadmap-stat__value">{{ totals.pending }}</span>
          <span class="roadmap-stat__label">Pendentes</span>
        </div>
        <div v-if="totals.blocked > 0" class="roadmap-stat roadmap-stat--blocked">
          <span class="roadmap-stat__value">{{ totals.blocked }}</span>
          <span class="roadmap-stat__label">Bloqueadas</span>
        </div>
      </div>

      <div class="roadmap-progress">
        <div
          class="roadmap-progress__bar"
          role="progressbar"
          :aria-valuenow="overallProgress"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <div class="roadmap-progress__fill" :style="{ width: `${overallProgress}%` }"></div>
        </div>
        <span class="roadmap-progress__label">{{ overallProgress }}% das tarefas concluidas</span>
      </div>
    </header>

    <div class="roadmap-timeline-layout">
      <aside class="roadmap-anchor-menu" aria-label="Navegar pelas fases do roadmap">
        <span class="roadmap-anchor-menu__eyebrow">Fases</span>
        <nav class="roadmap-anchor-menu__list">
          <template v-for="item in anchorItems" :key="item.id">
            <span v-if="item.groupLabel" class="roadmap-anchor-menu__group-label">
              {{ item.groupLabel }}
            </span>
            <button
              v-else
              type="button"
              class="roadmap-anchor-menu__item"
              :class="`roadmap-anchor-menu__item--${item.status}`"
              @click="scrollToPhase(item.id)"
            >
              <span class="roadmap-anchor-menu__dot" aria-hidden="true"></span>
              <span class="roadmap-anchor-menu__body">
                <span class="roadmap-anchor-menu__code">{{ item.code }}</span>
                <span class="roadmap-anchor-menu__title">{{ item.title }}</span>
              </span>
              <span class="roadmap-anchor-menu__progress">{{ item.progress }}%</span>
            </button>
          </template>
        </nav>
      </aside>

      <div class="roadmap-timeline-groups">
        <section v-for="group in groupedPhases" :key="group.id" class="roadmap-group">
          <header class="roadmap-group__header">
            <h2 class="roadmap-group__title">{{ group.label }}</h2>
            <p v-if="group.description" class="roadmap-group__description">
              {{ group.description }}
            </p>
          </header>

          <ol class="roadmap-timeline">
            <li
              v-for="phase in group.phases"
              :id="phase.id"
              :key="phase.id"
              class="roadmap-phase"
              :class="`roadmap-phase--${phase.status}`"
            >
              <div class="roadmap-phase__marker" aria-hidden="true">
                <span class="material-icons-round">{{ STATUS_ICON[phase.status] }}</span>
              </div>

              <article class="roadmap-phase__card">
                <header class="roadmap-phase__header">
                  <div class="roadmap-phase__title-row">
                    <span class="roadmap-phase__code">{{ phase.code }}</span>
                    <h3 class="roadmap-phase__title">{{ phase.title }}</h3>
                    <span
                      class="roadmap-phase__status"
                      :class="`roadmap-phase__status--${phase.status}`"
                    >
                      {{ STATUS_LABEL[phase.status] }}
                    </span>
                  </div>
                  <p class="roadmap-phase__goal">{{ phase.goal }}</p>
                </header>

                <dl class="roadmap-phase__meta">
                  <div class="roadmap-phase__meta-item">
                    <dt>Estimativa</dt>
                    <dd>{{ phase.estimateWeeks }}</dd>
                  </div>
                  <div v-if="phase.startedAt" class="roadmap-phase__meta-item">
                    <dt>Inicio</dt>
                    <dd>{{ phase.startedAt }}</dd>
                  </div>
                  <div v-if="phase.finishedAt" class="roadmap-phase__meta-item">
                    <dt>Conclusao</dt>
                    <dd>{{ phase.finishedAt }}</dd>
                  </div>
                  <div class="roadmap-phase__meta-item">
                    <dt>Progresso</dt>
                    <dd>{{ phaseProgress(phase) }}%</dd>
                  </div>
                </dl>

                <div class="roadmap-phase__progress" aria-hidden="true">
                  <div
                    class="roadmap-phase__progress-fill"
                    :style="{ width: `${phaseProgress(phase)}%` }"
                  ></div>
                </div>

                <ul class="roadmap-phase__tasks">
                  <li
                    v-for="task in phase.tasks"
                    :key="task.id"
                    class="roadmap-task"
                    :class="{ 'roadmap-task--done': task.done }"
                  >
                    <span class="roadmap-task__check material-icons-round" aria-hidden="true">
                      {{ task.done ? 'check_box' : 'check_box_outline_blank' }}
                    </span>
                    <div class="roadmap-task__body">
                      <span class="roadmap-task__label">{{ task.label }}</span>
                      <span v-if="task.note" class="roadmap-task__note">{{ task.note }}</span>
                    </div>
                  </li>
                </ul>

                <p class="roadmap-phase__verifiable">
                  <span class="material-icons-round" aria-hidden="true">verified</span>
                  <span>{{ phase.verifiable }}</span>
                </p>

                <div
                  v-if="phase.blockers && phase.blockers.length > 0"
                  class="roadmap-phase__blockers"
                >
                  <strong>Bloqueios:</strong>
                  <ul>
                    <li v-for="(blocker, index) in phase.blockers" :key="index">{{ blocker }}</li>
                  </ul>
                </div>
              </article>
            </li>
          </ol>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.roadmap-timeline-view {
  display: grid;
  gap: 1.4rem;
}

.roadmap-timeline-layout {
  display: grid;
  grid-template-columns: minmax(180px, 230px) minmax(0, 1fr);
  align-items: start;
  gap: 1.1rem;
}

.roadmap-timeline-groups {
  display: grid;
  gap: 2.5rem;
}

.roadmap-group__header {
  margin-bottom: 1.25rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid rgb(var(--border));
}

.roadmap-group__title {
  font-size: 1.1rem;
  font-weight: 700;
  color: rgb(var(--text, 226 232 240));
  margin: 0 0 0.25rem;
}

.roadmap-group__description {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin: 0;
}

.roadmap-anchor-menu__group-label {
  display: block;
  padding: 0.35rem 0.3rem 0.1rem;
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--text-muted);
  border-top: 1px solid rgb(var(--border));
  margin-top: 0.25rem;
}

.roadmap-anchor-menu__group-label:first-child {
  border-top: none;
  margin-top: 0;
}

.roadmap-anchor-menu {
  position: sticky;
  top: 0.5rem;
  display: grid;
  gap: 0.5rem;
  padding: 0.7rem;
  border: 1px solid rgb(var(--border));
  border-radius: 16px;
  background: rgb(var(--surface-2));
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.16);
}

.roadmap-anchor-menu__eyebrow {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.roadmap-anchor-menu__list {
  display: grid;
  gap: 0.25rem;
}

.roadmap-anchor-menu__item {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
  min-height: 34px;
  padding: 0.38rem 0.5rem;
  border: 1px solid rgb(var(--border));
  border-radius: 10px;
  background: rgb(var(--surface-2));
  color: var(--text-muted);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.roadmap-anchor-menu__item:hover,
.roadmap-anchor-menu__item:focus-visible {
  border-color: rgb(var(--primary) / 0.45);
  background: rgb(var(--primary) / 0.12);
  color: var(--text-main);
  outline: none;
  transform: translateX(2px);
}

.roadmap-anchor-menu__dot {
  width: 0.48rem;
  height: 0.48rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  box-shadow: 0 0 0 3px rgba(148, 163, 184, 0.08);
}

.roadmap-anchor-menu__item--done .roadmap-anchor-menu__dot {
  background: rgb(var(--success));
  box-shadow: 0 0 0 3px rgba(74, 222, 128, 0.12);
}

.roadmap-anchor-menu__item--in_progress .roadmap-anchor-menu__dot {
  background: rgb(var(--primary));
  box-shadow: 0 0 0 3px rgb(var(--primary) / 0.14);
}

.roadmap-anchor-menu__item--blocked .roadmap-anchor-menu__dot {
  background: rgb(var(--danger));
  box-shadow: 0 0 0 3px rgb(var(--danger) / 0.14);
}

.roadmap-anchor-menu__body {
  display: grid;
  min-width: 0;
}

.roadmap-anchor-menu__code {
  color: var(--text-main);
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1.15;
}

.roadmap-anchor-menu__title {
  overflow: hidden;
  color: inherit;
  font-size: 0.7rem;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.roadmap-anchor-menu__progress {
  color: var(--text-muted);
  font-size: 0.68rem;
  font-variant-numeric: tabular-nums;
}

@media (max-height: 760px) and (min-width: 981px) {
  .roadmap-anchor-menu {
    gap: 0.4rem;
    padding: 0.6rem;
  }

  .roadmap-anchor-menu__eyebrow {
    display: none;
  }

  .roadmap-anchor-menu__item {
    min-height: 30px;
    padding: 0.32rem 0.45rem;
  }

  .roadmap-anchor-menu__title {
    display: none;
  }
}

.roadmap-stats-header {
  display: grid;
  gap: 0.85rem;
}

.roadmap-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.roadmap-stat {
  display: grid;
  gap: 0.15rem;
  padding: 0.85rem 1.1rem;
  border-radius: 14px;
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
  min-width: 110px;
}

.roadmap-stat__value {
  font-size: 1.55rem;
  font-weight: 600;
  color: var(--text-main);
  line-height: 1;
}

.roadmap-stat__label {
  font-size: 0.78rem;
  color: var(--text-muted);
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.roadmap-stat--done {
  border-color: rgb(var(--success) / 0.45);
  background: rgb(var(--success) / 0.12);
}
.roadmap-stat--done .roadmap-stat__value {
  color: rgb(var(--success));
}

.roadmap-stat--in-progress {
  border-color: rgb(var(--primary) / 0.45);
  background: rgb(var(--primary) / 0.12);
}
.roadmap-stat--in-progress .roadmap-stat__value {
  color: rgb(var(--primary));
}

.roadmap-stat--pending {
  border-color: rgb(var(--border));
  background: rgb(var(--surface-2));
}

.roadmap-stat--blocked {
  border-color: rgb(var(--danger) / 0.45);
  background: rgb(var(--danger) / 0.12);
}
.roadmap-stat--blocked .roadmap-stat__value {
  color: rgb(var(--danger));
}

.roadmap-progress {
  display: grid;
  gap: 0.35rem;
}

.roadmap-progress__bar {
  width: 100%;
  height: 8px;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  overflow: hidden;
}

.roadmap-progress__fill {
  height: 100%;
  background: linear-gradient(90deg, rgb(var(--primary)), rgb(var(--success)));
  transition: width 0.3s ease;
}

.roadmap-progress__label {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.roadmap-timeline {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 1rem;
  position: relative;
}

.roadmap-timeline::before {
  content: '';
  position: absolute;
  left: 18px;
  top: 18px;
  bottom: 18px;
  width: 2px;
  background: rgb(var(--surface-2));
}

.roadmap-phase {
  position: relative;
  padding-left: 3rem;
  scroll-margin-top: 1rem;
}

.roadmap-phase__marker {
  position: absolute;
  left: 0;
  top: 0.5rem;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: rgb(var(--surface-2));
  border: 2px solid rgb(var(--border));
  z-index: 1;
}

.roadmap-phase__marker .material-icons-round {
  font-size: 1.2rem;
  color: var(--text-muted);
}

.roadmap-phase--done .roadmap-phase__marker {
  border-color: rgb(var(--success) / 0.7);
}
.roadmap-phase--done .roadmap-phase__marker .material-icons-round {
  color: rgb(var(--success));
}

.roadmap-phase--in_progress .roadmap-phase__marker {
  border-color: rgb(var(--primary) / 0.75);
}
.roadmap-phase--in_progress .roadmap-phase__marker .material-icons-round {
  color: rgb(var(--primary));
  animation: roadmap-spin 2.4s linear infinite;
}

.roadmap-phase--blocked .roadmap-phase__marker {
  border-color: rgb(var(--danger) / 0.7);
}
.roadmap-phase--blocked .roadmap-phase__marker .material-icons-round {
  color: rgb(var(--danger));
}

@keyframes roadmap-spin {
  from {
    transform: rotate(0);
  }
  to {
    transform: rotate(360deg);
  }
}

.roadmap-phase__card {
  display: grid;
  gap: 0.85rem;
  padding: 1.15rem 1.25rem;
  border-radius: 16px;
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.18);
}

.roadmap-phase--done .roadmap-phase__card {
  border-color: rgb(var(--success) / 0.3);
}

.roadmap-phase--in_progress .roadmap-phase__card {
  border-color: rgb(var(--primary) / 0.35);
}

.roadmap-phase--blocked .roadmap-phase__card {
  border-color: rgb(var(--danger) / 0.35);
}

.roadmap-phase__header {
  display: grid;
  gap: 0.45rem;
}

.roadmap-phase__title-row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.roadmap-phase__code {
  display: inline-block;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.roadmap-phase__title {
  margin: 0;
  font-size: 1.05rem;
  color: var(--text-main);
}

.roadmap-phase__status {
  margin-left: auto;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.roadmap-phase__status--pending {
  background: rgb(var(--surface-2));
  color: var(--text-main);
}

.roadmap-phase__status--in_progress {
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
}

.roadmap-phase__status--done {
  background: rgb(var(--success) / 0.18);
  color: rgb(var(--success));
}

.roadmap-phase__status--blocked {
  background: rgb(var(--danger) / 0.18);
  color: rgb(var(--danger));
}

.roadmap-phase__goal {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.86rem;
  line-height: 1.5;
}

.roadmap-phase__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 1.2rem;
  margin: 0;
}

.roadmap-phase__meta-item {
  display: grid;
  gap: 0.1rem;
}

.roadmap-phase__meta-item dt {
  margin: 0;
  font-size: 0.7rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.roadmap-phase__meta-item dd {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-main);
}

.roadmap-phase__progress {
  width: 100%;
  height: 6px;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  overflow: hidden;
}

.roadmap-phase__progress-fill {
  height: 100%;
  background: linear-gradient(90deg, rgb(var(--primary)), rgb(var(--success)));
  transition: width 0.3s ease;
}

.roadmap-phase__tasks {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.45rem;
}

.roadmap-task {
  display: flex;
  align-items: flex-start;
  gap: 0.6rem;
  padding: 0.55rem 0.75rem;
  border-radius: 10px;
  background: rgb(var(--surface-2));
  border: 1px solid rgb(var(--border));
}

.roadmap-task--done {
  background: rgb(var(--success) / 0.08);
  border-color: rgb(var(--success) / 0.2);
}

.roadmap-task__check {
  font-size: 1.2rem;
  color: var(--text-muted);
  flex-shrink: 0;
  margin-top: 0.05rem;
}

.roadmap-task--done .roadmap-task__check {
  color: rgb(var(--success));
}

.roadmap-task__body {
  display: grid;
  gap: 0.15rem;
}

.roadmap-task__label {
  font-size: 0.86rem;
  color: var(--text-main);
  line-height: 1.4;
}

.roadmap-task--done .roadmap-task__label {
  color: var(--text-muted);
  text-decoration: line-through;
}

.roadmap-task__note {
  font-size: 0.78rem;
  color: var(--text-muted);
  font-style: italic;
}

.roadmap-phase__verifiable {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin: 0;
  padding: 0.6rem 0.8rem;
  border-radius: 10px;
  background: rgb(var(--success) / 0.06);
  border: 1px solid rgb(var(--success) / 0.18);
  color: rgb(var(--success));
  font-size: 0.82rem;
  line-height: 1.5;
}

.roadmap-phase__verifiable .material-icons-round {
  font-size: 1.05rem;
  color: rgb(var(--success));
  margin-top: 0.05rem;
  flex-shrink: 0;
}

.roadmap-phase__blockers {
  padding: 0.6rem 0.8rem;
  border-radius: 10px;
  background: rgb(var(--danger) / 0.08);
  border: 1px solid rgb(var(--danger) / 0.25);
  color: rgb(var(--danger));
  font-size: 0.82rem;
}

.roadmap-phase__blockers strong {
  color: rgb(var(--danger));
  display: block;
  margin-bottom: 0.3rem;
}

.roadmap-phase__blockers ul {
  margin: 0;
  padding-left: 1.2rem;
}

@media (max-width: 980px) {
  .roadmap-timeline-layout {
    grid-template-columns: 1fr;
  }

  .roadmap-anchor-menu {
    top: 0;
    z-index: 4;
    max-height: none;
    padding: 0.7rem;
    overflow: hidden;
  }

  .roadmap-anchor-menu__list {
    display: flex;
    gap: 0.45rem;
    overflow-x: auto;
    padding: 0 0 0.15rem;
  }

  .roadmap-anchor-menu__item {
    grid-template-columns: auto minmax(94px, 1fr);
    flex: 0 0 150px;
  }

  .roadmap-anchor-menu__progress {
    display: none;
  }
}

@media (max-width: 640px) {
  .roadmap-stat {
    min-width: calc(50% - 0.4rem);
  }

  .roadmap-phase {
    padding-left: 0;
  }

  .roadmap-timeline::before,
  .roadmap-phase__marker {
    display: none;
  }
}
</style>
