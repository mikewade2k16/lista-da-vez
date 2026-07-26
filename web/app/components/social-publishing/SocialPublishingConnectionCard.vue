<script setup lang="ts">
import type { SocialPublishingConnection } from '~/domain/social-publishing/model'

const props = defineProps<{
  connection: SocialPublishingConnection | null
  busy: boolean
  canConnect: boolean
}>()

const emit = defineEmits<{
  connect: [accessToken: string]
  disconnect: []
}>()

const accessToken = ref('')
const localError = ref('')

const statusLabel = computed(() => {
  if (props.connection?.connected) return 'Conectado'
  if (props.connection?.status === 'pending') return 'Validando'
  if (props.connection?.status === 'error') return 'Erro na conexão'
  return 'Desconectado'
})

const secretLabel = computed(() => {
  if (!props.connection?.secretSet) return 'Nenhum token configurado'
  return props.connection.secretLast4
    ? `Token configurado · •••• ${props.connection.secretLast4}`
    : 'Token configurado'
})

function submit(): void {
  const normalized = accessToken.value.trim()
  if (!normalized) {
    localError.value = 'Cole um token de acesso válido para continuar.'
    return
  }
  localError.value = ''
  emit('connect', normalized)
  accessToken.value = ''
}
</script>

<template>
  <div class="sp-connection">
    <section class="sp-connection__pilot omni-glass" aria-labelledby="sp-connection-title">
      <div class="sp-connection__head">
        <div class="sp-connection__brand" aria-hidden="true">
          <UIcon name="i-lucide-instagram" />
        </div>
        <div>
          <div class="sp-connection__title-line">
            <h2 id="sp-connection-title">Instagram profissional</h2>
            <span>Beta técnico</span>
          </div>
          <p>Conexão inicial por token para validar publicação e leitura de analytics.</p>
        </div>
      </div>

      <div class="sp-connection__status">
        <span
          class="sp-connection__status-dot"
          :class="{ 'sp-connection__status-dot--active': connection?.connected }"
          aria-hidden="true"
        ></span>
        <div>
          <strong>{{ statusLabel }}</strong>
          <p>{{ secretLabel }}</p>
        </div>
      </div>

      <dl v-if="connection?.connected" class="sp-connection__details">
        <div>
          <dt>Perfil</dt>
          <dd>{{ connection.username ? `@${connection.username}` : 'Conta profissional' }}</dd>
        </div>
        <div>
          <dt>Tipo</dt>
          <dd>{{ connection.accountType || 'Não informado' }}</dd>
        </div>
        <div>
          <dt>Mídias no perfil</dt>
          <dd>{{ connection.mediaCount.toLocaleString('pt-BR') }}</dd>
        </div>
        <div>
          <dt>ID Instagram</dt>
          <dd>{{ connection.igUserId || 'Não informado' }}</dd>
        </div>
      </dl>

      <form
        v-if="canConnect && !connection?.connected"
        class="sp-connection__form"
        @submit.prevent="submit"
      >
        <label for="sp-access-token">Token de acesso</label>
        <div class="sp-connection__input-row">
          <input
            id="sp-access-token"
            v-model="accessToken"
            type="password"
            name="instagram-access-token"
            autocomplete="new-password"
            spellcheck="false"
            :disabled="busy"
            aria-describedby="sp-token-help"
          />
          <UButton
            type="submit"
            color="primary"
            label="Validar conexão"
            icon="i-lucide-plug-zap"
            :loading="busy"
          />
        </div>
        <p id="sp-token-help">
          O valor é enviado uma única vez, armazenado cifrado pelo backend e nunca volta para a
          tela.
        </p>
        <p v-if="localError" class="sp-connection__error" role="alert">{{ localError }}</p>
      </form>

      <div v-else-if="canConnect" class="sp-connection__actions">
        <UButton
          type="button"
          color="error"
          variant="soft"
          label="Desconectar Instagram"
          icon="i-lucide-unplug"
          :loading="busy"
          @click="emit('disconnect')"
        />
      </div>

      <p v-else class="sp-connection__readonly">
        Você pode consultar o status, mas não possui permissão para alterar a conexão.
      </p>
    </section>

    <aside class="sp-connection__notice omni-glass" aria-labelledby="sp-scopes-title">
      <div class="sp-connection__notice-icon" aria-hidden="true">
        <UIcon name="i-lucide-flask-conical" />
      </div>
      <div>
        <h2 id="sp-scopes-title">Piloto controlado</h2>
        <p>
          Esta etapa usa um token técnico de uma conta profissional. O fluxo OAuth para clientes
          será uma evolução posterior, depois da validação operacional.
        </p>
        <h3>Permissões esperadas no app Meta</h3>
        <ul>
          <li><code>instagram_business_basic</code></li>
          <li><code>instagram_business_content_publish</code></li>
          <li><code>instagram_business_manage_insights</code></li>
        </ul>
        <p class="sp-connection__warning">
          Nunca compartilhe o token em mensagens, tarefas ou documentos. Revogue-o no Meta caso
          suspeite de exposição.
        </p>
        <details class="sp-connection__guide">
          <summary>
            <UIcon name="i-lucide-book-open-check" aria-hidden="true" />
            Como obter este token
          </summary>
          <ol>
            <li>
              No
              <a
                href="https://developers.facebook.com/apps/"
                target="_blank"
                rel="noopener noreferrer"
              >
                Meta for Developers,
              </a>
              abra o app da plataforma e configure
              <strong>Instagram API with Instagram Login.</strong>
            </li>
            <li>
              Adicione a conta profissional Business ou Creator como conta de teste e conclua o
              aceite no Instagram.
            </li>
            <li>
              Em
              <strong>Generate access tokens</strong>
              , autorize as três permissões acima e copie o Instagram User Access Token.
            </li>
            <li>
              Selecione o cliente correto no topo desta página, cole o token uma única vez e confira
              o
              <strong>@usuário</strong>
              validado antes de agendar.
            </li>
          </ol>
          <p>
            Este é o fluxo manual do piloto. Para clientes em produção, a conexão deverá usar OAuth
            e Advanced Access aprovado pela Meta.
          </p>
          <a
            class="sp-connection__guide-link"
            href="https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/get-started/"
            target="_blank"
            rel="noopener noreferrer"
          >
            Abrir o guia oficial da Meta
            <UIcon name="i-lucide-external-link" aria-hidden="true" />
          </a>
        </details>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.sp-connection {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(18rem, 0.85fr);
  gap: 1rem;
  align-items: start;
}

