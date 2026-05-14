<script setup lang="ts">
interface AdminPageHeaderProps {
  eyebrow?: string
  title?: string
  description?: string
}

const props = withDefaults(defineProps<AdminPageHeaderProps>(), {
  eyebrow: '',
  title: '',
  description: ''
})

const hasEyebrow = computed(() => props.eyebrow.trim().length > 0)
const hasTitle = computed(() => props.title.trim().length > 0)
const hasDescription = computed(() => props.description.trim().length > 0)

const showEyebrow = computed(() => hasEyebrow.value)
const showTitle = computed(() => hasTitle.value)
const showDescription = computed(() => hasDescription.value)
const showHeader = computed(() => showEyebrow.value || showTitle.value || showDescription.value)
</script>

<template>
  <header v-if="showHeader" class="admin-page-header space-y-1.5">
    <p v-if="showEyebrow" class="admin-page-header__eyebrow text-[11px] font-semibold uppercase tracking-wider text-[rgb(var(--muted))] sm:text-xs">
      {{ eyebrow }}
    </p>

    <h1 v-if="showTitle" class="admin-page-header__title text-xl font-semibold tracking-tight sm:text-2xl lg:text-3xl">
      {{ title }}
    </h1>

    <p v-if="showDescription" class="admin-page-header__description text-sm leading-6 text-[rgb(var(--muted))] sm:text-[0.95rem]">
      {{ description }}
    </p>
  </header>
</template>
