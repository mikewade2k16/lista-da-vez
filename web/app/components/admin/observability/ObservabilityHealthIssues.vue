<script setup lang="ts">
import type { ObsHealthIssue, ObsTabKey } from '~/types/observability'
import {
  formatAge,
  resolveLinkTab,
  SEVERITY_LABEL,
  severityBand,
} from '~/utils/observability/health'

// "O que está ruim agora": issues já ordenados por severidade pelo back.
defineProps<{ issues: ObsHealthIssue[]; now: number }>()
const emit = defineEmits<{ navigate: [tab: ObsTabKey] }>()

function open(issue: ObsHealthIssue) {
  const tab = resolveLinkTab(issue.link)
  if (tab) emit('navigate', tab)
}
</script>

<template>
  <section class="obs-section">
    <h3 class="obs-section__title">O que está ruim agora</h3>
    <p v-if="issues.length === 0" class="obs-text--good obs-health__ok">
      <UIcon name="i-lucide-circle-check" />
      Nada ruim no momento.
    </p>
    <ul v-else class="obs-health__issues">
      <li v-for="(issue, index) in issues" :key="`${issue.title}-${index}`">
        <button
          type="button"
          class="obs-health__issue"
          :disabled="!resolveLinkTab(issue.link)"
          @click="open(issue)"
        >
          <span :class="['obs-band', `obs-band--${severityBand(issue.severity)}`]">
            {{ SEVERITY_LABEL[issue.severity] || issue.severity }}
          </span>
          <span class="obs-health__issuetext">
            <strong>{{ issue.title }}</strong>
            <span class="obs-muted obs-clip">{{ issue.detail }}</span>
          </span>
          <span class="obs-muted obs-health__since">desde {{ formatAge(issue.since, now) }}</span>
        </button>
      </li>
    </ul>
  </section>
</template>
