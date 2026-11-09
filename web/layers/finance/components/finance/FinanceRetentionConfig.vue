<script setup lang="ts">
import { computed } from 'vue'
import { useFinanceRetention } from '../../composables/useFinanceRecords'
import {
  formatBytes,
  formatRecordDate,
  RETENTION_PRESETS,
  retentionDaysLabel,
} from '../../utils/finance-records'

// Prazo de guarda do arquivo original dos comprovantes (política da conta).
// Só o arquivo é apagado no prazo; dados extraídos, vínculos e históricos ficam no banco.
defineProps<{ canManage: boolean }>()
const retention = useFinanceRetention()
const { settings, history, days, dirty, saving, loading, error, status } = retention
const summary = computed(() =>
  settings.value
    ? `${retentionDaysLabel(settings.value.receiptOriginalDays)}${settings.value.isDefault ? ' · padrão' : ''}`
    : 'carregando',
)
function setDays(value: number) {
  days.value = value
  retention.touched.value = true
}
</script>

<template>
  <details class="finance-records">
    <summary>
      Retenção dos originais
      <span class="finance-records__summary-meta">{{ summary }}</span>
    </summary>
    <p class="finance-records__intro">
      O arquivo enviado (PDF ou foto do comprovante) é guardado em armazenamento privado e apagado
      depois do prazo. Valor, data, partes, vínculos com a planilha e históricos continuam no banco
      sem prazo.
    </p>
    <p v-if="loading" class="finance-records__status" role="status">Carregando...</p>
    <p v-if="error" class="finance-records__status finance-records__status--error" role="alert">
      {{ error }}
    </p>
    <p
      v-if="settings && !settings.storageAvailable"
      class="finance-records__status finance-records__status--error"
      role="alert"
    >
      Armazenamento privado desligado neste ambiente: novos originais não estão sendo guardados.
    </p>
    <template v-if="settings">
      <div class="finance-records__stats">
        <div class="finance-records__stat">
          <strong>{{ settings.stored }}</strong>
          guardados ({{ formatBytes(settings.storedBytes) }})
        </div>
        <div class="finance-records__stat">
          <strong>{{ settings.purged }}</strong>
          já apagados
        </div>
        <div class="finance-records__stat">
          <strong>{{ settings.notStored }}</strong>
          não guardados
        </div>
        <div class="finance-records__stat">
          <strong>{{ formatRecordDate(settings.nextExpiry) || '—' }}</strong>
          próximo vencimento
        </div>
      </div>
      <form @submit.prevent="retention.save">
        <fieldset :disabled="!canManage || saving" class="finance-records__form">
          <label>
            Prazo
            <select
              :value="days"
              @change="setDays(Number(($event.target as HTMLSelectElement).value))"
            >
              <option v-for="preset in RETENTION_PRESETS" :key="preset.days" :value="preset.days">
                {{ preset.label }}
              </option>
              <option
                v-if="!RETENTION_PRESETS.some((preset) => preset.days === days)"
                :value="days"
              >
                {{ days }} dias
              </option>
            </select>
          </label>
          <label>
            Ou em dias (1 a 3650)
            <input
              :value="days"
              type="number"
              min="1"
              max="3650"
              inputmode="numeric"
              @input="setDays(Number(($event.target as HTMLInputElement).value))"
            />
          </label>
          <div class="finance-records__form-actions">
            <button type="submit" class="finance-records__button" :disabled="!dirty || saving">
              {{ saving ? 'Salvando...' : 'Salvar prazo' }}
            </button>
            <button type="button" class="finance-records__button" @click="retention.loadHistory">
              Ver histórico de alterações
            </button>
          </div>
        </fieldset>
      </form>
      <p v-if="!canManage" class="finance-records__status">
        Somente quem gerencia a configuração financeira pode mudar o prazo.
      </p>
      <p v-if="dirty" class="finance-records__status" role="status">
        O novo prazo também vale para os originais já guardados.
      </p>
      <p v-if="status" class="finance-records__status" role="status">{{ status }}</p>
      <ul v-if="history.length" class="finance-records__history">
        <li v-for="event in history" :key="event.version">
          {{ formatRecordDate(event.createdAt) }}: {{ event.previousDays ?? '90 (padrão)' }} →
          {{ event.receiptOriginalDays }} dias (v{{ event.version }})
        </li>
      </ul>
    </template>
  </details>
</template>

<style src="../../assets/finance-records.css"></style>
