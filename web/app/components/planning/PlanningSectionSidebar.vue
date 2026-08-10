<script setup lang="ts">
import { Building2, CalendarDays, Settings, Target } from 'lucide-vue-next'
import type { PlanningSectionId } from '~/domain/planning/types'

defineProps<{ modelValue: PlanningSectionId; canEdit: boolean }>()
const emit = defineEmits<{ openSettings: [] }>()

const sections = [
  {
    id: 'goals' as const,
    label: 'Metas',
    description: 'Loja e indicadores comerciais',
    icon: Target,
    to: '/planejamento/metas',
  },
  {
    id: 'operation' as const,
    label: 'Funcionamento',
    description: 'Dias e horários da loja',
    icon: Building2,
    to: '/planejamento/funcionamento',
  },
  {
    id: 'schedule' as const,
    label: 'Escala',
    description: 'Equipe por dia da semana',
    icon: CalendarDays,
    to: '/planejamento/escalas',
  },
]
</script>

<template>
  <aside class="planning-sidebar" aria-label="Áreas do planejamento">
    <span class="planning-sidebar__label">Planejamento</span>
    <nav>
      <NuxtLink
        v-for="section in sections"
        :key="section.id"
        :class="{ 'is-active': modelValue === section.id }"
        :to="section.to"
      >
        <component :is="section.icon" :size="17" :stroke-width="2" />
        <span>
          <strong>{{ section.label }}</strong>
          <small>{{ section.description }}</small>
        </span>
      </NuxtLink>
    </nav>
    <button
      type="button"
      class="planning-sidebar__settings"
      title="Configurações do planejamento"
      aria-label="Configurações do planejamento"
      :disabled="!canEdit"
      @click="emit('openSettings')"
    >
      <Settings :size="16" />
      <span>Configurações</span>
    </button>
  </aside>
</template>

<style scoped>
.planning-sidebar {
  display: grid;
  align-content: start;
  gap: 0.55rem;
  min-width: 0;
  border-right: 1px solid rgb(var(--border) / 0.62);
  padding: 0.75rem;
  background: rgb(var(--surface-2) / 0.38);
}
.planning-sidebar__label {
  padding: 0 0.4rem;
  color: var(--text-muted);
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.planning-sidebar nav {
  display: grid;
  gap: 0.28rem;
}
.planning-sidebar a {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  width: 100%;
  border: 1px solid transparent;
  border-radius: 0.7rem;
  padding: 0.62rem;
  background: transparent;
  color: var(--text-muted);
  text-align: left;
  cursor: pointer;
  text-decoration: none;
}
.planning-sidebar a:hover {
  background: rgb(var(--surface) / 0.7);
}
.planning-sidebar a.is-active {
  border-color: rgb(var(--primary) / 0.28);
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}
.planning-sidebar a > span {
  display: grid;
  gap: 0.12rem;
  min-width: 0;
}
.planning-sidebar strong {
  color: inherit;
  font-size: 0.74rem;
}
.planning-sidebar small {
  color: var(--text-muted);
  font-size: 0.62rem;
  line-height: 1.3;
}
.planning-sidebar__settings {
  width: 100%;
  margin-top: 0.25rem;
  border-color: rgb(var(--border) / 0.62) !important;
  background: transparent !important;
  color: var(--text-muted) !important;
}
@media (max-width: 820px) {
  .planning-sidebar {
    display: block;
    border-right: 0;
    border-bottom: 1px solid rgb(var(--border) / 0.62);
    overflow-x: auto;
  }
  .planning-sidebar__label {
    display: none;
  }
  .planning-sidebar nav {
    display: flex;
    min-width: max-content;
  }
  .planning-sidebar__settings {
    width: auto;
  }
  .planning-sidebar a {
    width: auto;
    min-width: 9.5rem;
  }
}
</style>
