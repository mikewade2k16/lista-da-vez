<script setup lang="ts">
import { onMounted } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

function onClientChange(adAccountId: string, event: Event): void {
  const clientAccountId = (event.target as HTMLSelectElement).value
  void store.setAdAccountClient(adAccountId, clientAccountId)
}

function onInstagramClientChange(igUserId: string, event: Event): void {
  const clientAccountId = (event.target as HTMLSelectElement).value
  void store.setInstagramIdentityClient(igUserId, clientAccountId)
}

onMounted(() => {
  void store.loadInstagramIdentities()
})
</script>

<template>
  <article class="ma-client-map" aria-labelledby="ma-client-map-title">
    <header class="ma-client-map__head">
      <div>
        <h3 id="ma-client-map-title" class="ma-client-map__title">Ativos Meta por cliente</h3>
        <p class="ma-client-map__subtitle">
          Vincule cada conta de anúncios ao cliente correto. Esse vínculo limita o contexto usado
          pelo assistente e pelas visões client-side.
        </p>
      </div>
      <span v-if="store.clientScopeLoading" class="ma-client-map__loading">
        Carregando clientes…
      </span>
    </header>

    <p v-if="store.clientMappingError" class="ma-client-map__error" role="alert">
      {{ store.clientMappingError }}
    </p>

    <h4 class="ma-client-map__section-title">Contas de anúncios</h4>

    <div v-if="store.adAccounts.length" class="ma-client-map__list">
      <label v-for="adAccount in store.adAccounts" :key="adAccount.id" class="ma-client-map__row">
        <span class="ma-client-map__account">
          <strong>{{ adAccount.name }}</strong>
          <small>{{ adAccount.metaAdAccountId }}</small>
        </span>
        <select
          class="ma-client-map__select"
          :value="adAccount.clientAccountId || ''"
          :disabled="
            !store.canManageClientMapping ||
            store.clientScopeLoading ||
            store.clientMappingBusyId === adAccount.id
          "
          :aria-label="`Cliente da conta de anúncios ${adAccount.name}`"
          @change="onClientChange(adAccount.id, $event)"
        >
          <option value="">Sem cliente vinculado</option>
          <option v-for="client in store.clientScope.clients" :key="client.id" :value="client.id">
            {{ client.name }}
          </option>
        </select>
      </label>
    </div>

    <p v-else-if="!store.clientScopeLoading" class="ma-client-map__empty">
      Nenhuma conta de anúncios sincronizada.
    </p>

    <section class="ma-client-map__identity-section" aria-labelledby="ma-client-map-identity-title">
      <div class="ma-client-map__identity-head">
        <div>
          <h4 id="ma-client-map-identity-title" class="ma-client-map__section-title">
            Páginas e perfis do Instagram
          </h4>
          <p class="ma-client-map__identity-copy">
            O assistente só mostra ao cliente os posts da identidade vinculada aqui. A lista é
            validada ao vivo na Meta antes de cada alteração.
          </p>
        </div>
        <span v-if="store.instagramIdentitiesLoading" class="ma-client-map__loading">
          Carregando identidades…
        </span>
      </div>

      <p v-if="store.instagramIdentityMappingError" class="ma-client-map__error" role="alert">
        {{ store.instagramIdentityMappingError }}
      </p>

      <div v-if="store.instagramIdentities.length" class="ma-client-map__list">
        <label
          v-for="identity in store.instagramIdentities"
          :key="`${identity.igUserId}:${identity.pageId}`"
          class="ma-client-map__row"
        >
          <span class="ma-client-map__account">
            <strong>{{ identity.username ? `@${identity.username}` : identity.pageName }}</strong>
            <small>{{ identity.pageName || 'Página sem nome' }} · IG {{ identity.igUserId }}</small>
          </span>
          <select
            class="ma-client-map__select"
            :value="identity.clientAccountId || ''"
            :disabled="
              !store.canManageClientMapping ||
              store.instagramIdentitiesLoading ||
              store.instagramIdentityMappingBusyId === identity.igUserId
            "
            :aria-label="`Cliente do Instagram ${identity.username || identity.igUserId}`"
            @change="onInstagramClientChange(identity.igUserId, $event)"
          >
            <option value="">Sem cliente vinculado</option>
            <option v-for="client in store.clientScope.clients" :key="client.id" :value="client.id">
              {{ client.name }}
            </option>
          </select>
        </label>
      </div>

      <p v-else-if="!store.instagramIdentitiesLoading" class="ma-client-map__empty">
        Nenhuma Página com Instagram Business foi encontrada nesta conexão.
      </p>
    </section>

    <p v-if="!store.canManageClientMapping" class="ma-client-map__hint">
      Seu acesso permite consultar os vínculos, mas não alterá-los.
    </p>
  </article>
</template>

<style scoped>
.ma-client-map {
  display: grid;
  gap: 0.9rem;
  padding: 1.1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-card);
}

.ma-client-map__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ma-client-map__title {
  color: var(--text-main);
  font-size: 1rem;
  font-weight: 700;
}

.ma-client-map__subtitle,
.ma-client-map__identity-copy,
.ma-client-map__empty,
.ma-client-map__hint,
.ma-client-map__loading {
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.45;
}

.ma-client-map__section-title {
  margin: 0;
  color: var(--text-main);
  font-size: 0.86rem;
  font-weight: 700;
}

.ma-client-map__identity-section {
  display: grid;
  gap: 0.7rem;
  padding-top: 0.9rem;
  border-top: 1px solid var(--line-soft);
}

.ma-client-map__identity-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ma-client-map__identity-copy {
  max-width: 68ch;
  margin: 0.2rem 0 0;
}

.ma-client-map__subtitle {
  max-width: 68ch;
  margin-top: 0.25rem;
}

.ma-client-map__loading {
  flex: 0 0 auto;
}

.ma-client-map__error {
  margin: 0;
  padding: 0.6rem 0.7rem;
  border-radius: var(--radius-soft);
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
  font-size: 0.8rem;
}

.ma-client-map__list {
  display: grid;
  gap: 0.55rem;
}

.ma-client-map__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(13rem, 0.7fr);
  align-items: center;
  gap: 0.8rem;
  padding: 0.7rem;
  border: 1px solid rgb(var(--border) / 0.82);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.48);
}

.ma-client-map__account {
  display: grid;
  gap: 0.15rem;
  min-width: 0;
}

.ma-client-map__account strong {
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.84rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ma-client-map__account small {
  color: var(--text-muted);
  font-size: 0.7rem;
}

.ma-client-map__select {
  width: 100%;
  min-height: 2.35rem;
  padding: 0.45rem 0.65rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-soft);
  background: rgb(var(--surface));
  color: var(--text-main);
  font: inherit;
  font-size: 0.8rem;
}

.ma-client-map__select:focus-visible {
  outline: 3px solid rgb(var(--ring) / 0.22);
  outline-offset: 1px;
}

.ma-client-map__select:disabled {
  cursor: not-allowed;
  opacity: 0.62;
}

.ma-client-map__empty,
.ma-client-map__hint {
  margin: 0;
}

@media (max-width: 720px) {
  .ma-client-map__head,
  .ma-client-map__identity-head,
  .ma-client-map__row {
    grid-template-columns: 1fr;
  }

  .ma-client-map__head {
    display: grid;
  }
}
</style>
