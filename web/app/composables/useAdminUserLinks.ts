import type { Ref } from 'vue'
import type {
  AccountMembershipItem,
  AvailablePermission,
  OrgRole,
  UserOverridesResponse,
  UserPermissionOverride,
} from '~/types/admin-users'
import { getApiErrorMessage } from '~/utils/api-client'

// Contexto compartilhado vindo do useAdminUsersManager. Mantem erro/saving/linha
// unificados com o resto do manager (uma fonte de errorMessage, um savingMap) em
// vez de o auxiliar criar estado proprio e divergir. apiRequest e o mesmo client
// (X-Account-Id da account ativa injetado pelo api-client).
export interface AdminUserLinksContext {
  apiRequest: (path: string, options?: Record<string, unknown>) => Promise<unknown>
  setSaving: (key: string, value: boolean) => void
  errorMessage: Ref<string>
  // Aplica o AdminUserItem retornado (vinculo de org muda o user) na linha da grade.
  applyPatch: (id: string, raw: Record<string, unknown>) => void
  // Normaliza um membership cru do backend (mesmo normalizeMembership do manager).
  normalizeMembership: (raw: Record<string, unknown>) => AccountMembershipItem
}

function normalizeOverride(raw: Record<string, unknown>): UserPermissionOverride {
  const effect = raw.effect === 'deny' ? 'deny' : 'allow'
  const note = String(raw.note ?? '').trim()
  return {
    permissionKey: String(raw.permissionKey ?? ''),
    effect,
    ...(note ? { note } : {}),
  }
}

function normalizeAvailablePermission(raw: Record<string, unknown>): AvailablePermission {
  return {
    key: String(raw.key ?? ''),
    label: String(raw.label ?? ''),
    moduleId: String(raw.moduleId ?? ''),
    scope: String(raw.scope ?? ''),
  }
}

function normalizeOverridesResponse(raw: unknown): UserOverridesResponse {
  const data = (raw ?? {}) as {
    overrides?: Record<string, unknown>[]
    available?: Record<string, unknown>[]
  }
  return {
    overrides: (data.overrides ?? []).map(normalizeOverride),
    available: (data.available ?? []).map(normalizeAvailablePermission),
  }
}

