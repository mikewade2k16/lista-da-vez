<script setup lang="ts">
import type { ObsHealth } from '~/types/observability'
import { formatInt } from '~/utils/observability/format'
import { formatAge } from '~/utils/observability/health'

defineProps<{ online: ObsHealth['online']; now: number }>()
</script>

<template>
  <section class="obs-section">
    <div class="obs-section__head">
      <h3 class="obs-section__title">Usuários online</h3>
      <strong>{{ formatInt(online.count) }}</strong>
    </div>
    <p v-if="online.users.length === 0" class="obs-hint">Ninguém online nos últimos 90 s.</p>
    <div v-else class="obs-tablewrap">
      <table class="obs-table">
        <thead>
          <tr>
            <th>Nome</th>
            <th>Página atual</th>
            <th>Visto</th>
            <th class="obs-num">Abas</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in online.users" :key="user.userId">
            <td>{{ user.name || user.userId }}</td>
            <td class="obs-mono">{{ user.page || '-' }}</td>
            <td>{{ formatAge(user.lastSeen, now) }}</td>
            <td class="obs-num">{{ formatInt(user.tabs) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
