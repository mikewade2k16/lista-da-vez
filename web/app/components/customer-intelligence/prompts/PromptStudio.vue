<script setup lang="ts">
import { ref } from 'vue'
import CustomerIntelligenceStatus from '~/components/customer-intelligence/CustomerIntelligenceStatus.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { CANONICAL_PROCESS_KEYS } from '~/domain/customer-intelligence/prompt-types'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'
import { usePromptStudio } from '~/composables/customer-intelligence/usePromptStudio'
import PromptEditor from './PromptEditor.vue'
import PromptEvaluationPanel from './PromptEvaluationPanel.vue'
import PromptLayersPanel from './PromptLayersPanel.vue'
import PromptLegacyMediaNotice from './PromptLegacyMediaNotice.vue'
import PromptPipelinePanel from './PromptPipelinePanel.vue'
import PromptProcessList from './PromptProcessList.vue'
import PromptRolloutPanel from './PromptRolloutPanel.vue'
import PromptVersionsPanel from './PromptVersionsPanel.vue'

const access = useCustomerIntelligenceAccess()
const studio = usePromptStudio()
const pendingProcessKey = ref('')

const catalogWarning = computed(() => {
  const keys = new Set(studio.catalog.value.processes.map((process) => process.processKey))
  const missing = CANONICAL_PROCESS_KEYS.filter((key) => !keys.has(key))
  return missing.length ? `Catalogo incompleto: ${missing.join(', ')}` : ''
})

async function chooseProcess(processKey: string): Promise<void> {
  const changed = await studio.selectProcess(processKey)
  pendingProcessKey.value = changed ? '' : processKey
}

async function discardAndSwitch(): Promise<void> {
  const target = pendingProcessKey.value
  studio.discardChanges()
  pendingProcessKey.value = ''
  if (target) await studio.selectProcess(target, true)
}

function updatePolicyValue(key: string, value: string | number | boolean): void {
  studio.editorConfig.value = { ...studio.editorConfig.value, [key]: value }
}
</script>

