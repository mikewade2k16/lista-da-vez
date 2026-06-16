<script setup lang="ts">
import { useAdminPageHeaderVisibility } from '../../../core/composables/useAdminPageHeaderVisibility'

interface AdminPageHeaderProps {
  eyebrow?: string
  title?: string
  description?: string
}

const props = withDefaults(defineProps<AdminPageHeaderProps>(), {
  eyebrow: '',
  title: '',
  description: '',
})

// Respeita o toggle GLOBAL de cabecalho (themes > PAGE HEADERS): eyebrow/title/
// description so aparecem se ligados no tema. Espelha o AdminPageHeader do core —
// paginas que usam este (via auto-import, ex.: /site/produtos) agora seguem o
// mesmo padrao de tasks. Ver useAdminPageHeaderVisibility.
const pageHeaderVisibility = useAdminPageHeaderVisibility()

const hasEyebrow = computed(() => props.eyebrow.trim().length > 0)
const hasTitle = computed(() => props.title.trim().length > 0)
const hasDescription = computed(() => props.description.trim().length > 0)

const showEyebrow = computed(() => pageHeaderVisibility.showEyebrow.value && hasEyebrow.value)
const showTitle = computed(() => pageHeaderVisibility.showTitle.value && hasTitle.value)
const showDescription = computed(
  () => pageHeaderVisibility.showDescription.value && hasDescription.value,
)
const showHeader = computed(() => showEyebrow.value || showTitle.value || showDescription.value)
</script>

<template>
  <header v-if="showHeader" class="admin-page-header space-y-1.5">
    <p
      v-if="showEyebrow"
      class="admin-page-header__eyebrow text-[11px] font-semibold uppercase tracking-wider text-[rgb(var(--muted))] sm:text-xs"
    >
      {{ eyebrow }}
    </p>

    <h1
      v-if="showTitle"
      class="admin-page-header__title text-xl font-semibold tracking-tight sm:text-2xl lg:text-3xl"
    >
      {{ title }}
    </h1>

    <p
      v-if="showDescription"
      class="admin-page-header__description text-sm leading-6 text-[rgb(var(--muted))] sm:text-[0.95rem]"
    >
      {{ description }}
    </p>
  </header>
</template>
