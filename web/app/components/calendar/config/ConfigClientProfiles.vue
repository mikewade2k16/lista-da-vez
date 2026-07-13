<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useCalendarClientProfiles } from '~/composables/useCalendarClientProfiles'
import { useCalendarRealtime } from '~/composables/useCalendarRealtime'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import {
  defaultClientProfile,
  type CalendarClient,
  type CalendarClientProfile,
  type CalendarClientProfileExtra,
} from '~/utils/calendar'

// Secao Perfil estrategico do cliente (SPEC-F4, contrato C3): select de cliente +
// form dos campos estaveis + textareas do bloco `extra`. Salva por cliente (botao
// proprio + feedback). Dirty guard: trocar de cliente com edicao pendente avisa.
// Badge preenchido/vazio na lista via `filled` do index. O account_id nunca vai no
// body: o back resolve pelo Principal.
const props = defineProps<{ clients: CalendarClient[] }>()

const ui = useUiStore()
const { index, loadingProfile, saving, fetchIndex, loadProfile, saveProfile } =
  useCalendarClientProfiles()

// Campos estaveis do perfil (label + chave). Ordem = contrato C3.
type StableKey =
  | 'segment'
  | 'positioning'
  | 'description'
  | 'history'
  | 'siteUrl'
  | 'instagram'
  | 'address'
  | 'objectives'
  | 'brandVoice'

const STABLE_FIELDS: Array<{ key: StableKey; label: string; area?: boolean }> = [
  { key: 'segment', label: 'Segmento' },
  { key: 'positioning', label: 'Posicionamento' },
  { key: 'siteUrl', label: 'Site' },
  { key: 'instagram', label: 'Instagram' },
  { key: 'address', label: 'Endereco' },
  { key: 'description', label: 'Descricao', area: true },
  { key: 'history', label: 'Historia', area: true },
  { key: 'objectives', label: 'Objetivos', area: true },
  { key: 'brandVoice', label: 'Tom de voz', area: true },
]

const EXTRA_FIELDS: Array<{ key: keyof CalendarClientProfileExtra; label: string }> = [
  { key: 'audience', label: 'Publico-alvo' },
  { key: 'offer', label: 'Oferta' },
  { key: 'pillars', label: 'Pilares de conteudo' },
  { key: 'cadence', label: 'Cadencia de postagem' },
  { key: 'restrictions', label: 'Restricoes' },
  { key: 'performance', label: 'Performance / metas' },
  { key: 'assets', label: 'Assets disponiveis' },
]

const selectedClientId = ref('')
// Rascunho editavel do perfil do cliente selecionado. Re-hidrata do back ao trocar
// de cliente (fonte unica = banco); so preserva enquanto ha edicao pendente.
const draft = ref<CalendarClientProfile>(defaultClientProfile())
const touched = ref(false)

// WAVE 10: tempo real do perfil. O back publica calendar.client_profile_updated no PutClientProfile;
// aqui refazemos o fetch (indice + perfil aberto) SEM reload — para o dono e para quem editar junto.
// O reload do perfil aberto respeita edicao pendente (touched): nunca clobbera o rascunho do usuario.
const auth = useAuthStore()
useCalendarRealtime({
  enabled: computed(() => auth.isAuthenticated),
  onClientProfileUpdated: async (clientId) => {
    await fetchIndex()
    if (clientId && clientId === selectedClientId.value && !touched.value) {
      draft.value = await loadProfile(clientId)
    }
  },
})

const filledSet = computed(() => {
  const set = new Set<string>()
  for (const item of index.value) if (item.filled) set.add(item.clientId)
  return set
})

const selectedClientName = computed(
  () => props.clients.find((client) => client.id === selectedClientId.value)?.name || '',
)

onMounted(() => {
  void fetchIndex()
})

// Troca de cliente com edicao pendente: confirma antes de descartar o rascunho.
// Dirty-guard unico da casa via ui.confirm (padrao do drawer de config — SPEC-F6).
async function selectClient(clientId: string): Promise<void> {
  if (clientId === selectedClientId.value) return
  if (touched.value) {
    const { confirmed } = await ui.confirm({
      title: 'Descartar alterações?',
      message:
        'Há alterações não salvas neste perfil. Trocar de cliente vai descartá-las. Continuar?',
      confirmLabel: 'Descartar',
      cancelLabel: 'Continuar editando',
    })
    if (!confirmed) return
  }
  selectedClientId.value = clientId
  touched.value = false
  draft.value = await loadProfile(clientId)
}

