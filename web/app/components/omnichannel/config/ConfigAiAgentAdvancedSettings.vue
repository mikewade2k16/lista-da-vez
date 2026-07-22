<script setup lang="ts">
const debounceMs = defineModel<number>('debounceMs', { required: true })
const maxContextMessages = defineModel<number>('maxContextMessages', { required: true })
const maxAiTurns = defineModel<number>('maxAiTurns', { required: true })
const minConfidence = defineModel<number>('minConfidence', { required: true })
const handoffOnError = defineModel<boolean>('handoffOnError', { required: true })
const handoffOnLimit = defineModel<boolean>('handoffOnLimit', { required: true })

defineProps<{ disabled?: boolean }>()

function percentage(value: number): string {
  const normalized = Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0
  return `${Math.round(normalized * 100)}%`
}
</script>

<template>
  <details class="calendar-config__collapse">
    <summary class="calendar-config__collapse-head">Resposta automática e limites</summary>
    <div class="calendar-config__collapse-body">
      <div class="cfg-advanced__notice">
        <UIcon name="i-lucide-info" />
        <span>
          Estas regras controlam quando a IA responde sozinha. Elas não alteram a confiança exigida
          para encerrar o atendimento, configurada na tab Atendimento.
        </span>
      </div>

      <div class="calendar-config__grid2">
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Tempo para agrupar mensagens</span>
          <input
            v-model.number="debounceMs"
            class="calendar-config__input"
            type="number"
            min="500"
            max="15000"
            step="100"
            :disabled="disabled"
          />
          <small class="calendar-config__hint">
            Aguarda novas mensagens antes de chamar a IA. Ex.: 2500 = 2,5 segundos.
          </small>
        </label>

        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Mensagens de contexto</span>
          <input
            v-model.number="maxContextMessages"
            class="calendar-config__input"
            type="number"
            min="1"
            max="100"
            :disabled="disabled"
          />
          <small class="calendar-config__hint">
            Quantas mensagens recentes a IA recebe para entender a conversa.
          </small>
        </label>

        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Máximo de respostas automáticas</span>
          <input
            v-model.number="maxAiTurns"
            class="calendar-config__input"
            type="number"
            min="1"
            max="20"
            :disabled="disabled"
          />
          <small class="calendar-config__hint">
            Ao atingir este total na conversa, a automação para ou transfere conforme a opção
            abaixo.
          </small>
        </label>

        <label class="calendar-config__field">
          <span class="calendar-config__field-label">
            Confiança mínima para responder: {{ percentage(minConfidence) }}
          </span>
          <input
            v-model.number="minConfidence"
            class="calendar-config__input"
            type="number"
            min="0"
            max="1"
            step="0.01"
            :disabled="disabled"
          />
          <small class="calendar-config__hint">
            Abaixo deste valor, a IA não responde automaticamente e solicita intervenção.
          </small>
        </label>
      </div>

      <div class="cfg-advanced__toggles">
        <label class="calendar-config__check">
          <input v-model="handoffOnError" type="checkbox" :disabled="disabled" />
          <span>
            <strong>Transferir se a IA falhar</strong>
            <small>Chave ausente, provedor indisponível ou resposta inválida.</small>
          </span>
        </label>
        <label class="calendar-config__check">
          <input v-model="handoffOnLimit" type="checkbox" :disabled="disabled" />
          <span>
            <strong>Transferir ao atingir uma regra de parada</strong>
            <small>Confiança baixa, máximo de respostas ou limite mensal.</small>
          </span>
        </label>
      </div>

      <div class="cfg-advanced__manual">
        <UIcon name="i-lucide-mouse-pointer-click" />
        <span>
          <strong>Comando manual</strong>
          “Forçar IA a responder” ignora confiança baixa, máximo de respostas e a sugestão de
          transferência para uma resposta. Configuração ausente, indisponibilidade do modelo, limite
          mensal e geração de conversa inválida continuam protegidos pelo Go.
        </span>
      </div>
    </div>
  </details>
</template>

<style scoped>
.cfg-advanced__toggles {
  display: grid;
  gap: 0.7rem;
}

.cfg-advanced__toggles .calendar-config__check > span,
.cfg-advanced__manual > span {
  display: grid;
  gap: 0.15rem;
}

.cfg-advanced__toggles strong,
.cfg-advanced__manual strong {
  color: var(--text-main);
  font-size: 0.78rem;
}

.cfg-advanced__toggles small {
  color: var(--text-muted);
  font-size: 0.7rem;
}

.cfg-advanced__notice,
.cfg-advanced__manual {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.65rem;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.65);
  color: var(--text-muted);
  font-size: 0.74rem;
  line-height: 1.4;
}

.cfg-advanced__manual {
  border: 1px solid rgb(var(--primary) / 0.3);
  background: rgb(var(--primary) / 0.08);
}
</style>
