import type { AvailablePermission } from '~/types/admin-users'
import type {
  RoleTemplateCreateInput,
  RoleTemplateSummary,
  RoleTemplateUpdateInput,
} from '~/types/admin-role-templates'
import { useInlineEditManager } from '~/composables/useInlineEditManager'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Catalogo GLOBAL de papeis-padrao (role templates) via /v1/admin/role-templates.
// SOMENTE platform_admin (gate na UI/menu + backend). Os templates de SISTEMA
// (isSystem=true) sao read-only: o backend bloqueia PATCH/PUT/DELETE e o front nem
// oferece editar/deletar. A fonte de verdade e sempre a resposta do backend —
// re-le (fetchTemplates) apos cada escrita. Mesmo padrao de api-client/saving-map
// do useAdminUsersManager/useAccountRolesManager.

function normalizeTemplate(raw: Record<string, unknown>): RoleTemplateSummary {
  return {
    id: String(raw.id ?? ''),
    moduleId: String(raw.moduleId ?? 'core'),
    label: String(raw.label ?? ''),
    description: String(raw.description ?? ''),
    isSystem: Boolean(raw.isSystem),
    isLocked: Boolean(raw.isLocked),
    sortOrder: Number(raw.sortOrder ?? 0) || 0,
    permissionKeys: Array.isArray(raw.permissionKeys)
      ? (raw.permissionKeys as unknown[]).map((p) => String(p ?? '').trim()).filter(Boolean)
      : [],
  }
}

function normalizeAvailable(raw: Record<string, unknown>): AvailablePermission {
  return {
    key: String(raw.key ?? ''),
    label: String(raw.label ?? ''),
    moduleId: String(raw.moduleId ?? 'outros'),
    scope: String(raw.scope ?? ''),
  }
}

export function useAdminRoleTemplatesManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  // Estado autoritativo: a lista de templates e o catalogo de permissoes elegiveis.
  const templates = ref<RoleTemplateSummary[]>([])
  const available = ref<AvailablePermission[]>([])
  const loading = ref(false)
  const creating = ref(false)
  const errorMessage = ref('')

  // saving granular (chave por template/operacao) — a UI desabilita so o controle
  // em voo, sem travar a tela. Mesma mecanica compartilhada dos outros managers.
  const { savingMap, setSaving } = useInlineEditManager()

  // GET /v1/admin/role-templates -> { templates, available }.
  async function fetchTemplates() {
    loading.value = true
    errorMessage.value = ''
    try {
      const resp = (await apiRequest('/v1/admin/role-templates')) as {
        templates?: Record<string, unknown>[]
        available?: Record<string, unknown>[]
      }
      const rawTemplates = resp.templates ?? []
      const rawAvailable = resp.available ?? []
      templates.value = rawTemplates.map(normalizeTemplate)
      available.value = rawAvailable.map(normalizeAvailable)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar os papeis-padrao.')
    } finally {
      loading.value = false
    }
  }

  // POST /v1/admin/role-templates -> 201. is_system=false, module_id='core' definidos
  // pelo back. true = criado (a tela re-le via fetchTemplates para refletir o back).
  async function createTemplate(input: RoleTemplateCreateInput): Promise<boolean> {
    const id = String(input?.id ?? '').trim()
    const label = String(input?.label ?? '').trim()
    if (!id || !label) {
      errorMessage.value = 'Informe o id (slug) e o nome do papel-padrao.'
      return false
    }
    creating.value = true
    errorMessage.value = ''
    try {
      await apiRequest('/v1/admin/role-templates', {
        method: 'POST',
        body: {
          id,
          label,
          description: String(input?.description ?? '').trim(),
          permissionKeys: Array.isArray(input?.permissionKeys)
            ? input.permissionKeys.map((p) => String(p ?? '').trim()).filter(Boolean)
            : [],
        },
      })
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar o papel-padrao.')
      return false
    } finally {
      creating.value = false
    }
  }

  // PATCH /v1/admin/role-templates/{id} -> 200 (metadados). Bloqueado p/ isSystem no
  // back. true = salvo.
  async function updateTemplate(id: string, input: RoleTemplateUpdateInput): Promise<boolean> {
    const templateId = String(id ?? '').trim()
    if (!templateId) return false
    const key = `template:${templateId}:meta`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const body: Record<string, unknown> = {}
      if (input.label !== undefined) body.label = String(input.label ?? '').trim()
      if (input.description !== undefined) body.description = String(input.description ?? '').trim()
      if (input.sortOrder !== undefined) body.sortOrder = Number(input.sortOrder) || 0
      await apiRequest(`/v1/admin/role-templates/${encodeURIComponent(templateId)}`, {
        method: 'PATCH',
        body,
      })
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar o papel-padrao.')
      return false
    } finally {
      setSaving(key, false)
    }
  }

  // PUT /v1/admin/role-templates/{id}/permissions -> 200 (substitui a matriz).
  // Bloqueado p/ isSystem no back. true = salvo.
  async function updatePermissions(id: string, permissionKeys: string[]): Promise<boolean> {
    const templateId = String(id ?? '').trim()
    if (!templateId) return false
    const key = `template:${templateId}:perms`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/role-templates/${encodeURIComponent(templateId)}/permissions`, {
        method: 'PUT',
        body: {
          permissionKeys: Array.isArray(permissionKeys)
            ? permissionKeys.map((p) => String(p ?? '').trim()).filter(Boolean)
            : [],
        },
      })
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar a matriz de permissoes.')
      return false
    } finally {
      setSaving(key, false)
    }
  }

  // DELETE /v1/admin/role-templates/{id} -> 204. Bloqueado p/ isSystem no back.
  // true = removido.
  async function deleteTemplate(id: string): Promise<boolean> {
    const templateId = String(id ?? '').trim()
    if (!templateId) return false
    const key = `template:${templateId}:delete`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/role-templates/${encodeURIComponent(templateId)}`, {
        method: 'DELETE',
      })
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao remover o papel-padrao.')
      return false
    } finally {
      setSaving(key, false)
    }
  }

  function isSaving(templateId: string, op: 'meta' | 'perms' | 'delete'): boolean {
    const id = String(templateId ?? '').trim()
    return Boolean(id ? savingMap.value[`template:${id}:${op}`] : false)
  }

  return {
    templates,
    available,
    loading,
    creating,
    errorMessage,
    savingMap,
    fetchTemplates,
    createTemplate,
    updateTemplate,
    updatePermissions,
    deleteTemplate,
    isSaving,
  }
}
