<script setup lang="ts">
import { computed } from 'vue'
import { getRoleLabel } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

// Bloco "Conta e acesso" (somente leitura): mostra ao usuario o proprio contexto
// — papel, conta/cliente ativo, organizacao e modulos contratados. Tudo ja vem
// no contexto da sessao (auth.principal / useCoreAccountStore); nenhuma chamada
// nova. Isolamento garantido: o back so devolve o que o Principal pode ver.
const auth = useAuthStore()
const account = useCoreAccountStore()

const MODULE_LABELS: Record<string, string> = {
  queue: 'Fila / Operacao',
  crm: 'CRM / ERP',
  tasks: 'Tasks',
  site: 'Site',
  bio: 'Bio',
  cardapio: 'Cardapio',
  meta_ads: 'Meta Ads',
}

const roleLabel = computed(() => getRoleLabel(auth.role))
const accountName = computed(() => String(account.activeAccount?.name || '').trim())
const orgName = computed(() => String(account.activeAccount?.organizationName || '').trim())
const modules = computed(() =>
  (account.enabledModules || []).map((id) => ({
    id,
    label: MODULE_LABELS[id] || id,
  })),
)
</script>

<template>
  <article class="settings-card profile-access">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Conta e acesso</h3>
      <p class="settings-card__text">Seu papel e o que voce acessa nesta conta.</p>
    </header>

    <dl class="profile-access__grid">
      <div class="profile-access__item">
        <dt class="profile-access__label">Papel</dt>
        <dd class="profile-access__value">
          <span class="profile-access__badge">{{ roleLabel }}</span>
        </dd>
      </div>
      <div class="profile-access__item">
        <dt class="profile-access__label">Conta ativa</dt>
        <dd class="profile-access__value">{{ accountName || '—' }}</dd>
      </div>
      <div v-if="orgName" class="profile-access__item">
        <dt class="profile-access__label">Organizacao</dt>
        <dd class="profile-access__value">{{ orgName }}</dd>
      </div>
    </dl>

    <div v-if="modules.length" class="profile-access__modules">
      <span class="profile-access__label">Modulos contratados</span>
      <ul class="profile-access__chips">
        <li v-for="mod in modules" :key="mod.id" class="profile-access__chip">{{ mod.label }}</li>
      </ul>
    </div>
  </article>
</template>

<style scoped>
.profile-access__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.85rem;
  margin: 0;
}

.profile-access__item {
  display: grid;
  gap: 0.25rem;
}

.profile-access__label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.profile-access__value {
  margin: 0;
  font-size: 0.9rem;
  color: var(--text-main);
}

.profile-access__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent-info) 16%, transparent);
  color: var(--accent-info);
  font-size: 0.8rem;
  font-weight: 600;
}

.profile-access__modules {
  display: grid;
  gap: 0.45rem;
  margin-top: 1rem;
}

.profile-access__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.profile-access__chip {
  padding: 0.2rem 0.6rem;
  border-radius: var(--radius-soft);
  border: 1px solid var(--line-soft);
  background: var(--bg-muted);
  color: var(--text-main);
  font-size: 0.78rem;
}
</style>
