<script setup lang="ts">
import { storeToRefs } from 'pinia'

import type { StorageSettingsInput } from '~/types/storage'
import AdminPageHeader from '../../../../layers/core/components/admin/AdminPageHeader.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import StorageLimitsDialog from '~/components/admin/storage/StorageLimitsDialog.vue'
import { useAuthStore } from '~/stores/auth'
import { useStorageStore } from '~/stores/storage'

const auth = useAuthStore()
const storageStore = useStorageStore()
const { status, loading, saving, checking, errorMessage } = storeToRefs(storageStore)
const limitsDialogOpen = ref(false)

const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
const numberFormatter = new Intl.NumberFormat('pt-BR')
const decimalFormatter = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 2 })

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes >= 1_000_000_000) return `${decimalFormatter.format(bytes / 1_000_000_000)} GB`
  if (bytes >= 1024 * 1024) return `${decimalFormatter.format(bytes / (1024 * 1024))} MiB`
  if (bytes >= 1024) return `${decimalFormatter.format(bytes / 1024)} KiB`
  return `${numberFormatter.format(bytes)} B`
}

function percentage(value: number, limit: number) {
  if (!Number.isFinite(value) || !Number.isFinite(limit) || limit <= 0) return 0
  return Math.min(100, Math.max(0, (value / limit) * 100))
}

const storageUsed = computed(
  () =>
    (status.value?.cloudUsage.storedBytes || 0) +
    (status.value?.cloudUsage.metadataBytes || 0) +
    (status.value?.usage.uploadedBytes || 0) +
    (status.value?.usage.pendingBytes || 0),
)
const classAUsed = computed(
  () => (status.value?.cloudUsage.classARequests || 0) + (status.value?.usage.classARequests || 0),
)
const classBUsed = computed(
  () => (status.value?.cloudUsage.classBRequests || 0) + (status.value?.usage.classBRequests || 0),
)

async function saveLimits(input: StorageSettingsInput) {
  await storageStore.saveSettings(input)
}

async function validateConnection() {
  await storageStore.checkConnection()
}

async function setR2Uploads(enabled: boolean) {
  if (!status.value) return
  const settings = status.value.settings
  if (settings.uploadsEnabled === enabled) return
  await storageStore.saveSettings(
    {
      uploadsEnabled: enabled,
      billingCycleDay: settings.billingCycleDay,
      storageLimitBytes: settings.storageLimitBytes,
      classALimit: settings.classALimit,
      classBLimit: settings.classBLimit,
      imageMaxBytes: settings.imageMaxBytes,
      videoMaxBytes: settings.videoMaxBytes,
      audioMaxBytes: settings.audioMaxBytes,
      documentMaxBytes: settings.documentMaxBytes,
    },
    `Upload para R2 ${enabled ? 'ativado' : 'desativado'}.`,
  )
}

async function refreshAfterUpload() {
  await storageStore.load(true)
}

onMounted(() => {
  if (isPlatformAdmin.value) void storageStore.load(true)
})
</script>

