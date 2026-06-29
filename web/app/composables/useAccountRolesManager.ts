import type { RoleCreateInput, RoleDetail, RoleSummary, RoleUpdateInput } from '~/types/admin-users'
import { useInlineEditManager } from '~/composables/useInlineEditManager'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Papeis customizados por cliente (core.roles) via /v1/accounts/{accountId}/roles*.
// O MESMO endpoint serve o painel do cliente e o admin global — o backend escopa
// pelo accountId do PATH + autorizacao do ator (core.roles.view/manage). O admin
// pode gerenciar papeis de uma account DIFERENTE da ativa; por isso enviamos
// X-Account-Id = accountId explicito em cada chamada (padrao das rotas
// account-scoped no projeto, ex.: layers/tasks). O api-client so injeta o header
// global da account ativa quando nao ha um manual (`if (accountId && !headers[...])`),
// entao o nosso valor explicito vence e o escopo casa com o recurso do path.

function normalizeRole(raw: Record<string, unknown>): RoleSummary {
  const description = String(raw.description ?? '').trim()
  return {
    id: String(raw.id ?? ''),
    code: String(raw.code ?? ''),
    label: String(raw.label ?? ''),
    isLocked: Boolean(raw.isLocked),
    isDefault: Boolean(raw.isDefault),
    ...(description ? { description } : {}),
  }
}

