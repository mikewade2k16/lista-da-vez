<script setup lang="ts">
import AppDetailDialog from '~/components/ui/AppDetailDialog.vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    individualMode: boolean
    connected: boolean
    username?: string
    canOpenConnection: boolean
  }>(),
  {
    username: '',
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  'open-connection': []
}>()

const currentCapabilities = [
  'Selecionar todos os clientes ou trabalhar em um cliente individual.',
  'Validar e guardar de forma cifrada o token de um Instagram profissional.',
  'Criar rascunhos com legenda, imagem HTTPS pública e texto alternativo.',
  'Agendar a publicação automática de uma imagem na data e no horário definidos.',
  'Acompanhar fila, conteúdo publicado, cancelamentos, falhas e novas tentativas.',
  'Sincronizar visualizações, alcance, interações, curtidas, comentários, salvos e compartilhamentos.',
]

const plannedCapabilities = [
  'Receber diretamente os arquivos e horários definidos no Calendário.',
  'Devolver ao Calendário o status e um resumo dos analytics após a publicação.',
  'Dar à Crow Assistant contexto e ações seguras sobre agenda e desempenho.',
  'Trocar a conexão técnica manual por OAuth, renovação e alertas de expiração.',
  'Ampliar os formatos para vídeos, Reels e carrosséis.',
]

const publishingFlow = [
  { label: 'Cliente', icon: 'i-lucide-building-2' },
  { label: 'Instagram', icon: 'i-lucide-instagram' },
  { label: 'Conteúdo e hora', icon: 'i-lucide-calendar-clock' },
  { label: 'Publicação', icon: 'i-lucide-send' },
  { label: 'Analytics', icon: 'i-lucide-chart-no-axes-column' },
]

const connectionStatus = computed(() => {
  if (!props.individualMode) {
    return {
      label: 'Visão geral',
      detail: 'Selecione um cliente para configurar uma conexão.',
      tone: 'neutral',
      icon: 'i-lucide-users',
    }
  }
  if (props.connected) {
    const username = props.username.trim()
    return {
      label: 'Canal conectado',
      detail: username
        ? `O perfil @${username} está pronto para receber agendamentos.`
        : 'O Instagram deste cliente está pronto para receber agendamentos.',
      tone: 'success',
      icon: 'i-lucide-circle-check-big',
    }
  }
  return {
    label: 'Conexão pendente',
    detail: 'Ainda falta validar o Instagram profissional deste cliente.',
    tone: 'warning',
    icon: 'i-lucide-plug-zap',
  }
})

function close(): void {
  emit('update:open', false)
}

function openConnection(): void {
  close()
  emit('open-connection')
}
</script>