<template>
  <section class="storage-workspace">
    <AdminPageHeader
      eyebrow="Plataforma"
      title="Storage R2"
      description="Controle central dos uploads da plataforma, com reserva autoritativa de uso antes de cada operacao e limites abaixo da franquia gratuita do Cloudflare R2."
    />

    <UAlert
      v-if="!isPlatformAdmin"
      color="error"
      variant="soft"
      icon="i-lucide-shield-alert"
      title="Acesso restrito"
      description="Esta area e exclusiva para administradores da plataforma."
    />

    <template v-else>
      <div class="storage-workspace__toolbar">
        <div v-if="status" class="storage-workspace__provider">
          <span
            class="storage-workspace__status-dot"
            :class="{
              'storage-workspace__status-dot--ready': status.enabled && status.initialized,
            }"
          ></span>
          <span>
            {{
              status.enabled
                ? status.initialized
                  ? 'R2 conectado'
                  : 'R2 aguardando validacao'
                : 'R2 desativado no servidor'
            }}
          </span>
          <span v-if="status.bucket" class="storage-workspace__bucket">{{ status.bucket }}</span>
        </div>
        <div class="storage-workspace__actions">
          <div v-if="status" class="storage-workspace__r2-switch">
            <AppToggleSwitch
              :model-value="status.settings.uploadsEnabled"
              :label="`Upload para R2: ${status.settings.uploadsEnabled ? 'ativado' : 'desativado'}`"
              compact
              :disabled="
                saving ||
                (!status.settings.uploadsEnabled &&
                  (!status.enabled || !status.cloudUsage.configured))
              "
              @update:model-value="setR2Uploads"
            />
            <UIcon v-if="saving" name="i-lucide-loader-circle" class="animate-spin" />
          </div>
          <UButton
            label="Validar conexao"
            icon="i-lucide-plug-zap"
            color="neutral"
            variant="soft"
            :loading="checking"
            :disabled="!status?.enabled || checking"
            @click="validateConnection"
          />
          <UButton
            label="Configurar storage"
            icon="i-lucide-sliders-horizontal"
            color="primary"
            :disabled="!status"
            @click="limitsDialogOpen = true"
          />
        </div>
      </div>

      <UAlert
        v-if="errorMessage"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        title="Nao foi possivel concluir a operacao"
        :description="errorMessage"
      />

      <div v-if="loading && !status" class="storage-workspace__state">
        <UIcon name="i-lucide-loader-circle" class="storage-workspace__spinner" />
        Carregando limites e consumo...
      </div>

      <template v-else-if="status">
        <div class="storage-workspace__metrics">
          <article class="storage-workspace__card">
            <div class="storage-workspace__card-heading">
              <span>Armazenamento</span>
              <strong>
                {{
                  decimalFormatter.format(
                    percentage(storageUsed, status.settings.storageLimitBytes),
                  )
                }}%
              </strong>
            </div>
            <p class="storage-workspace__value">
              {{ status.cloudUsage.available ? formatBytes(status.cloudUsage.storedBytes) : '—' }}
              <span>de {{ formatBytes(status.settings.storageLimitBytes) }}</span>
            </p>
            <progress
              class="storage-workspace__progress"
              :value="percentage(storageUsed, status.settings.storageLimitBytes)"
              max="100"
            ></progress>
            <p class="storage-workspace__detail">
              Proteção conservadora: {{ formatBytes(storageUsed) }} · inclui
              {{ formatBytes(status.cloudUsage.metadataBytes) }} de metadados e
              {{ formatBytes(status.usage.uploadedBytes) }} confirmados pelo Omni neste ciclo, além
              de {{ formatBytes(status.usage.pendingBytes) }} pendentes.
            </p>
          </article>

          <article class="storage-workspace__card">
            <div class="storage-workspace__card-heading">
              <span>Operacoes Classe A</span>
              <strong>
                {{ decimalFormatter.format(percentage(classAUsed, status.settings.classALimit)) }}%
              </strong>
            </div>
            <p class="storage-workspace__value">
              {{ status.cloudUsage.available ? numberFormatter.format(classAUsed) : '—' }}
              <span>de {{ numberFormatter.format(status.settings.classALimit) }}</span>
            </p>
            <progress
              class="storage-workspace__progress"
              :value="percentage(classAUsed, status.settings.classALimit)"
              max="100"
            ></progress>
            <p class="storage-workspace__detail">
              Proteção conservadora: conta Cloudflare + reservas Omni do ciclo.
            </p>
          </article>

          <article class="storage-workspace__card">
            <div class="storage-workspace__card-heading">
              <span>Operacoes Classe B</span>
              <strong>
                {{ decimalFormatter.format(percentage(classBUsed, status.settings.classBLimit)) }}%
              </strong>
            </div>
            <p class="storage-workspace__value">
              {{ status.cloudUsage.available ? numberFormatter.format(classBUsed) : '—' }}
              <span>de {{ numberFormatter.format(status.settings.classBLimit) }}</span>
            </p>
            <progress
              class="storage-workspace__progress"
              :value="percentage(classBUsed, status.settings.classBLimit)"
              max="100"
            ></progress>
            <p class="storage-workspace__detail">
              Proteção conservadora: conta Cloudflare + reservas Omni do ciclo.
            </p>
          </article>

          <article class="storage-workspace__card storage-workspace__card--file">
            <div class="storage-workspace__card-heading"><span>Limites por tipo</span></div>
            <div class="storage-workspace__type-limits">
              <span>
                Imagem
                <strong>{{ formatBytes(status.settings.imageMaxBytes) }}</strong>
              </span>
              <span>
                Video
                <strong>{{ formatBytes(status.settings.videoMaxBytes) }}</strong>
              </span>
              <span>
                Audio
                <strong>{{ formatBytes(status.settings.audioMaxBytes) }}</strong>
              </span>
              <span>
                Documento
                <strong>{{ formatBytes(status.settings.documentMaxBytes) }}</strong>
              </span>
            </div>
            <p class="storage-workspace__detail">Validacao por MIME detectado antes do envio.</p>
          </article>
        </div>

        <div class="storage-workspace__notices">
          <UAlert
            v-if="!status.cloudUsage.configured"
            color="warning"
            variant="soft"
            icon="i-lucide-chart-no-axes-combined"
            title="Metricas reais ainda sem token"
            description="Preencha R2_ANALYTICS_API_TOKEN no ambiente da API. Enquanto isso, ativar novos uploads R2 fica bloqueado; o modo local continua funcionando."
          />
          <UAlert
            v-else-if="!status.cloudUsage.available"
            color="error"
            variant="soft"
            icon="i-lucide-cloud-alert"
            title="Metricas Cloudflare indisponiveis"
            description="O painel nao substitui os valores reais por estimativas locais e novos uploads R2 ficam bloqueados por seguranca."
          />
          <UAlert
            v-if="!status.enabled"
            color="warning"
            variant="soft"
            icon="i-lucide-key-round"
            title="Credenciais ainda nao configuradas"
            description="Os limites ja podem ser editados. Para conectar, configure R2_ENABLED, conta, bucket dedicado e token no servidor. Segredos nunca sao gravados pelo painel."
          />
          <UAlert
            v-else-if="!status.initialized"
            color="warning"
            variant="soft"
            icon="i-lucide-cloud-cog"
            title="Valide o bucket dedicado"
            description="A primeira validacao confirma a credencial e exige que o bucket esteja vazio antes de registrar o provider no PostgreSQL."
          />
          <UAlert
            color="neutral"
            variant="soft"
            icon="i-lucide-info"
            title="Escopo da protecao"
            description="Os cartões usam a conta Cloudflare inteira, incluindo local, producao e outros buckets. Reservas pendentes deste banco sao somadas ao armazenamento antes de autorizar outro upload."
          />
        </div>

        <p class="storage-workspace__updated">
          Fonte: {{ status.cloudUsage.available ? 'Cloudflare account-wide' : 'indisponivel' }} ·
          sincronizado
          {{
            status.cloudUsage.fetchedAt
              ? new Date(status.cloudUsage.fetchedAt).toLocaleString('pt-BR')
              : '—'
          }}. Janela consultada:
          {{
            status.cloudUsage.windowStart
              ? new Date(status.cloudUsage.windowStart).toLocaleString('pt-BR')
              : '—'
          }}
          até agora. Ciclo inicia no dia {{ status.settings.billingCycleDay }}. Metadados
          protegidos: {{ formatBytes(status.cloudUsage.metadataBytes) }}. Última alteração dos
          limites: {{ new Date(status.settings.updatedAt).toLocaleString('pt-BR') }}.
        </p>

        <StorageLimitsDialog
          :open="limitsDialogOpen"
          :settings="status.settings"
          :saving="saving"
          :enabled="status.enabled"
          :initialized="status.initialized"
          @update:open="limitsDialogOpen = $event"
          @save="saveLimits"
          @uploaded="refreshAfterUpload"
          @toggle-uploads="setR2Uploads"
        />
      </template>
    </template>
  </section>
