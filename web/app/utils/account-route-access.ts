export interface ModulePathGuard {
  prefix: string
  moduleId: string
}

// Politicas novas mantidas em um modulo puro para que o contrato menu x URL
// direta tenha cobertura sem carregar o runtime Nuxt.
export const EDITOR_MODULE_PATH_GUARD: ModulePathGuard = {
  prefix: '/editor',
  moduleId: 'tools',
}

export const AGENCY_ONLY_PATHS = [
  '/manage/users',
  '/manage/organizations',
  '/manage/role-templates',
  '/manage/storage',
  '/manage/clientes-web',
  '/themes',
  '/manage/auditoria',
  '/manage/integracoes',
  '/roadmap',
] as const

export function pathMatchesPrefix(path: string, prefix: string): boolean {
  return path === prefix || path.startsWith(`${prefix}/`)
}

export function findEditorModulePathGuard(path: string): ModulePathGuard | undefined {
  return pathMatchesPrefix(path, EDITOR_MODULE_PATH_GUARD.prefix)
    ? EDITOR_MODULE_PATH_GUARD
    : undefined
}

export function isAgencyOnlyPath(path: string): boolean {
  return AGENCY_ONLY_PATHS.some((prefix) => pathMatchesPrefix(path, prefix))
}