<template>
  <CustomerIntelligenceStatus
    v-if="studio.loading.value"
    title="Carregando Prompt Studio"
    loading
  />
  <CustomerIntelligenceStatus
    v-else-if="studio.error.value && !studio.catalog.value.processes.length"
    title="Prompt Studio indisponivel"
    :error="studio.error.value"
  />
  <div v-else class="prompt-studio">
    <div v-if="catalogWarning" class="prompt-studio__warning">{{ catalogWarning }}</div>
    <div class="prompt-studio__warning">
      Publicacao e rollback usam binding versionado e agente publicado. Pipelines, corpus de
      evaluations e compatibilidade legada continuam indisponiveis neste painel.
    </div>
    <div v-if="pendingProcessKey" class="prompt-studio__warning">
      Ha alteracoes nao salvas.
      <button type="button" @click="discardAndSwitch">Descartar e trocar processo</button>
    </div>

    <PromptProcessList
      :processes="studio.catalog.value.processes"
      :selected-process-key="studio.selectedProcessKey.value"
      @select="chooseProcess"
    />

    <main v-if="studio.processView.value" class="prompt-studio__main">
      <header class="prompt-studio__header">
        <div>
          <small>{{ studio.processView.value.process.processKey }}</small>
          <h2>{{ studio.processView.value.process.name }}</h2>
          <p>{{ studio.processView.value.process.description }}</p>
        </div>
        <span>
          {{ studio.processView.value.process.inputSchemaVersion }} →
          {{ studio.processView.value.process.outputSchemaVersion }}
        </span>
      </header>

      <PromptEditor
        v-model="studio.editorPrompt.value"
        :variables="studio.processView.value.process.allowedVariables"
        :disabled="!access.canManagePrompts.value"
      />

      <section class="prompt-policy">
        <h3>Politica estruturada</h3>
        <p>Bounds, modelo, budget e fallback sao controles tipados e vencem o prompt.</p>
        <div class="prompt-policy__grid">
          <AppSelectField
            v-if="studio.processView.value.publishAgents.length"
            v-model="studio.selectedAgentVersionId.value"
            :options="
              studio.processView.value.publishAgents.map((item) => ({
                value: item.agentVersionId,
                label: item.label,
              }))
            "
            label="Agente publicado"
            :disabled="!access.canPublishPrompts.value || studio.saving.value"
          />
          <p v-else>Publique e habilite um agente deste cliente antes de publicar o prompt.</p>
          <template
            v-for="field in studio.processView.value.process.policySchema || []"
            :key="field.key"
          >
            <AppSelectField
              v-if="field.type === 'select'"
              :model-value="String(studio.editorConfig.value[field.key] ?? '')"
              :options="field.options || []"
              :label="field.label"
              :disabled="!access.canManagePrompts.value"
              @update:model-value="updatePolicyValue(field.key, $event)"
            />
            <label v-else class="prompt-policy__field">
              <span>{{ field.label }}</span>
              <input
                v-if="field.type !== 'boolean'"
                :value="studio.editorConfig.value[field.key] ?? ''"
                :type="field.type"
                :min="field.min"
                :max="field.max"
                :disabled="!access.canManagePrompts.value"
                @input="
                  updatePolicyValue(
                    field.key,
                    field.type === 'number'
                      ? Number(($event.target as HTMLInputElement).value)
                      : ($event.target as HTMLInputElement).value,
                  )
                "
              />
              <input
                v-else
                type="checkbox"
                :checked="Boolean(studio.editorConfig.value[field.key])"
                :disabled="!access.canManagePrompts.value"
                @change="updatePolicyValue(field.key, ($event.target as HTMLInputElement).checked)"
              />
            </label>
          </template>
        </div>
      </section>

      <div class="prompt-studio__details">
        <PromptLayersPanel :process-key="studio.processView.value.process.processKey" />
        <PromptVersionsPanel
          :versions="studio.processView.value.versions"
          :active-version-id="studio.processView.value.effectiveBinding?.activeVersionId"
        />
        <PromptEvaluationPanel :evaluations="studio.processView.value.evaluations" />
        <PromptPipelinePanel :pipelines="studio.catalog.value.pipelines" />
      </div>

      <PromptLegacyMediaNotice :capabilities="studio.catalog.value.legacyManagedCapabilities" />
      <PromptRolloutPanel
        :view="studio.processView.value"
        :dirty="studio.dirty.value"
        :saving="studio.saving.value"
        :can-manage="access.canManagePrompts.value"
        :can-publish="access.canPublishPrompts.value"
        @save="studio.saveDraft"
        @discard="studio.discardChanges"
        @validate="studio.runAction('validate')"
        @test="studio.runAction('test')"
        @publish="studio.runAction('publish')"
        @rollback="studio.rollback"
      />
    </main>
    <CustomerIntelligenceStatus
      v-else
      title="Selecione um processo"
      empty
      empty-text="O catalogo nao retornou um processo configuravel."
    />
  </div>
</template>

<style scoped>
.prompt-studio {
  display: grid;
  grid-template-columns: minmax(14rem, 18rem) minmax(0, 1fr);
  gap: 1rem;
}

.prompt-studio__main {
  display: grid;
  gap: 1rem;
  min-width: 0;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.9);
}

.prompt-studio__header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.prompt-studio__header h2,
.prompt-studio__header p {
  margin: 0.2rem 0;
}

.prompt-studio__header p,
.prompt-studio__header small,
.prompt-studio__header span,
.prompt-policy p {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.prompt-policy__grid,
.prompt-studio__details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.8rem;
}

.prompt-policy__field {
  display: grid;
  gap: 0.3rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.prompt-policy__field input:not([type='checkbox']) {
  min-height: 2.4rem;
  padding: 0 0.7rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.7rem;
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
}

.prompt-studio__warning {
  grid-column: 1 / -1;
  padding: 0.65rem;
  border-radius: 0.7rem;
  background: rgb(var(--warning) / 0.1);
  color: rgb(var(--warning));
  font-size: 0.75rem;
}

@media (max-width: 950px) {
  .prompt-studio,
  .prompt-policy__grid,
  .prompt-studio__details {
    grid-template-columns: 1fr;
  }
}
</style>