</template>

<style scoped>
.storage-workspace {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  align-content: start;
  gap: 1rem;
  padding: 1rem 1.2rem 2rem;
}

.storage-workspace__toolbar,
.storage-workspace__provider,
.storage-workspace__actions,
.storage-workspace__card-heading,
.storage-workspace__state {
  display: flex;
  align-items: center;
}

.storage-workspace__toolbar {
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.storage-workspace__provider,
.storage-workspace__actions {
  gap: 0.55rem;
}

.storage-workspace__r2-switch {
  min-height: 2.25rem;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0 0.7rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
}

.storage-workspace__r2-switch > svg,
.storage-workspace__r2-switch > .iconify {
  width: 0.9rem;
  height: 0.9rem;
  color: rgb(var(--muted));
}

.storage-workspace__provider {
  min-height: 2.25rem;
  font-size: 0.82rem;
  color: rgb(var(--text));
}

.storage-workspace__bucket {
  padding-left: 0.55rem;
  border-left: 1px solid rgb(var(--border));
  color: rgb(var(--muted));
}

.storage-workspace__status-dot {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 999px;
  background: rgb(var(--warning));
  box-shadow: 0 0 0 0.2rem rgb(var(--warning) / 0.15);
}

.storage-workspace__status-dot--ready {
  background: rgb(var(--success));
  box-shadow: 0 0 0 0.2rem rgb(var(--success) / 0.15);
}

.storage-workspace__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.85rem;
}

