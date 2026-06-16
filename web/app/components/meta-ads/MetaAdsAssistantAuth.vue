<script setup lang="ts">
import { ref } from 'vue'
import { useMetaAdsStore } from '~/stores/meta-ads'

const store = useMetaAdsStore()

// URL de callback que o usuario copia da barra de enderecos apos autorizar.
const callbackUrl = ref('')

function onStart() {
  if (store.assistantAuthBusy) return
  void store.startAssistantAuth()
}

async function onComplete() {
  if (store.assistantAuthBusy) return
  const ok = await store.completeAssistantAuth(callbackUrl.value)
  if (ok) {
    callbackUrl.value = ''
  }
}
</script>

<template>
  <section class="ma-auth" aria-label="Conectar o assistente a Meta">
    <header class="ma-auth__head">
      <div>
        <h3 class="ma-auth__title">Conectar o assistente a Meta</h3>
        <p class="ma-auth__subtitle">
          O assistente cria/edita campanhas via MCP oficial da Meta. Faca o login uma vez para
          liberar os comandos por texto. Campanhas criadas pela IA nascem PAUSADAS.
        </p>
      </div>
      <button
        type="button"
        class="ma-auth__btn"
        :disabled="store.assistantAuthBusy"
        @click="onStart"
      >
        <span v-if="store.assistantAuthBusy" class="ma-auth__spinner" aria-hidden="true"></span>
        {{ store.assistantAuthUrl ? 'Gerar novo link' : 'Conectar a Meta' }}
      </button>
    </header>

    <p v-if="store.assistantAuthDone" class="ma-auth__ok">
      Assistente conectado a Meta. Ja pode mandar comandos no chat abaixo.
    </p>

    <div v-if="store.assistantAuthUrl" class="ma-auth__flow">
      <ol class="ma-auth__steps">
        <li>
          <a
            class="ma-auth__link"
            :href="store.assistantAuthUrl"
            target="_blank"
            rel="noopener noreferrer"
          >
            Abrir o login da Meta
          </a>
          e autorizar (escolha as contas + acesso read/write).
        </li>
        <li>
          Volte aqui e clique
          <strong>Concluir</strong>
          — na maioria das vezes finaliza sozinho. Se cair numa pagina
          <code>localhost/callback</code>
          com erro de conexao (normal), copie a URL inteira da barra de enderecos e cole abaixo
          (opcional):
        </li>
      </ol>

      <div class="ma-auth__complete">
        <input
          v-model="callbackUrl"
          type="text"
          class="ma-auth__input"
          placeholder="http://localhost:.../callback?code=...&state=..."
          :disabled="store.assistantAuthBusy"
          @keydown.enter.prevent="onComplete"
        />
        <button
          type="button"
          class="ma-auth__btn ma-auth__btn--primary"
          :disabled="store.assistantAuthBusy"
          @click="onComplete"
        >
          {{ store.assistantAuthBusy ? 'Concluindo...' : 'Concluir' }}
        </button>
      </div>
    </div>

    <p v-if="store.assistantAuthError" class="ma-auth__error">{{ store.assistantAuthError }}</p>
  </section>
</template>

<style scoped>
.ma-auth {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1.25rem 1.4rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
}

.ma-auth__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.ma-auth__title {
  font-size: 1.05rem;
  font-weight: 700;
}

.ma-auth__subtitle {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-top: 0.25rem;
  max-width: 64ch;
}

.ma-auth__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.88rem;
  font-weight: 600;
  padding: 0.55rem 1.1rem;
  border-radius: 0.55rem;
  cursor: pointer;
  border: 1px solid var(--line-strong);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  white-space: nowrap;
}

.ma-auth__btn--primary {
  border-color: transparent;
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  color: rgb(255 255 255);
}

.ma-auth__btn:disabled {
  opacity: 0.6;
  cursor: progress;
}

.ma-auth__flow {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.ma-auth__steps {
  margin: 0;
  padding-left: 1.1rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.ma-auth__steps code {
  font-size: 0.8rem;
  padding: 0.05rem 0.3rem;
  border-radius: 0.3rem;
  background: rgb(var(--surface-2) / 0.7);
  color: var(--text-main);
}

.ma-auth__link {
  color: rgb(var(--primary));
  font-weight: 600;
  text-decoration: underline;
}

.ma-auth__complete {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.ma-auth__input {
  flex: 1;
  min-width: 16rem;
  padding: 0.55rem 0.75rem;
  border-radius: 0.55rem;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-main);
  font: inherit;
  font-size: 0.85rem;
}

.ma-auth__input:focus {
  outline: none;
  border-color: rgb(var(--ring) / 0.5);
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.16);
}

.ma-auth__ok {
  font-size: 0.85rem;
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.14);
  padding: 0.55rem 0.8rem;
  border-radius: var(--radius-soft);
}

.ma-auth__error {
  font-size: 0.85rem;
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.16);
  padding: 0.55rem 0.8rem;
  border-radius: var(--radius-soft);
}

.ma-auth__spinner {
  width: 13px;
  height: 13px;
  border-radius: 50%;
  border: 2px solid rgb(var(--primary) / 0.3);
  border-top-color: rgb(var(--primary));
  animation: ma-auth-spin 0.7s linear infinite;
}

@keyframes ma-auth-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
