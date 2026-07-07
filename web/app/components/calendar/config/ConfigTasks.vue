<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
// Store de Tasks vive em outra layer; import cross-layer (precedente ja existe no
// app, ex.: OmniDataTable). Boards sao carregados lazy so ao abrir esta aba.
import { useTasksStore } from '../../../../layers/tasks/stores/tasks'
import { EVENT_TYPE_META, STATUS_META } from '~/utils/calendar'
import type { CalendarTasksConfig } from '~/utils/calendar'

// Aba Integracoes (SPEC-F6, contrato C6 + WAVE 5): escolhe o board + a coluna de destino ao
// criar uma task a partir de um evento; liga o espelho task->evento e mapeia status<->coluna
// (E5). Vazio = integracao desligada. O modelo faz parte do draft compartilhado do
// CalendarConfig (salva pelo footer).
const props = defineProps<{ modelValue: CalendarTasksConfig }>()
const emit = defineEmits<{ 'update:modelValue': [value: CalendarTasksConfig] }>()

const tasksStore = useTasksStore()
const { projects, initializing, initialized } = storeToRefs(tasksStore)

// Enquanto a 1a tentativa de boot nao settla, mostramos "carregando" para nao
// piscar o aviso de "nenhum board" antes dos dados chegarem.
const booting = ref(true)

onMounted(async () => {
  // Lazy: so busca os boards quando a aba e aberta pela primeira vez. Sem
  // auto-criar board (uma conta-cliente vazia nao deve ganhar board fantasma).
  try {
    if (!initialized.value && !initializing.value) {
      await tasksStore.initialize({ allowAutoCreate: false })
    }
  } catch {
    // Conta sem modulo tasks / sem permissao: cai no aviso acionavel de "nenhum board".
  } finally {
    booting.value = false
  }
})

const boardOptions = computed(() =>
  projects.value.map((project) => ({ value: project.id, label: project.name })),
)

const selectedBoard = computed(
  () => projects.value.find((project) => project.id === props.modelValue.boardId) || null,
)

const columnOptions = computed(() =>
  (selectedBoard.value?.columns || []).map((column) => ({
    value: column.id,
    label: column.label,
  })),
)

// Opcoes de tipo do evento-espelho e a lista de status (para o mapa status<->coluna, E5).
const typeOptions = computed(() =>
  Object.entries(EVENT_TYPE_META).map(([value, meta]) => ({ value, label: meta.label })),
)
const statusList = computed(() =>
  Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: meta.label })),
)

const hasBoards = computed(() => projects.value.length > 0)
const loading = computed(() => booting.value || initializing.value)

// patch aplica uma alteracao parcial preservando o resto do modelo (nunca dropar campos).
function patch(next: Partial<CalendarTasksConfig>): void {
  emit('update:modelValue', { ...props.modelValue, ...next })
}

function setBoard(boardId: string): void {
  // Trocar de board invalida a coluna e o mapa de status (as colunas eram do board antigo).
  const board = projects.value.find((project) => project.id === boardId) || null
  const keepColumn = board?.columns.some((c) => c.id === props.modelValue.defaultColumnId)
    ? props.modelValue.defaultColumnId
    : ''
  patch({ boardId, defaultColumnId: keepColumn, statusColumnMap: [] })
}

function setColumn(columnId: string): void {
  patch({ defaultColumnId: columnId })
}

function setMirror(checked: boolean): void {
  patch({ mirrorTasks: checked })
}

function setDefaultEventType(value: string): void {
  patch({ defaultEventType: value })
}

// Mapa status<->coluna: a coluna atual de um status (E5) e o setter que atualiza o mapa.
function columnForStatus(status: string): string {
  return props.modelValue.statusColumnMap.find((e) => e.eventStatus === status)?.columnId || ''
}

function setStatusColumn(status: string, columnId: string): void {
  const map = props.modelValue.statusColumnMap.filter((e) => e.eventStatus !== status)
  if (columnId) {
    map.push({ eventStatus: status, columnId })
  }
  patch({ statusColumnMap: map })
}

