<script setup lang="ts">
import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import type { ErpBancoSection, ErpTab } from '~/domain/utils/erp-display'

defineProps<{
  activeBancoSection: ErpBancoSection
  activeBancoTab: string
  bancoTabs: ErpTab[]
  currentProductCount: number
  rawItemRows: number
}>()

const emit = defineEmits<{
  (e: 'update:activeBancoTab', value: string): void
}>()
</script>

<template>
  <section class="erp-banco">
    <div class="erp-banco__intro">
      <h3>{{ activeBancoSection.title }}</h3>
      <p>{{ activeBancoSection.text }}</p>
    </div>

    <SettingsTabs
      :tabs="bancoTabs"
      :active-tab="activeBancoTab"
      @update:active-tab="emit('update:activeBancoTab', $event)"
    />

    <div class="erp-banco__grid">
      <article
        v-for="item in activeBancoSection.cards"
        :key="item.table"
        class="erp-banco__card"
        :class="`erp-banco__card--${item.badge.split(' ')[0]}`"
      >
        <div class="erp-banco__card-head">
          <code class="erp-banco__table-name">{{ item.table }}</code>
          <span class="erp-banco__badge">{{ item.badge }}</span>
        </div>
        <strong class="erp-banco__card-label">{{ item.label }}</strong>
        <p class="erp-banco__card-desc">{{ item.desc }}</p>
        <div v-if="item.table === 'erp_item_current'" class="erp-banco__live">
          <span class="erp-banco__live-label">Registros atuais</span>
          <strong class="erp-banco__live-count">
            {{ currentProductCount.toLocaleString('pt-BR') }}
          </strong>
        </div>
        <div v-if="item.table === 'erp_item_raw'" class="erp-banco__live">
          <span class="erp-banco__live-label">Linhas brutas importadas</span>
          <strong class="erp-banco__live-count">{{ rawItemRows.toLocaleString('pt-BR') }}</strong>
        </div>
      </article>
    </div>

    <div class="erp-banco__note">
      {{ activeBancoSection.note }}
    </div>
  </section>
</template>
