<script setup lang="ts">
import { computed } from 'vue'

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

const badgesMeta = computed<string>(() => {
  const badges = ctx.gamificationBadges || []
  const active = badges.filter((badge) => badge.enabled).length
  return `${active}/${badges.length} ativos`
})
</script>

<template>
  <div class="settings-grid">
    <SettingsScoreWeightsCard
      :settings="ctx.state.settings"
      :can-edit="ctx.canEditSettings"
      :on-change-weight="ctx.updateNumericSetting"
    />

    <article class="settings-card gamification-badges">
      <header class="gamification-badges__header">
        <div>
          <h3 class="settings-card__title">Badges de gamificacao</h3>
          <p class="settings-card__text">
            Conquistas exibidas no perfil do consultor. Desabilitar oculta o badge sem apagar o
            historico.
          </p>
        </div>
        <span class="gamification-badges__meta">{{ badgesMeta }}</span>
      </header>

      <div class="gamification-badge-list">
        <div v-for="badge in ctx.gamificationBadges" :key="badge.id" class="gamification-badge-row">
          <div class="gamification-badge-row__heading">
            <span class="gamification-badge-row__icon">{{ badge.icon }}</span>
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

          <label v-if="badge.id === 'top-rank'" class="gamification-badge-row__threshold">
            <span>Top N</span>
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
        </div>
      </div>
    </article>
  </div>
</template>

<style scoped>
.settings-grid {
  gap: 0.65rem;
}

.gamification-badges {
  display: grid;
  gap: 0.55rem;
  padding: 0.65rem;
}

.gamification-badges__header,
.gamification-badge-row__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.gamification-badges__header .settings-card__text {
  margin: 0.1rem 0 0;
  font-size: 0.7rem;
  line-height: 1.25;
}

.gamification-badges__meta {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--primary) / 0.24);
  border-radius: 999px;
  padding: 0.25rem 0.45rem;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-size: 0.68rem;
  font-weight: 700;
}

.gamification-badge-list {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.4rem;
}

.gamification-badge-row {
  display: grid;
  align-content: start;
  gap: 0.35rem;
  min-width: 0;
  padding: 0.45rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface) / 0.6);
}

.gamification-badge-row__icon {
  font-size: 1.05rem;
  flex-shrink: 0;
  line-height: 1;
}

.gamification-badge-row__label {
  font-size: 0.76rem;
  font-weight: 500;
  color: var(--text-main);
  min-width: 0;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  padding: 0.32rem 0.4rem;
  background: rgb(var(--surface-2) / 0.76);
  width: 100%;
  transition: border-color 0.15s;
}

.gamification-badge-row__label:not(:disabled):hover,
.gamification-badge-row__label:not(:disabled):focus {
  border-color: rgb(var(--primary) / 0.48);
  outline: none;
}

.gamification-badge-row__desc {
  min-height: 1.8rem;
  font-size: 0.68rem;
  color: var(--text-muted);
  line-height: 1.3;
}

.gamification-badge-row__threshold {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.72rem;
  color: var(--text-muted);
}

.gamification-badge-row__threshold input[type='number'] {
  width: 3.25rem;
  padding: 0.2rem 0.3rem;
  font-size: 0.72rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-main);
}

.gamification-badge-row__toggle {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.72rem;
  color: var(--text-muted);
  cursor: pointer;
  user-select: none;
}

.gamification-badge-row__toggle input[type='checkbox'] {
  cursor: pointer;
  accent-color: rgb(var(--primary));
}

@media (max-width: 1200px) {
  .gamification-badge-list {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .gamification-badges__header {
    align-items: flex-start;
  }

  .gamification-badge-list {
    grid-template-columns: 1fr;
  }
}
</style>
