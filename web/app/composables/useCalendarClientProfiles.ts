import { ref } from 'vue'

import { useAuthStore } from '~/stores/auth'
import * as calendarApi from '~/domain/calendar/calendar-api'
import { createApiRequest } from '~/utils/api-client'
import {
  defaultClientProfile,
  type CalendarClientProfile,
  type CalendarClientProfileIndexItem,
} from '~/utils/calendar'

// useCalendarClientProfiles concentra o I/O do PERFIL ESTRATEGICO do cliente
// (contrato C3, SPEC-F4): index de quem esta preenchido + carregar/salvar por
// cliente. Fica fora do store por ser usado so na pagina de config e para nao
// passar de 450 linhas. O account_id nunca trafega no body: o back resolve pelo
// Principal (accountScope); aqui so o clientId (conta-cliente) viaja.
export function useCalendarClientProfiles() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const index = ref<CalendarClientProfileIndexItem[]>([])
  const loadingIndex = ref(false)
  const loadingProfile = ref(false)
  const saving = ref(false)

  async function withSession<T>(run: () => Promise<T>, fallback: T): Promise<T> {
    await auth.ensureSession()
    if (!auth.isAuthenticated) return fallback
    try {
      return await run()
    } catch {
      return fallback
    }
  }

  async function fetchIndex(): Promise<void> {
    loadingIndex.value = true
    index.value = await withSession(
      () => calendarApi.fetchClientProfilesIndex(apiRequest),
      index.value,
    )
    loadingIndex.value = false
  }

  // Carrega o perfil de um cliente; ausente = defaults (contrato C3, 200 vazio).
  async function loadProfile(clientId: string): Promise<CalendarClientProfile> {
    if (!clientId) return defaultClientProfile()
    loadingProfile.value = true
    const profile = await withSession(
      () => calendarApi.fetchClientProfile(apiRequest, clientId),
      defaultClientProfile(clientId),
    )
    loadingProfile.value = false
    return profile
  }

  // Salva (upsert full-replace). Devolve o perfil normalizado do back ou null em
  // erro, e atualiza o index local (filled/updatedAt) sem refetch.
  async function saveProfile(
    profile: CalendarClientProfile,
  ): Promise<CalendarClientProfile | null> {
    saving.value = true
    const saved = await withSession(() => calendarApi.putClientProfile(apiRequest, profile), null)
    saving.value = false
    if (saved) applyToIndex(saved)
    return saved
  }

  // Atualiza (ou insere) a entrada do index a partir do perfil salvo, sem refetch.
  function applyToIndex(profile: CalendarClientProfile): void {
    const entry: CalendarClientProfileIndexItem = {
      clientId: profile.clientId,
      filled: profileHasContent(profile),
      updatedAt: profile.updatedAt,
    }
    const next = index.value.filter((item) => item.clientId !== profile.clientId)
    next.push(entry)
    index.value = next
  }

  return {
    index,
    loadingIndex,
    loadingProfile,
    saving,
    fetchIndex,
    loadProfile,
    saveProfile,
  }
}

// filled = algum campo estavel nao-vazio (contrato C3). Espelha a regra do back
// para o badge da lista refletir na hora, sem depender de refetch do index.
export function profileHasContent(profile: CalendarClientProfile): boolean {
  const stable = [
    profile.segment,
    profile.positioning,
    profile.description,
    profile.history,
    profile.siteUrl,
    profile.instagram,
    profile.address,
    profile.objectives,
    profile.brandVoice,
  ]
  return stable.some((value) => value.trim() !== '')
}
