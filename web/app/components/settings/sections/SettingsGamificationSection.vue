<script setup lang="ts">
import SettingsScoreWeightsCard from '~/components/settings/sections/SettingsScoreWeightsCard.vue'
import type { BadgeRule } from '~/composables/useGamificationConfig'

const props = defineProps<{
  ctx: Record<string, unknown>
}>()

const ctx = props.ctx as {
  canEditSettings: boolean
  gamificationBadges: BadgeRule[]
  updateGamificationBadge: (id: string, patch: Partial<BadgeRule>) => Promise<void>
  updateNumericSetting: (settingId: string, value: string) => Promise<void>
  state: { settings: Record<string, unknown> }
}
</script>

<template>
  <div class="settings-grid">
    <SettingsScoreWeightsCard
      :settings="ctx.state.settings"
      :can-edit="ctx.canEditSettings"
      :on-change-weight="ctx.updateNumericSetting"
    />

    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Badges de gamificacao</h3>
        <p class="settings-card__text">
          Configure quais conquistas aparecem no perfil do consultor. Desabilitar remove o badge do
          ranking sem apagar o historico.
        </p>
      </header>

      <div class="gamification-badge-list">
        <div v-for="badge in ctx.gamificationBadges" :key="badge.id" class="gamification-badge-row">
          <div class="gamification-badge-row__info">
            <span class="gamification-badge-row__icon">{{ badge.icon }}</span>
            <div class="gamification-badge-row__meta">
              <input
                class="gamification-badge-row__label"
                type="text"
                :value="badge.label"
                :disabled="!ctx.canEditSettings"
                @change="
                  ctx.updateGamificationBadge(badge.id, {
                    label: ($event.target as HTMLInputElement).value,
                  })
                "
              />
              <span class="gamification-badge-row__desc">{{ badge.description }}</span>
            </div>
          </div>

          <div class="gamification-badge-row__controls">
            <label v-if="badge.id === 'top-rank'" class="gamification-badge-row__threshold">
              <span class="gamification-badge-row__threshold-label">Top N</span>
              <input
                type="number"
                min="1"
                max="20"
                step="1"
                :value="badge.threshold ?? 3"
                :disabled="!ctx.canEditSettings"
                @change="
                  ctx.updateGamificationBadge(badge.id, {
                    threshold: Number(($event.target as HTMLInputElement).value),
                  })
                "
              />
            </label>

            <label class="gamification-badge-row__toggle">
              <input
                type="checkbox"
                :checked="badge.enabled"
                :disabled="!ctx.canEditSettings"
                @change="
                  ctx.updateGamificationBadge(badge.id, {
                    enabled: ($event.target as HTMLInputElement).checked,
                  })
                "
              />
              <span>{{ badge.enabled ? 'Ativo' : 'Inativo' }}</span>
            </label>
          </div>
        </div>
      </div>
    </article>
  </div>
</template>

<style scoped>
.gamification-badge-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.gamification-badge-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface) / 0.6);
}

.gamification-badge-row__info {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  flex: 1;
  min-width: 0;
}

.gamification-badge-row__icon {
  font-size: 1.25rem;
  flex-shrink: 0;
  line-height: 1;
  margin-top: 0.125rem;
}

.gamification-badge-row__meta {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  flex: 1;
  min-width: 0;
}

.gamification-badge-row__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-main);
  background: transparent;
  border: none;
  border-bottom: 1px solid transparent;
  padding: 0;
  width: 100%;
  transition: border-color 0.15s;
}

.gamification-badge-row__label:not(:disabled):hover,
.gamification-badge-row__label:not(:disabled):focus {
  border-bottom-color: var(--line-strong);
  outline: none;
}

.gamification-badge-row__desc {
  font-size: 0.75rem;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.gamification-badge-row__controls {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-shrink: 0;
}

.gamification-badge-row__threshold {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.8125rem;
  color: var(--text-muted);
}

.gamification-badge-row__threshold input[type='number'] {
  width: 3.5rem;
  padding: 0.25rem 0.375rem;
  font-size: 0.8125rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
}

.gamification-badge-row__toggle {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.8125rem;
  color: var(--text-muted);
  cursor: pointer;
  user-select: none;
}

.gamification-badge-row__toggle input[type='checkbox'] {
  cursor: pointer;
  accent-color: rgb(var(--primary));
}
</style>
