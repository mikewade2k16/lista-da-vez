<script setup lang="ts">
import type { ObsHealth } from '~/types/observability'
import { formatInt } from '~/utils/observability/format'

defineProps<{ errors: ObsHealth['errors'] }>()
</script>

<template>
  <section class="obs-section">
    <h3 class="obs-section__title">Erros e falhas</h3>
    <div class="obs-tablewrap">
      <table class="obs-table">
        <thead>
          <tr>
            <th>Período</th>
            <th class="obs-num">API 5xx</th>
            <th class="obs-num">Front</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Última hora</td>
            <td class="obs-num">{{ formatInt(errors.lastHour.api5xx) }}</td>
            <td class="obs-num">{{ formatInt(errors.lastHour.frontend) }}</td>
          </tr>
          <tr>
            <td>24 horas</td>
            <td class="obs-num">{{ formatInt(errors.last24h.api5xx) }}</td>
            <td class="obs-num">{{ formatInt(errors.last24h.frontend) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-if="errors.top.length === 0" class="obs-hint">Nenhuma assinatura de erro recente.</p>
    <ul v-else class="obs-list">
      <li v-for="item in errors.top" :key="`${item.source}-${item.signature}`">
        <span :class="['obs-band', item.source === 'api' ? 'obs-band--bad' : 'obs-band--warn']">
          {{ item.source === 'api' ? 'API' : 'Front' }}
        </span>
        <span class="obs-clip" :title="item.signature">{{ item.signature }}</span>
        <span class="obs-muted">{{ formatInt(item.count) }}x</span>
      </li>
    </ul>
  </section>
</template>