.sp-connection__pilot,
.sp-connection__notice {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-xs);
}

.sp-connection__pilot {
  display: grid;
  gap: 1.1rem;
  padding: 1.2rem;
}

.sp-connection__head {
  display: flex;
  align-items: center;
  gap: 0.8rem;
}

.sp-connection__brand,
.sp-connection__notice-icon {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  border-radius: var(--radius-soft);
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
}

.sp-connection__brand {
  width: 3rem;
  height: 3rem;
}

.sp-connection__brand :deep(svg) {
  width: 1.35rem;
  height: 1.35rem;
}

.sp-connection__title-line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.sp-connection h2,
.sp-connection h3,
.sp-connection p {
  margin: 0;
}

.sp-connection h2 {
  color: rgb(var(--text));
  font-size: 1rem;
}

.sp-connection h3 {
  margin-top: 1rem;
  color: rgb(var(--text));
  font-size: 0.8rem;
}

.sp-connection__title-line span {
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
  font-size: 0.66rem;
  font-weight: 750;
  text-transform: uppercase;
}

.sp-connection__head p,
.sp-connection__notice p {
  margin-top: 0.25rem;
  color: rgb(var(--muted));
  font-size: 0.8rem;
  line-height: 1.5;
}

.sp-connection__status {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  padding: 0.8rem;
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.7);
}