export function useAccountRolesManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const errorMessage = ref('')
  // Reusa o padrao savingMap/setSaving do projeto (chave granular por operacao)
  // para a UI desabilitar so o controle em voo, sem travar a tela inteira. Vem do
  // useInlineEditManager compartilhado; este manager nao tem debounce nem grade,
  // entao usa so o subconjunto savingMap/setSaving (sem rowIsSaving/schedulePatch).
  const { savingMap, setSaving } = useInlineEditManager()

  // Header de escopo: account-id explicito da chamada (alvo no path). Vence o
  // header global da account ativa injetado pelo api-client.
  function accountHeaders(accountId: string): Record<string, string> {
    const id = String(accountId ?? '').trim()
    return id ? { 'X-Account-Id': id } : {}
  }

  // Lista os papeis da account. GET /v1/accounts/{accountId}/roles -> { roles }.
  async function listRoles(accountId: string): Promise<RoleSummary[]> {
    const id = String(accountId ?? '').trim()
    if (!id) return []
    errorMessage.value = ''
    try {
      const resp = await apiRequest(`/v1/accounts/${encodeURIComponent(id)}/roles`, {
        headers: accountHeaders(id),
      })
      const raw = (resp as { roles?: Record<string, unknown>[] }).roles ?? []
      return raw.map(normalizeRole)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar os papeis.')
      return []
    }
  }

  // Le os papeis (core.roles) atribuidos a UM membro naquela account, para o
  // painel exibir o estado atual antes de editar. GET
  // /v1/accounts/{accountId}/members/{userId}/roles -> { roles }. Em erro retorna
  // [] (simetria com listRoles, a outra leitura de papeis): o painel itera sem
  // checagem extra de null. Fora do escopo / nao-membro -> resposta vazia/erro.
  async function getUserRoles(accountId: string, userId: string): Promise<RoleSummary[]> {
    const id = String(accountId ?? '').trim()
    const user = String(userId ?? '').trim()
    if (!id || !user) return []
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/accounts/${encodeURIComponent(id)}/members/${encodeURIComponent(user)}/roles`,
        { headers: accountHeaders(id) },
      )
      const raw = (resp as { roles?: Record<string, unknown>[] }).roles ?? []
      return raw.map(normalizeRole)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar os papeis do usuario.')
      return []
    }
  }

  // Detalhe de um papel + matriz de permissoes marcadas.
  // GET /v1/accounts/{accountId}/roles/{roleId} -> { role, permissions }.
  async function getRole(accountId: string, roleId: string): Promise<RoleDetail | null> {
    const id = String(accountId ?? '').trim()
    const role = String(roleId ?? '').trim()
    if (!id || !role) return null
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/accounts/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`,
        { headers: accountHeaders(id) },
      )
      const data = (resp ?? {}) as { role?: Record<string, unknown>; permissions?: unknown }
      return {
        role: normalizeRole(data.role ?? {}),
        permissions: Array.isArray(data.permissions)
          ? (data.permissions as unknown[]).map((p) => String(p ?? ''))
          : [],
      }
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar o papel.')
      return null
    }
  }

  // Cria um papel customizado. POST /v1/accounts/{accountId}/roles {code,label,
  // description} -> { role }. Code duplicado na account -> 409 (errorMessage).
  async function createRole(
    accountId: string,
    input: RoleCreateInput,
  ): Promise<RoleSummary | null> {
    const id = String(accountId ?? '').trim()
    const code = String(input?.code ?? '').trim()
    const label = String(input?.label ?? '').trim()
    if (!id || !code || !label) {
      errorMessage.value = 'Informe code e nome do papel.'
      return null
    }
    const key = `${id}:role:create`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(`/v1/accounts/${encodeURIComponent(id)}/roles`, {
        method: 'POST',
        headers: accountHeaders(id),
        body: { code, label, description: String(input?.description ?? '').trim() },
      })
      const raw = (resp as { role?: Record<string, unknown> }).role ?? {}
      return normalizeRole(raw)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar o papel.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  // Edita label/descricao e SUBSTITUI a matriz de permissoes do papel.
  // PATCH /v1/accounts/{accountId}/roles/{roleId} {label,description,permissions}
  // -> { role }. Devolve o resumo atualizado ou null em erro.
  async function updateRole(
    accountId: string,
    roleId: string,
    input: RoleUpdateInput,
  ): Promise<RoleSummary | null> {
    const id = String(accountId ?? '').trim()
    const role = String(roleId ?? '').trim()
    if (!id || !role) return null
    const key = `${id}:role:${role}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/accounts/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`,
        {
          method: 'PATCH',
          headers: accountHeaders(id),
          body: {
            label: String(input?.label ?? '').trim(),
            description: String(input?.description ?? '').trim(),
            permissions: Array.isArray(input?.permissions)
              ? input.permissions.map((p) => String(p ?? '').trim()).filter(Boolean)
              : [],
          },
        },
      )
      const raw = (resp as { role?: Record<string, unknown> }).role ?? {}
      return normalizeRole(raw)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar o papel.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  // Remove um papel customizado. DELETE /v1/accounts/{accountId}/roles/{roleId}
  // -> 204. Papel bloqueado (isLocked) -> 422 (errorMessage). true = removido.
  async function deleteRole(accountId: string, roleId: string): Promise<boolean> {
    const id = String(accountId ?? '').trim()
    const role = String(roleId ?? '').trim()
    if (!id || !role) return false
    const key = `${id}:role:${role}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/accounts/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`, {
        method: 'DELETE',
        headers: accountHeaders(id),
      })
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao remover o papel.')
      return false
    } finally {
      setSaving(key, false)
    }
  }

  // Atribui em LOTE os papeis de um membro (replace transacional).
  // PUT /v1/accounts/{accountId}/members/{userId}/roles {roleIds} -> { roles }.
  // Devolve a lista final de papeis do usuario ou null em erro.
  async function setUserRoles(
    accountId: string,
    userId: string,
    roleIds: string[],
  ): Promise<RoleSummary[] | null> {
    const id = String(accountId ?? '').trim()
    const user = String(userId ?? '').trim()
    if (!id || !user) return null
    const key = `${id}:member:${user}:roles`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/accounts/${encodeURIComponent(id)}/members/${encodeURIComponent(user)}/roles`,
        {
          method: 'PUT',
          headers: accountHeaders(id),
          body: {
            roleIds: Array.isArray(roleIds)
              ? roleIds.map((r) => String(r ?? '').trim()).filter(Boolean)
              : [],
          },
        },
      )
      const raw = (resp as { roles?: Record<string, unknown>[] }).roles ?? []
      return raw.map(normalizeRole)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao atribuir os papeis ao usuario.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  return {
    errorMessage,
    savingMap,
    listRoles,
    getUserRoles,
    getRole,
    createRole,
    updateRole,
    deleteRole,
    setUserRoles,
  }
}
