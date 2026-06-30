<script setup lang="ts">
import type { RoleTemplateCreateInput, RoleTemplateSummary } from '~/types/admin-role-templates'
import { useAdminRoleTemplatesManager } from '~/composables/useAdminRoleTemplatesManager'
import AdminRoleTemplateCreateForm from '~/components/admin/role-templates/AdminRoleTemplateCreateForm.vue'
import AdminRoleTemplateListItem from '~/components/admin/role-templates/AdminRoleTemplateListItem.vue'

// Host da area de papeis-padrao (role templates). Concentra o ESTADO (selecao,
// abertura do form) e orquestra o manager; a apresentacao da linha/form/matriz foi
// fatiada. SOMENTE platform_admin: o gate de rota/menu ja impede o acesso, mas
// reforcamos aqui (fail-closed) — quem nao for platform_admin ve so um aviso e
// nenhum fetch dispara. Fonte de verdade = resposta do backend (re-le apos escrita).

const manager = useAdminRoleTemplatesManager()
const {
  templates,
  available,
  loading,
  creating,
  errorMessage,
  fetchTemplates,
  createTemplate,
  updateTemplate,
  updatePermissions,
  deleteTemplate,
  isSaving,
} = manager

const auth = useAuthStore()
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')

// Template aberto para edicao (toggle: clicar no mesmo fecha). Custom e sistema
// podem abrir; o de sistema renderiza so leitura.
const expandedId = ref('')
const showCreate = ref(false)

// Separa sistema (read-only) de custom (editavel) para listar em blocos distintos.
const systemTemplates = computed(() => templates.value.filter((t) => t.isSystem || t.isLocked))
const customTemplates = computed(() => templates.value.filter((t) => !t.isSystem && !t.isLocked))

function toggleExpanded(id: string) {
  expandedId.value = expandedId.value === id ? '' : id
}

async function onCreate(input: RoleTemplateCreateInput) {
  const ok = await createTemplate(input)
  if (!ok) return
  showCreate.value = false
  await fetchTemplates()
  // Abre direto o recem-criado para o usuario continuar montando a matriz.
  expandedId.value = input.id
}

async function onSaveMeta(
  template: RoleTemplateSummary,
  payload: { label: string; description: string },
) {
  const ok = await updateTemplate(template.id, payload)
  if (!ok) return
  await fetchTemplates()
}

async function onSavePerms(template: RoleTemplateSummary, permissionKeys: string[]) {
  const ok = await updatePermissions(template.id, permissionKeys)
  if (!ok) return
  await fetchTemplates()
}

async function onRemove(template: RoleTemplateSummary) {
  if (template.isSystem || template.isLocked) return
  if (import.meta.client) {
    const confirmed = window.confirm(
      `Remover o papel-padrao "${template.label}"? Esta acao nao pode ser desfeita.`,
    )
    if (!confirmed) return
  }
  const ok = await deleteTemplate(template.id)
  if (!ok) return
  if (expandedId.value === template.id) expandedId.value = ''
  await fetchTemplates()
}

onMounted(() => {
  // Gate por papel ANTES do fetch (espelha o back: rota exige platform_admin).
  if (!isPlatformAdmin.value) return
  void fetchTemplates()
})
</script>

<template>
  <section class="role-templates-workspace flex h-full min-h-0 flex-col gap-4 overflow-hidden">
    <AdminPageHeader
      eyebrow="Admin"
      title="Papeis-padrao"
      description="Catalogo global de papeis-padrao (templates) que as contas novas clonam. So platform_admin. Templates de sistema sao somente leitura; os customizados podem ser criados, editados e removidos."
    />

    <div v-if="!isPlatformAdmin" class="role-templates-workspace__denied">
      Esta area e exclusiva para administradores da plataforma.
    </div>

    <template v-else>
      <div class="role-templates-workspace__toolbar">
        <button
          class="role-templates-workspace__new-btn"
          type="button"
          @click="showCreate = !showCreate"
        >
          {{ showCreate ? 'Cancelar' : 'Novo papel-padrao' }}
        </button>
      </div>

      <UAlert
        v-if="errorMessage"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        title="Erro"
        :description="errorMessage"
      />

      <AdminRoleTemplateCreateForm
        v-if="showCreate"
        :creating="creating"
        :available="available"
        @submit="onCreate"
      />

      <div class="role-templates-workspace__scroll flex-1 min-h-0 overflow-y-auto">
        <p v-if="loading" class="role-templates-workspace__state">Carregando papeis-padrao...</p>
        <p v-else-if="!templates.length" class="role-templates-workspace__state">
          Nenhum papel-padrao cadastrado ainda. Crie o primeiro acima.
        </p>

        <template v-else>
          <section v-if="customTemplates.length" class="role-templates-workspace__block">
            <h4 class="role-templates-workspace__block-title">Customizados</h4>
            <ul class="role-templates-workspace__list">
              <AdminRoleTemplateListItem
                v-for="template in customTemplates"
                :key="template.id"
                :template="template"
                :available="available"
                :expanded="expandedId === template.id"
                :saving-meta="isSaving(template.id, 'meta')"
                :saving-perms="isSaving(template.id, 'perms')"
                :deleting="isSaving(template.id, 'delete')"
                @toggle="toggleExpanded(template.id)"
                @save-meta="onSaveMeta(template, $event)"
                @save-perms="onSavePerms(template, $event)"
                @remove="onRemove(template)"
              />
            </ul>
          </section>

          <section v-if="systemTemplates.length" class="role-templates-workspace__block">
            <h4 class="role-templates-workspace__block-title">Sistema (somente leitura)</h4>
            <ul class="role-templates-workspace__list">
              <AdminRoleTemplateListItem
                v-for="template in systemTemplates"
                :key="template.id"
                :template="template"
                :available="available"
                :expanded="expandedId === template.id"
                :saving-meta="false"
                :saving-perms="false"
                :deleting="false"
                @toggle="toggleExpanded(template.id)"
              />
            </ul>
          </section>
        </template>
      </div>
    </template>
  </section>
</template>

<style scoped>
.role-templates-workspace__denied,
.role-templates-workspace__state {
  margin: 0;
  font-size: 0.85rem;
  color: rgb(var(--muted));
}

.role-templates-workspace__toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

.role-templates-workspace__new-btn {
  min-height: 2.1rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface));
  color: rgb(var(--text));
}

.role-templates-workspace__block {
  display: grid;
  gap: 0.6rem;
  margin-bottom: 1rem;
}

.role-templates-workspace__block-title {
  margin: 0;
  font-size: 0.76rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: rgb(var(--muted));
}

.role-templates-workspace__list {
  display: grid;
  gap: 0.6rem;
  margin: 0;
  padding: 0;
  list-style: none;
}
</style>
