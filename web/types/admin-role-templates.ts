import type { AvailablePermission } from '~/types/admin-users'

// Tipos da area admin de "papeis-padrao" (role templates) — catalogo GLOBAL de
// templates que as contas novas clonam. Bate na API real /v1/admin/role-templates.
// Reusa AvailablePermission (mesmo shape de chave/label/modulo) do catalogo de
// usuarios: o `available` do GET e o universo de checkboxes da matriz.

// Resumo de um template, espelhando 1:1 a resposta do backend (camelCase).
// isSystem=true => template de sistema: read-only no front (e bloqueado no back).
// isLocked acompanha o backend (template protegido); tratado como sinonimo de
// read-only junto com isSystem por seguranca (defesa em profundidade no front).
export interface RoleTemplateSummary {
  id: string
  moduleId: string
  label: string
  description: string
  isSystem: boolean
  isLocked: boolean
  sortOrder: number
  // Permissionkeys marcadas no template (a matriz do template). Fonte de verdade
  // = resposta do backend; re-hidrata apos salvar.
  permissionKeys: string[]
}

// Resposta de GET /v1/admin/role-templates: a lista de templates + o catalogo de
// permissoes elegiveis (`available`), agrupado por moduleId na UI.
export interface RoleTemplatesResponse {
  templates: RoleTemplateSummary[]
  available: AvailablePermission[]
}

// Body de POST /v1/admin/role-templates. O backend define is_system=false e
// module_id='core'; o front so envia id (slug), label, descricao e a matriz.
export interface RoleTemplateCreateInput {
  id: string
  label: string
  description: string
  permissionKeys: string[]
}

// Body de PATCH /v1/admin/role-templates/{id} (metadados; matriz vai no PUT
// separado). Todos opcionais — so se envia o que mudou.
export interface RoleTemplateUpdateInput {
  label?: string
  description?: string
  sortOrder?: number
}
