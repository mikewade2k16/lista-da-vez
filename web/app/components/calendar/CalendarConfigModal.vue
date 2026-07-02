<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useCalendarStore } from '~/stores/calendar'
import { defaultCalendarConfig } from '~/utils/calendar'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const store = useCalendarStore()
const { config, members } = storeToRefs(store)

const selected = ref<Set<string>>(new Set())
const holidays = ref({ ...defaultCalendarConfig().holidays })
const saving = ref(false)

const holidayOptions = [
  { key: 'brNational', label: 'Feriados nacionais (Brasil)' },
  { key: 'sergipe', label: 'Sergipe (estadual)' },
  { key: 'aracaju', label: 'Aracaju (municipal)' },
  { key: 'luxuryIntl', label: 'Internacionais (marcas de luxo)' },
] as const

watch(
  () => props.open,
  (open) => {
    if (!open) return
    void store.fetchConfig()
    void store.fetchMembers()
  },
  { immediate: true },
)

watch(
  config,
  (cfg) => {
    selected.value = new Set(cfg.responsibleUserIds || [])
    holidays.value = { ...defaultCalendarConfig().holidays, ...cfg.holidays }
  },
  { immediate: true, deep: true },
)

function toggleMember(id: string): void {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

async function save(): Promise<void> {
  saving.value = true
  const ok = await store.saveConfig({
    responsibleUserIds: [...selected.value],
    holidays: { ...holidays.value },
  })
  saving.value = false
  if (ok) emit('close')
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && props.open) emit('close')
}
onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div
    v-if="open"
    class="calendar-form-overlay"
    role="dialog"
    aria-modal="true"
    aria-label="Configurações do calendário"
    @click.self="emit('close')"
  >
    <div class="calendar-form calendar-config">
      <header class="calendar-form__header">
        <strong class="calendar-form__title">Configurações do calendário</strong>
        <button
          type="button"
          class="calendar-form__close"
          aria-label="Fechar"
          @click="emit('close')"
        >
          <UIcon name="i-lucide-x" aria-hidden="true" />
        </button>
      </header>

      <div class="calendar-config__body">
        <section class="calendar-config__section">
          <h3 class="calendar-config__section-title">Responsáveis</h3>
          <p class="calendar-config__hint">
            Quais usuários da conta aparecem na lista de responsáveis. Nenhum marcado = todos.
          </p>
          <div v-if="members.length" class="calendar-config__members">
            <label v-for="member in members" :key="member.id" class="calendar-config__check">
              <input
                type="checkbox"
                :checked="selected.has(member.id)"
                @change="toggleMember(member.id)"
              />
              <span>{{ member.name }}</span>
            </label>
          </div>
          <p v-else class="calendar-config__empty">Nenhum usuário na conta.</p>
        </section>

        <section class="calendar-config__section">
          <h3 class="calendar-config__section-title">Feriados & datas comemorativas</h3>
          <p class="calendar-config__hint">Quais conjuntos aparecem marcados no calendário.</p>
          <label v-for="opt in holidayOptions" :key="opt.key" class="calendar-config__check">
            <input v-model="holidays[opt.key]" type="checkbox" />
            <span>{{ opt.label }}</span>
          </label>
        </section>
      </div>

      <footer class="calendar-form__footer">
        <span class="calendar-form__footer-spacer"></span>
        <AppPanelButton variant="ghost" @click="emit('close')">Cancelar</AppPanelButton>
        <AppPanelButton variant="primary" :disabled="saving" @click="save">Salvar</AppPanelButton>
      </footer>
    </div>
  </div>
</template>
