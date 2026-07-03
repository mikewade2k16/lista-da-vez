<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import ConfigResponsibles from '~/components/calendar/config/ConfigResponsibles.vue'
import ConfigHolidays from '~/components/calendar/config/ConfigHolidays.vue'
import ConfigAppearance from '~/components/calendar/config/ConfigAppearance.vue'
import ConfigAi from '~/components/calendar/config/ConfigAi.vue'
import ConfigClientProfiles from '~/components/calendar/config/ConfigClientProfiles.vue'
import ConfigMediaLimits from '~/components/calendar/config/ConfigMediaLimits.vue'
import { useCalendarStore } from '~/stores/calendar'
import { useUiStore } from '~/stores/ui'
import { defaultCalendarConfig, type CalendarConfig } from '~/utils/calendar'

definePageMeta({
  layout: 'dashboard',
  // Tela global (sem modulo): mesmo criterio da /calendario (evita o gate de
  // workspace do auth.global.ts). Preview front; o gating real entra no back.
  workspaceId: '',
})

const store = useCalendarStore()
const ui = useUiStore()
const { config, members, clients } = storeToRefs(store)

// Rascunho editavel: re-hidrata da resposta do back sempre que a config chega
// (fonte unica = banco). So se preserva enquanto ha edicao pendente (touched).
const draft = ref<CalendarConfig>(defaultCalendarConfig())
const touched = ref(false)
const saving = ref(false)

watch(
  config,
  (cfg) => {
    if (touched.value) return
    draft.value = JSON.parse(JSON.stringify(cfg)) as CalendarConfig
  },
  { immediate: true, deep: true },
)

function mark(): void {
  touched.value = true
}

onMounted(() => {
  store.init()
  void store.fetchConfig()
  void store.fetchMembers()
})

async function save(): Promise<void> {
  saving.value = true
  const ok = await store.saveConfig(draft.value)
  saving.value = false
  if (ok) {
    touched.value = false
    ui.success('Configurações salvas.')
  } else {
    ui.error('Não foi possível salvar as configurações.')
  }
}

function goBack(): void {
  void navigateTo('/calendario')
}
</script>

<template>
  <div class="calendar-config-page">
    <header class="calendar-config-page__head">
      <button
        type="button"
        class="calendar-config-page__back"
        aria-label="Voltar para o calendário"
        @click="goBack"
      >
        <UIcon name="i-lucide-arrow-left" aria-hidden="true" />
        Voltar
      </button>
      <AdminPageHeader
        eyebrow="Calendário"
        title="Configurações"
        description="Responsáveis, feriados, aparência, assistente de IA e limites de mídia."
      />
    </header>

    <div class="calendar-config-page__body">
      <ConfigResponsibles
        :model-value="draft.responsibleUserIds"
        :members="members"
        @update:model-value="
          (v) => {
            draft.responsibleUserIds = v
            mark()
          }
        "
      />

      <ConfigHolidays
        :model-value="draft.holidays"
        @update:model-value="
          (v) => {
            draft.holidays = v
            mark()
          }
        "
      />

      <ConfigAppearance
        :week-starts-on="draft.weekStartsOn"
        :client-colors="draft.clientColors"
        :type-colors="draft.typeColors"
        :white-label="draft.whiteLabel"
        :clients="clients"
        @update:week-starts-on="
          (v) => {
            draft.weekStartsOn = v
            mark()
          }
        "
        @update:client-colors="
          (v) => {
            draft.clientColors = v
            mark()
          }
        "
        @update:type-colors="
          (v) => {
            draft.typeColors = v
            mark()
          }
        "
        @update:white-label="
          (v) => {
            draft.whiteLabel = v
            mark()
          }
        "
      />

      <ConfigAi
        :model-value="draft.ai"
        @update:model-value="
          (v) => {
            draft.ai = v
            mark()
          }
        "
      />

      <!-- Perfil estrategico por cliente (SPEC-F4): salva por cliente (endpoint
           proprio), independente do botao "Salvar configuracoes" global. -->
      <ConfigClientProfiles :clients="clients" />

      <!-- Limites de midia sao GLOBAIS (endpoint proprio) e salvam por conta prop. -->
      <ConfigMediaLimits />
    </div>

    <footer class="calendar-config-page__footer">
      <span v-if="touched" class="calendar-config-page__dirty">Alterações não salvas</span>
      <AppPanelButton variant="primary" :disabled="saving" @click="save">
        Salvar configurações
      </AppPanelButton>
    </footer>
  </div>
</template>