function mark(): void {
  touched.value = true
}

function stableValue(key: StableKey): string {
  return draft.value[key]
}

function setStable(key: StableKey, value: string): void {
  draft.value[key] = value
  mark()
}

function extraValue(key: keyof CalendarClientProfileExtra): string {
  return draft.value.extra[key]
}

function setExtra(key: keyof CalendarClientProfileExtra, value: string): void {
  draft.value.extra[key] = value
  mark()
}

async function save(): Promise<void> {
  if (!selectedClientId.value) return
  const saved = await saveProfile({ ...draft.value, clientId: selectedClientId.value })
  if (saved) {
    draft.value = saved
    touched.value = false
    ui.success('Perfil do cliente salvo.')
  } else {
    ui.error('Nao foi possivel salvar o perfil.')
  }
}

// Se a lista de clientes chega depois de montar e ninguem foi selecionado, nao
// auto-seleciona (o usuario escolhe): mantem o estado vazio orientativo.
watch(
  () => props.clients,
  () => {
    if (selectedClientId.value && !props.clients.some((c) => c.id === selectedClientId.value)) {
      selectedClientId.value = ''
      draft.value = defaultClientProfile()
      touched.value = false
    }
  },
)
</script>

<template>
  <section class="calendar-config__section calendar-config__section--wide">
    <h3 class="calendar-config__section-title">Perfil estratégico do cliente</h3>
    <p class="calendar-config__hint">
      Base de estratégia usada pelo assistente de IA do mês. Salve por cliente.
    </p>

    <div class="calendar-config__block">
      <span class="calendar-config__field-label">Cliente</span>
      <div v-if="clients.length" class="calendar-profile__clients">
        <button
          v-for="client in clients"
          :key="client.id"
          type="button"
          class="calendar-profile__client"
          :class="{ 'is-active': client.id === selectedClientId }"
          @click="selectClient(client.id)"
        >
          <span class="calendar-profile__client-name">{{ client.name }}</span>
          <span
            class="calendar-profile__badge"
            :class="filledSet.has(client.id) ? 'is-filled' : 'is-empty'"
          >
            {{ filledSet.has(client.id) ? 'preenchido' : 'vazio' }}
          </span>
        </button>
      </div>
      <p v-else class="calendar-config__empty">Nenhum cliente ativo.</p>
    </div>

    <p v-if="!selectedClientId" class="calendar-config__empty">
      Selecione um cliente para editar o perfil.
    </p>

    <template v-else>
      <p v-if="loadingProfile" class="calendar-config__hint">Carregando perfil…</p>

      <div class="calendar-config__grid2">
        <label
          v-for="field in STABLE_FIELDS"
          :key="field.key"
          class="calendar-config__field"
          :class="{ 'calendar-config__field--full': field.area }"
        >
          <span class="calendar-config__field-label">{{ field.label }}</span>
          <textarea
            v-if="field.area"
            class="calendar-config__input calendar-config__textarea"
            :value="stableValue(field.key)"
            @input="setStable(field.key, ($event.target as HTMLTextAreaElement).value)"
          ></textarea>
          <input
            v-else
            class="calendar-config__input"
            :value="stableValue(field.key)"
            @input="setStable(field.key, ($event.target as HTMLInputElement).value)"
          />
        </label>
      </div>

      <div class="calendar-config__block">
        <span class="calendar-config__label">Estratégia detalhada</span>
        <div class="calendar-config__grid2">
          <label
            v-for="field in EXTRA_FIELDS"
            :key="field.key"
            class="calendar-config__field calendar-config__field--full"
          >
            <span class="calendar-config__field-label">{{ field.label }}</span>
            <textarea
              class="calendar-config__input calendar-config__textarea"
              :value="extraValue(field.key)"
              @input="setExtra(field.key, ($event.target as HTMLTextAreaElement).value)"
            ></textarea>
          </label>
        </div>
      </div>

      <div class="calendar-config__section-actions">
        <span v-if="touched" class="calendar-config-page__dirty">Alterações não salvas</span>
        <AppPanelButton variant="ghost" :disabled="saving" @click="save">
          Salvar perfil de {{ selectedClientName || 'cliente' }}
        </AppPanelButton>
      </div>
    </template>
  </section>
</template>