.sp-connection__status-dot {
  width: 0.65rem;
  height: 0.65rem;
  flex: 0 0 auto;
  border-radius: 999px;
  background: rgb(var(--muted));
}

.sp-connection__status-dot--active {
  background: rgb(var(--success));
  box-shadow: 0 0 0 4px rgb(var(--success) / 0.12);
}

.sp-connection__status strong {
  color: rgb(var(--text));
  font-size: 0.83rem;
}

.sp-connection__status p {
  margin-top: 0.12rem;
  color: rgb(var(--muted));
  font-size: 0.74rem;
}

.sp-connection__details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
  margin: 0;
}

.sp-connection__details div {
  min-width: 0;
  padding: 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-xs);
}

.sp-connection__details dt {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.sp-connection__details dd {
  margin: 0.2rem 0 0;
  overflow: hidden;
  color: rgb(var(--text));
  font-size: 0.8rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sp-connection__form {
  display: grid;
  gap: 0.4rem;
}

.sp-connection__form label {
  color: rgb(var(--text));
  font-size: 0.8rem;
  font-weight: 650;
}

.sp-connection__input-row {
  display: flex;
  gap: 0.55rem;
}

.sp-connection__input-row input {
  width: 100%;
  min-width: 0;
  min-height: 2.55rem;
  padding: 0 0.75rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-xs);
  outline: none;
  color: rgb(var(--text));
  background: rgb(var(--surface));
}

.sp-connection__input-row input:focus {
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.14);
}

.sp-connection__form > p,
.sp-connection__readonly {
  color: rgb(var(--muted));
  font-size: 0.72rem;
  line-height: 1.45;
}

.sp-connection__form .sp-connection__error {
  color: rgb(var(--danger));
}

.sp-connection__notice {
  display: flex;
  gap: 0.75rem;
  padding: 1rem;
}

.sp-connection__notice-icon {
  width: 2.35rem;
  height: 2.35rem;
}

.sp-connection__notice ul {
  display: grid;
  gap: 0.35rem;
  margin: 0.5rem 0 0;
  padding-left: 1.1rem;
  color: rgb(var(--muted));
  font-size: 0.74rem;
}

.sp-connection__notice code {
  overflow-wrap: anywhere;
  color: rgb(var(--text));
}

.sp-connection__notice .sp-connection__warning {
  margin-top: 1rem;
  padding: 0.65rem;
  border-radius: var(--radius-xs);
  color: rgb(var(--text));
  background: rgb(var(--warning) / 0.14);
}

.sp-connection__guide {
  margin-top: 0.8rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--line-soft);
}

.sp-connection__guide summary {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: rgb(var(--primary));
  cursor: pointer;
  font-size: 0.78rem;
  font-weight: 750;
  list-style: none;
}

.sp-connection__guide summary::-webkit-details-marker {
  display: none;
}

.sp-connection__guide summary :deep(svg),
.sp-connection__guide-link :deep(svg) {
  width: 0.95rem;
  height: 0.95rem;
  flex: 0 0 auto;
}

.sp-connection__guide ol {
  display: grid;
  gap: 0.55rem;
  margin: 0.75rem 0 0;
  padding-left: 1.15rem;
  color: rgb(var(--muted));
  font-size: 0.74rem;
  line-height: 1.5;
}

.sp-connection__guide a {
  color: rgb(var(--primary));
}

.sp-connection__guide .sp-connection__guide-link {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  margin-top: 0.7rem;
  font-size: 0.74rem;
  font-weight: 700;
  text-decoration: none;
}

.sp-connection__guide-link:hover {
  text-decoration: underline;
}

@media (max-width: 900px) {
  .sp-connection {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 560px) {
  .sp-connection__input-row {
    align-items: stretch;
    flex-direction: column;
  }

  .sp-connection__details {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
