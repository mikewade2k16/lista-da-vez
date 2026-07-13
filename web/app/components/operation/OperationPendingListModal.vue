<script setup>
import { buildNickname } from '~/domain/utils/person-display'
import { formatClock, formatDuration } from '~/domain/utils/time'

// Lista de pendencias do auto-encerramento (2h): atendimentos encerrados
// automaticamente aguardando a gestao ENCERRAR de verdade (mesmo modal de
// encerramento do fluxo normal; fica registrado que foi pelo gerente, quando e
// com a justificativa de por que o consultor nao encerrou).
defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  items: {
    type: Array,
    default: () => [],
  },
})

const emit = defineEmits(['close', 'finish'])

function personLabel(item) {
  return buildNickname(item?.personName || '') || 'Consultor'
}

function durationLabel(item) {
  return formatDuration(Math.max(0, Number(item?.durationMs || 0)))
}

function closedAtLabel(item) {
  const closedAt = Number(item?.autoClosedAt || item?.finishedAt || 0)
  return closedAt > 0 ? formatClock(closedAt) : ''
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="modal-backdrop"
      data-testid="operation-pending-list-backdrop"
      @click.self="emit('close')"
    >
      <div
        class="finish-modal finish-modal--compact"
        role="dialog"
        aria-modal="true"
        data-testid="operation-pending-list-modal"
      >
        <div class="finish-modal__header">
          <div>
            <h2 class="finish-modal__title">Atendimentos para encerrar</h2>
            <p class="finish-modal__subtitle">
              Encerrados automaticamente — encerre cada um com o desfecho real e a justificativa.
            </p>
          </div>
          <div class="finish-modal__header-actions">
            <button
              class="finish-modal__close"
              type="button"
              aria-label="Fechar"
              @click="emit('close')"
            >
              x
            </button>
          </div>
        </div>

        <div class="pending-list__body">
          <p v-if="!items.length" class="pending-list__empty">
            Nenhum atendimento pendente de encerramento.
          </p>
          <article v-for="item in items" :key="item.serviceId" class="pending-list__item">
            <div class="pending-list__info">
              <strong class="pending-list__name">
                {{ personLabel(item) }}
                <span v-if="item.storeName" class="pending-list__store">
                  {{ item.storeName }}
                </span>
              </strong>
              <span class="pending-list__meta">
                {{ durationLabel(item) }} de atendimento
                <template v-if="closedAtLabel(item)">
                  · encerrado automaticamente as {{ closedAtLabel(item) }}
                </template>
                <template v-if="Number(item.snoozeCount || 0) > 0">
                  · adiado {{ item.snoozeCount }}x
                </template>
              </span>
            </div>
            <button
              class="column-action column-action--primary pending-list__finish"
              type="button"
              :data-testid="`operation-pending-finish-${item.serviceId}`"
              @click="emit('finish', item)"
            >
              Encerrar
            </button>
          </article>
        </div>
      </div>
    </div>
  </Teleport>
</template>