// Vinculos (memberships/organizations) e overrides de modulo/pagina de um usuario.
// Fatiado do useAdminUsersManager para respeitar o limite de ~450 linhas/arquivo;
// re-exposto pelo manager (o consumidor chama via useAdminUsersManager()).
export function useAdminUserLinks(ctx: AdminUserLinksContext) {
  const { apiRequest, setSaving, errorMessage, applyPatch, normalizeMembership } = ctx

  function mapMemberships(raw: unknown): AccountMembershipItem[] {
    const list = (raw as { memberships?: Record<string, unknown>[] }).memberships ?? []
    return list.map(normalizeMembership)
  }

  // Adiciona um vinculo de cliente (account) ao usuario SEM substituir os demais.
  // POST /v1/admin/users/{id}/memberships {accountId, role} -> { memberships }.
  // Destravar usuario sem nenhum cliente. Fora do escopo do ator -> 404; conta
  // inativa/agencia -> 400/422 (a mensagem do backend cai no errorMessage).
  // Devolve a lista atualizada ou null em erro.
  async function addMembership(
    userId: string,
    accountId: string,
    role: string,
  ): Promise<AccountMembershipItem[] | null> {
    const target = String(accountId ?? '').trim()
    if (!userId || !target) return null
    const key = `${userId}:membership:${target}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(`/v1/admin/users/${encodeURIComponent(userId)}/memberships`, {
        method: 'POST',
        body: { accountId: target, role: String(role ?? '').trim() || 'owner' },
      })
      return mapMemberships(resp)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao vincular o cliente ao usuario.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  // Remove o vinculo do usuario com um cliente (desativa membership + tira papeis).
  // DELETE /v1/admin/users/{id}/memberships/{accountId} -> { memberships }.
  async function removeMembership(
    userId: string,
    accountId: string,
  ): Promise<AccountMembershipItem[] | null> {
    const target = String(accountId ?? '').trim()
    if (!userId || !target) return null
    const key = `${userId}:membership:${target}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/admin/users/${encodeURIComponent(userId)}/memberships/${encodeURIComponent(target)}`,
        { method: 'DELETE' },
      )
      return mapMemberships(resp)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao remover o cliente do usuario.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  // Vincula o usuario a uma organizacao (agencia). POST
  // /v1/admin/users/{id}/organizations/{orgId} {orgRole, confirmAgencyWideAccess}.
  // Virar membro da agencia da visao de TODOS os clientes da org -> exige
  // confirm=true, senao o backend responde 422 confirmation_required (a mensagem
  // cai no errorMessage). O AdminUserItem retornado e aplicado na linha da grade
  // (mesmo padrao do moveUserAccount); devolve true em sucesso / false em erro.
  async function linkOrganization(
    userId: string,
    orgId: string,
    orgRole: OrgRole,
    confirmAgencyWideAccess: boolean,
  ): Promise<boolean> {
    const org = String(orgId ?? '').trim()
    if (!userId || !org) return false
    const key = `${userId}:organization:${org}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/admin/users/${encodeURIComponent(userId)}/organizations/${encodeURIComponent(org)}`,
        {
          method: 'POST',
          body: { orgRole, confirmAgencyWideAccess: Boolean(confirmAgencyWideAccess) },
        },
      )
      applyPatch(userId, resp as Record<string, unknown>)
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao vincular a organizacao.')
      return false
    } finally {
      setSaving(key, false)
    }
  }

  // Desvincula o usuario de uma organizacao (agencia). DELETE
  // /v1/admin/users/{id}/organizations/{orgId} -> AdminUserItem atualizado.
  // Ultimo dono da org -> 409 (mensagem do backend no errorMessage).
  async function unlinkOrganization(userId: string, orgId: string): Promise<boolean> {
    const org = String(orgId ?? '').trim()
    if (!userId || !org) return false
    const key = `${userId}:organization:${org}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/admin/users/${encodeURIComponent(userId)}/organizations/${encodeURIComponent(org)}`,
        { method: 'DELETE' },
      )
      applyPatch(userId, resp as Record<string, unknown>)
      return true
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao desvincular a organizacao.')
      return false
    } finally {
      setSaving(key, false)
    }
  }

  // Le os overrides de modulo/pagina do usuario numa account + o catalogo de
  // permissoes elegiveis. GET /v1/admin/users/{id}/accounts/{accountId}/overrides
  // -> { overrides, available }. Fora do escopo / nao-membro -> 404.
  async function getOverrides(
    userId: string,
    accountId: string,
  ): Promise<UserOverridesResponse | null> {
    const target = String(accountId ?? '').trim()
    if (!userId || !target) return null
    const key = `${userId}:overrides:${target}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const resp = await apiRequest(
        `/v1/admin/users/${encodeURIComponent(userId)}/accounts/${encodeURIComponent(target)}/overrides`,
      )
      return normalizeOverridesResponse(resp)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar os modulos do usuario.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  // Substitui (replace) os overrides do usuario na account. PUT mesma rota com
  // { overrides:[{permissionKey, effect, note?}] } -> mesmo shape do GET (re-le do
  // backend). Key fora do catalogo / modulo desabilitado -> 422 (errorMessage).
  async function setOverrides(
    userId: string,
    accountId: string,
    overrides: UserPermissionOverride[],
  ): Promise<UserOverridesResponse | null> {
    const target = String(accountId ?? '').trim()
    if (!userId || !target) return null
    const key = `${userId}:overrides:${target}`
    setSaving(key, true)
    errorMessage.value = ''
    try {
      const body = {
        overrides: (overrides ?? []).map((o) => {
          const note = String(o.note ?? '').trim()
          return {
            permissionKey: String(o.permissionKey ?? '').trim(),
            effect: o.effect === 'deny' ? 'deny' : 'allow',
            ...(note ? { note } : {}),
          }
        }),
      }
      const resp = await apiRequest(
        `/v1/admin/users/${encodeURIComponent(userId)}/accounts/${encodeURIComponent(target)}/overrides`,
        { method: 'PUT', body },
      )
      return normalizeOverridesResponse(resp)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar os modulos do usuario.')
      return null
    } finally {
      setSaving(key, false)
    }
  }

  return {
    addMembership,
    removeMembership,
    linkOrganization,
    unlinkOrganization,
    getOverrides,
    setOverrides,
  }
}