// Auto-mapeamento default (WAVE 6, "vc já mapeando"): quando ha board configurado e o mapa
// status<->coluna esta VAZIO, preenche proporcionalmente (1o status -> 1a coluna; ultimo ->
// ultima; do meio distribuidos). Fica visivel/editavel na tabela abaixo e o footer salva. So
// preenche quando vazio (nao sobrescreve um mapa que o dono ja ajustou).
watch(
  () => [props.modelValue.boardId, columnOptions.value.length, loading.value] as const,
  ([boardId, colCount, isLoading]) => {
    if (isLoading || !boardId || !colCount) return
    if (props.modelValue.statusColumnMap.length) return
    const cols = columnOptions.value
    const statuses = statusList.value
    const map = statuses.map((st, i) => {
      const colIdx =
        statuses.length <= 1 ? 0 : Math.round((i / (statuses.length - 1)) * (cols.length - 1))
      return { eventStatus: st.value, columnId: cols[Math.min(colIdx, cols.length - 1)]!.value }
    })
    patch({ statusColumnMap: map })
  },
  { immediate: true },
)
</script>

<template>
  <section class="calendar-config__section">
    <h3 class="calendar-config__section-title">Integração com Tasks</h3>
    <p class="calendar-config__hint">
      Ao criar um evento você pode gerar uma task vinculada no board/coluna escolhidos aqui. Deixe o
      board vazio para desligar a integração.
    </p>

    <p v-if="loading" class="calendar-config__hint">Carregando boards…</p>

    <p v-else-if="!hasBoards" class="calendar-config__warn">
      <UIcon name="i-lucide-alert-triangle" aria-hidden="true" />
      <span>
        Nenhum board disponível.
        <NuxtLink to="/tasks" class="calendar-config__link">Crie um na página de Tasks</NuxtLink>
        para ligar a integração.
      </span>
    </p>

    <template v-else>
      <div class="calendar-config__grid2">
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Board de destino</span>
          <select
            class="calendar-config__input"
            :value="modelValue.boardId"
            @change="setBoard(($event.target as HTMLSelectElement).value)"
          >
            <option value="">Nenhum (integração desligada)</option>
            <option v-for="opt in boardOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>

        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Coluna inicial</span>
          <select
            class="calendar-config__input"
            :value="modelValue.defaultColumnId"
            :disabled="!modelValue.boardId"
            @change="setColumn(($event.target as HTMLSelectElement).value)"
          >
            <option value="">Primeira coluna do board</option>
            <option v-for="opt in columnOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
          <span class="calendar-config__hint">Vazio = a primeira coluna do board escolhido.</span>
        </label>
      </div>

      <!-- WAVE 5: espelho task -> evento + tipo do evento-espelho. So faz sentido com board. -->
      <div v-if="modelValue.boardId" class="calendar-config__grid2 calendar-config__mt">
        <label class="calendar-config__check">
          <input
            type="checkbox"
            :checked="modelValue.mirrorTasks"
            @change="setMirror(($event.target as HTMLInputElement).checked)"
          />
          <span>
            <strong>Espelhar tasks no calendário</strong>
            <span class="calendar-config__hint">
              Tasks com prazo neste board viram um evento no calendário (e sincronizam nos dois
              lados). Desligue para manter só evento → task.
            </span>
          </span>
        </label>

        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Tipo do evento espelhado</span>
          <select
            class="calendar-config__input"
            :value="modelValue.defaultEventType"
            :disabled="!modelValue.mirrorTasks"
            @change="setDefaultEventType(($event.target as HTMLSelectElement).value)"
          >
            <option value="">Padrão (post)</option>
            <option v-for="opt in typeOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>
      </div>

      <!-- WAVE 5 (E5): mapa status do evento <-> coluna do board (sincroniza nos dois sentidos). -->
      <div v-if="modelValue.boardId" class="calendar-config__mt">
        <h4 class="calendar-config__subtitle">Sincronizar status ↔ coluna</h4>
        <p class="calendar-config__hint">
          Mudar o status do evento move a task para a coluna mapeada — e mover a task para a coluna
          volta o status ao evento. Deixe em “—” para não sincronizar aquele status.
        </p>
        <div class="calendar-config__maprows">
          <label v-for="st in statusList" :key="st.value" class="calendar-config__maprow">
            <span class="calendar-config__field-label">{{ st.label }}</span>
            <select
              class="calendar-config__input"
              :value="columnForStatus(st.value)"
              @change="setStatusColumn(st.value, ($event.target as HTMLSelectElement).value)"
            >
              <option value="">—</option>
              <option v-for="opt in columnOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </label>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.calendar-config__mt {
  margin-top: 1rem;
}

.calendar-config__subtitle {
  margin: 0 0 0.35rem;
  font-size: 0.86rem;
  font-weight: 700;
  color: var(--text-main);
}

.calendar-config__check {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  cursor: pointer;
}

.calendar-config__check input {
  margin-top: 0.2rem;
  flex: 0 0 auto;
}

.calendar-config__check span {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.calendar-config__maprows {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 0.6rem;
}

.calendar-config__maprow {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
</style>
