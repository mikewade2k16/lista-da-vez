import { computed } from 'vue'
import { defineStore } from 'pinia'
import { useCoreAccountStore } from '../../layers/core/stores/account'

// COSTURA (F1) — adaptador temporario do modulo omnichannel portado do legado.
// Alvo de remocao: F14 (docs/LEGADO.md).
//
// No legado este store era a "simulacao de sessao": o admin da plataforma escolhia
// um cliente por um id NUMERICO e o BFF passava a responder como aquele cliente,
// via headers `x-client-id` / `x-tenant-id`.
//
// No Omni isso NAO se traduz, e a diferenca e proposital:
//
// 1. Conta ativa no Omni e UUID (core.accounts.id), nao numero. A API deste store
//    e numerica (`clientOptions[].value: number`, `option.value === effectiveClientId`),
//    entao nao ha como injetar as contas do Omni aqui sem editar os arquivos
//    verbatim — e o verbatim e o contrato da F1.
// 2. Quem troca de conta no Omni e o switcher do shell (useCoreAccountStore). Ter
//    um segundo switcher dentro do modulo criaria duas fontes de conta — o bug
//    registrado em project_account_source_divergence.
// 3. O escopo de conta viaja no X-Account-Id, injetado pelo provider global
//    (plugins/account-id-bridge.client.ts). Este store NAO emite header de conta.
//
// Resultado: a simulacao numerica fica DESLIGADA (`canSimulate = false`), o que
// esconde o switcher interno do modulo, e o switcher do shell segue sendo o unico.
// Nao e remover funcionalidade (principio 3): as duas sao mutuamente exclusivas —
// e a rota do legado que sustentava a troca (`POST /session/context`) nao existe
// no Go e nao esta prevista em nenhuma fase do plano.
//
// O que continua REAL aqui: `hasModule`, que le os modulos da conta ativa.

export interface SessionSimulationClientOption {
  label: string
  value: number
  coreTenantId?: string
  moduleCodes?: string[]
}

function normalizeModuleCode(value: unknown) {
  return String(value ?? '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_')
}

// O modulo portado pergunta pelo codigo do legado ("atendimento"); no Omni o
// modulo se chama 'omnichannel' (core.modules). Este mapa e a traducao, e o
// unico lugar onde ela existe.
const OMNI_MODULE_BY_LEGACY_CODE: Record<string, string> = {
  atendimento: 'omnichannel',
}

export const useSessionSimulationStore = defineStore('sessionSimulation', () => {
  const account = useCoreAccountStore()

  // Simulacao numerica do legado nao existe no Omni (ver cabecalho).
  const canSimulate = computed(() => false)
  const clientId = computed(() => 0)
  const effectiveClientId = computed(() => 0)
  const clientOptions = computed<SessionSimulationClientOption[]>(() => [])

  // Vazio de proposito: o X-Account-Id e injetado pelo provider global. Emitir
  // header de conta aqui seria a segunda fonte de escopo.
  const requestHeaders = computed<Record<string, string>>(() => ({}))

  function hasModule(moduleCode: unknown) {
    const normalized = normalizeModuleCode(moduleCode)
    if (!normalized) {
      return false
    }

    const omniModuleId = OMNI_MODULE_BY_LEGACY_CODE[normalized] ?? normalized
    return account.enabledModules.includes(omniModuleId)
  }

  // Inalcancavel pela UI enquanto canSimulate for false (o switcher interno nao
  // renderiza). Mantido porque o arquivo verbatim referencia o metodo.
  function setClientId(_next: unknown) {
    // No-op: trocar de conta e responsabilidade do switcher do shell.
  }

  return {
    canSimulate,
    clientId,
    effectiveClientId,
    clientOptions,
    requestHeaders,
    hasModule,
    setClientId,
  }
})
