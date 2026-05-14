<script setup lang="ts">
import AdminPageHeader from '../../core/components/admin/AdminPageHeader.vue'
import CoreSkeleton from '../../core/components/CoreSkeleton.vue'
import { useCoreLoading } from '../../core/composables/useCoreLoading'
import { useTasksWorkspace } from '../composables/useTasksWorkspace'

definePageMeta({
  layout: 'dashboard',
  workspaceId: '',
  pageLabel: 'Editor'
})

const tasksWorkspace = useTasksWorkspace()
const pageLoading = useCoreLoading()
const pageBootstrapping = ref(true)
const value = ref(`<h1>Documento OmniEditor</h1>
<p>Use este espaco para testar textos longos, imagens, HTML, emojis, mencoes, slash commands, toolbar flutuante e drag por bloco.</p>
<p>Digite <strong>/</strong> para comandos, <strong>@</strong> para pessoas, <strong>#</strong> para clientes/tasks e <strong>:</strong> para emojis.</p>`)

const people = computed(() => {
  const project = tasksWorkspace.projects.value.find(item => item.id === tasksWorkspace.activeProjectId.value)
  return project?.responsibles || []
})
const clients = computed(() => ['Dr Antonio', 'Perola Jardins', 'Crow Visuals', 'UNO'])
const tasks = computed(() => tasksWorkspace.tasks.value.map(task => task.title).filter(Boolean))

onMounted(async () => {
  try {
    await pageLoading.withLoading('Carregando editor...', async () => {
      await tasksWorkspace.initialize()
      await nextTick()
      if (import.meta.client) {
        await new Promise<void>((resolve) => {
          requestAnimationFrame(() => resolve())
        })
      }
    })
  } finally {
    pageBootstrapping.value = false
  }
})
</script>

<template>
  <section class="editor-page">
    <AdminPageHeader
      eyebrow="Editor"
      title="OmniEditor"
      description="Documento dedicado para validar o componente reutilizavel antes de espalhar pelos modulos."
      :actions="[]"
    />

    <div class="editor-page__surface">
      <div v-if="pageBootstrapping" class="grid h-full gap-4 p-5">
        <CoreSkeleton variant="block" width="220px" height="18px" />
        <CoreSkeleton variant="block" height="56px" />
        <CoreSkeleton variant="block" height="220px" />
      </div>

      <OmniEditor
        v-else
        v-model="value"
        content-type="html"
        :people="people"
        :clients="clients"
        :tasks="tasks"
        min-height="calc(100vh - 250px)"
        max-height="calc(100vh - 210px)"
      />
    </div>
  </section>
</template>

<style scoped>
.editor-page {
  width: 100%;
  min-height: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 1rem;
}

.editor-page__surface {
  min-height: 0;
  flex: 1;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  overflow: hidden;
}
</style>