.storage-workspace__card {
  min-width: 0;
  display: grid;
  gap: 0.7rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-card);
}

.storage-workspace__card-heading {
  justify-content: space-between;
  gap: 0.5rem;
  font-size: 0.74rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: rgb(var(--muted));
}

.storage-workspace__card-heading strong {
  color: rgb(var(--primary));
}

.storage-workspace__value,
.storage-workspace__detail,
.storage-workspace__updated {
  margin: 0;
}

.storage-workspace__value {
  font-size: 1.25rem;
  font-weight: 750;
  color: rgb(var(--text));
}

.storage-workspace__value span,
.storage-workspace__detail,
.storage-workspace__updated {
  font-size: 0.76rem;
  font-weight: 400;
  color: rgb(var(--muted));
}

.storage-workspace__progress {
  width: 100%;
  height: 0.45rem;
  overflow: hidden;
  border: 0;
  border-radius: 999px;
  background: rgb(var(--surface-2));
}

.storage-workspace__progress::-webkit-progress-bar {
  background: rgb(var(--surface-2));
}

.storage-workspace__progress::-webkit-progress-value {
  border-radius: 999px;
  background: rgb(var(--primary));
}

.storage-workspace__progress::-moz-progress-bar {
  border-radius: 999px;
  background: rgb(var(--primary));
}

.storage-workspace__notices {
  display: grid;
  gap: 0.75rem;
}

.storage-workspace__type-limits {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.4rem 0.7rem;
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.storage-workspace__type-limits span {
  display: flex;
  justify-content: space-between;
  gap: 0.4rem;
}

.storage-workspace__type-limits strong {
  color: rgb(var(--text));
}

.storage-workspace__state {
  min-height: 10rem;
  justify-content: center;
  gap: 0.5rem;
  color: rgb(var(--muted));
}

.storage-workspace__spinner {
  animation: storage-spin 0.8s linear infinite;
}

@keyframes storage-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1180px) {
  .storage-workspace__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 680px) {
  .storage-workspace {
    padding-inline: 0.8rem;
  }

  .storage-workspace__metrics {
    grid-template-columns: minmax(0, 1fr);
  }

  .storage-workspace__actions,
  .storage-workspace__actions > * {
    width: 100%;
  }
}
</style>