<template>
  <AppDetailDialog
    :model-value="open"
    title="Como funciona o Agendamento de Postagens"
    subtitle="O que este módulo resolve, o que já está operacional e o que ainda faz parte da integração planejada."
    width="min(64rem, calc(100vw - 2rem))"
    @update:model-value="emit('update:open', $event)"
  >
    <div class="sp-about">
      <section class="sp-about__intro">
        <div class="sp-about__intro-copy">
          <span class="sp-about__eyebrow">Objetivo do módulo</span>
          <h3>Transformar o planejamento de conteúdo em publicação mensurável.</h3>
          <p>
            Cada conexão pertence a um cliente da plataforma. O módulo organiza o conteúdo, publica
            no horário programado e concentra os resultados para acompanhamento.
          </p>
        </div>

        <div class="sp-about__status" :class="`sp-about__status--${connectionStatus.tone}`">
          <UIcon :name="connectionStatus.icon" aria-hidden="true" />
          <span>
            <strong>{{ connectionStatus.label }}</strong>
            <small>{{ connectionStatus.detail }}</small>
          </span>
        </div>
      </section>

      <section class="sp-about__flow" aria-labelledby="sp-about-flow-title">
        <div class="sp-about__section-heading">
          <span class="sp-about__section-icon">
            <UIcon name="i-lucide-workflow" aria-hidden="true" />
          </span>
          <div>
            <h3 id="sp-about-flow-title">Fluxo operacional</h3>
            <p>Depois que uma publicação é configurada, a automação executa a etapa de envio.</p>
          </div>
        </div>

        <ol class="sp-about__steps">
          <li v-for="(step, index) in publishingFlow" :key="step.label">
            <span class="sp-about__step-number">{{ index + 1 }}</span>
            <UIcon :name="step.icon" aria-hidden="true" />
            <strong>{{ step.label }}</strong>
          </li>
        </ol>
      </section>

      <div class="sp-about__columns">
        <section class="sp-about__panel" aria-labelledby="sp-about-now-title">
          <div class="sp-about__section-heading">
            <span class="sp-about__section-icon sp-about__section-icon--success">
              <UIcon name="i-lucide-badge-check" aria-hidden="true" />
            </span>
            <div>
              <span class="sp-about__kicker">Operacional agora</span>
              <h3 id="sp-about-now-title">O que já funciona hoje</h3>
            </div>
          </div>
          <ul class="sp-about__list">
            <li v-for="item in currentCapabilities" :key="item">
              <UIcon name="i-lucide-check" aria-hidden="true" />
              <span>{{ item }}</span>
            </li>
          </ul>
        </section>

        <section class="sp-about__panel" aria-labelledby="sp-about-next-title">
          <div class="sp-about__section-heading">
            <span class="sp-about__section-icon sp-about__section-icon--planned">
              <UIcon name="i-lucide-route" aria-hidden="true" />
            </span>
            <div>
              <span class="sp-about__kicker">Visão completa</span>
              <h3 id="sp-about-next-title">O que ele deverá fazer</h3>
            </div>
          </div>
          <ul class="sp-about__list sp-about__list--planned">
            <li v-for="item in plannedCapabilities" :key="item">
              <UIcon name="i-lucide-clock-3" aria-hidden="true" />
              <span>{{ item }}</span>
            </li>
          </ul>
        </section>
      </div>

      <section class="sp-about__answer" aria-labelledby="sp-about-ready-title">
        <div class="sp-about__answer-icon" aria-hidden="true">
          <UIcon name="i-lucide-circle-help" />
        </div>
        <div class="sp-about__answer-copy">
          <span class="sp-about__kicker">Resposta direta</span>
          <h3 id="sp-about-ready-title">Se eu conectar o Instagram, já fica tudo pronto?</h3>
          <p>
            <strong>O canal fica pronto, mas cada postagem ainda precisa ser configurada.</strong>
            Você informa uma imagem HTTPS pública, legenda, data e horário. Depois de agendar, o
            módulo coloca o item na fila, publica automaticamente e permite sincronizar os
            resultados.
          </p>
        </div>
        <div class="sp-about__requirements">
          <span>
            <UIcon name="i-lucide-instagram" aria-hidden="true" />
            Conta Business ou Creator
          </span>
          <span>
            <UIcon name="i-lucide-key-round" aria-hidden="true" />
            Token e permissões válidos
          </span>
          <span>
            <UIcon name="i-lucide-image" aria-hidden="true" />
            Imagem em URL HTTPS pública
          </span>
          <span>
            <UIcon name="i-lucide-clock-3" aria-hidden="true" />
            Data e horário definidos
          </span>
        </div>
      </section>

      <section class="sp-about__after" aria-labelledby="sp-about-after-title">
        <div class="sp-about__section-heading">
          <span class="sp-about__section-icon">
            <UIcon name="i-lucide-rocket" aria-hidden="true" />
          </span>
          <div>
            <h3 id="sp-about-after-title">Com a conta conectada você poderá</h3>
            <p>Operar a publicação individual daquele cliente dentro desta página.</p>
          </div>
        </div>
        <div class="sp-about__after-grid">
          <span>
            <UIcon name="i-lucide-file-pen-line" aria-hidden="true" />
            Preparar rascunhos
          </span>
          <span>
            <UIcon name="i-lucide-calendar-check-2" aria-hidden="true" />
            Agendar data e hora
          </span>
          <span>
            <UIcon name="i-lucide-send" aria-hidden="true" />
            Publicar automaticamente
          </span>
          <span>
            <UIcon name="i-lucide-rotate-ccw" aria-hidden="true" />
            Cancelar ou tentar novamente
          </span>
          <span>
            <UIcon name="i-lucide-activity" aria-hidden="true" />
            Acompanhar o status
          </span>
          <span>
            <UIcon name="i-lucide-chart-spline" aria-hidden="true" />
            Medir o desempenho
          </span>
        </div>
      </section>

      <aside class="sp-about__pilot">
        <UIcon name="i-lucide-flask-conical" aria-hidden="true" />
        <p>
          <strong>Estado atual do piloto:</strong>
          publicação de uma imagem por vez e conexão manual por token. A integração com Calendário,
          Crow Assistant, OAuth e novos formatos ainda não está ativa. O token também não é renovado
          automaticamente nesta etapa.
        </p>
      </aside>

      <footer class="sp-about__footer">
        <UButton type="button" color="neutral" variant="ghost" label="Fechar" @click="close" />
        <UButton
          v-if="individualMode && canOpenConnection && !connected"
          type="button"
          color="primary"
          icon="i-lucide-plug-zap"
          label="Ir para conexão"
          @click="openConnection"
        />
        <UButton v-else type="button" color="primary" label="Entendi" @click="close" />
      </footer>
    </div>
  </AppDetailDialog>
</template>

<style scoped src="./SocialPublishingAboutModal.css"></style>
