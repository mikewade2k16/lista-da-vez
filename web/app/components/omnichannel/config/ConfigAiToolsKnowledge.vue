<script setup lang="ts">
import { toRef } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import ConfigAiToolRuns from '~/components/omnichannel/config/ConfigAiToolRuns.vue'
import { useOmnichannelToolsKnowledge } from '~/composables/omnichannel/useOmnichannelToolsKnowledge'

const props = defineProps<{ canManage: boolean; canAudit: boolean }>()
const {
  agents,
  bases,
  toolBindings,
  toolRuns,
  approvals,
  knowledgeBindings,
  documents,
  selectedAgentId,
  selectedBaseId,
  loading,
  busy,
  newTool,
  newBase,
  newDocument,
  editingDocumentId,
  chunksDraft,
  selectedAgent,
  selectedBase,
  parseChunksDraft,
  createTool,
  toggleTool,
  disableTool,
  refreshToolRuns,
  decideToolApproval,
  createBase,
  toggleBase,
  createDocument,
  publishDocument,
  startChunksEdit,
  importKnowledgeFile,
  saveChunks,
  createKnowledgeBinding,
  toggleKnowledgeBinding,
  disableKnowledgeBinding,
} = useOmnichannelToolsKnowledge(toRef(props, 'canManage'), toRef(props, 'canAudit'))
</script>

<template>
  <div class="cfg-tab tools-config">
    <p class="cfg-tab__lead">
      Tools e conhecimento são orquestrados pelo n8n; autorização, execução, evidência e auditoria
      permanecem no Go. Sem registry explícito, nenhuma tool é simulada.
    </p>
    <p v-if="loading" class="cfg-tab__loading">Carregando…</p>
    <template v-else>
      <section class="tools-card">
        <div class="tools-card__head">
          <strong>Agente e base selecionados</strong>
          <span>configuração por conta</span>
        </div>
        <div class="tools-grid">
          <label class="cfg-field">
            <span class="cfg-field__label">Agente</span>
            <select v-model="selectedAgentId" class="cfg-input" :disabled="!canManage">
              <option v-for="agent in agents" :key="agent.id" :value="agent.id">
                {{ agent.name }}
              </option>
            </select>
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Base de conhecimento</span>
            <select v-model="selectedBaseId" class="cfg-input" :disabled="!canManage">
              <option v-for="base in bases" :key="base.id" :value="base.id">{{ base.name }}</option>
            </select>
          </label>
        </div>
        <p v-if="!selectedAgent" class="tools-muted">
          Crie um agente na aba Agente IA para configurar tools.
        </p>
      </section>

      <section class="tools-card">
        <div class="tools-card__head">
          <strong>Bindings de tools</strong>
          <span>{{ toolBindings.length }} configurados</span>
        </div>
        <div class="tools-grid tools-grid--new">
          <input
            v-model="newTool.toolId"
            class="cfg-input"
            placeholder="tool_id lógico (ex.: catalog.search)"
            :disabled="!canManage"
          />
          <select v-model="newTool.mode" class="cfg-input" :disabled="!canManage">
            <option value="read">leitura</option>
            <option value="propose_write">proposta de escrita</option>
            <option value="approved_write">escrita aprovada</option>
          </select>
          <input
            v-model="newTool.operations"
            class="cfg-input"
            placeholder="operações, separadas por vírgula"
            :disabled="!canManage"
          />
          <AppPanelButton
            :disabled="!canManage || busy || !newTool.toolId.trim()"
            @click="createTool"
          >
            Adicionar
          </AppPanelButton>
        </div>
        <div v-for="binding in toolBindings" :key="binding.id" class="tools-row">
          <div>
            <strong>{{ binding.toolId }}</strong>
            <small>
              {{ binding.mode }} · {{ binding.allowedOperations.join(', ') || 'sem operação' }} ·
              timeout {{ binding.timeoutMs }}ms · máx. {{ binding.maxCallsPerDispatch }}
            </small>
          </div>
          <span class="tools-status">{{ binding.isEnabled ? 'habilitada' : 'desabilitada' }}</span>
          <AppPanelButton
            variant="ghost"
            :disabled="!canManage || busy"
            @click="toggleTool(binding)"
          >
            {{ binding.isEnabled ? 'Desabilitar' : 'Habilitar' }}
          </AppPanelButton>
          <AppPanelButton
            variant="danger"
            :disabled="!canManage || busy"
            @click="disableTool(binding)"
          >
            Remover
          </AppPanelButton>
        </div>
        <p v-if="!toolBindings.length" class="tools-muted">Nenhum binding configurado.</p>
      </section>

      <ConfigAiToolRuns
        :runs="toolRuns"
        :approvals="approvals"
        :can-manage="canManage"
        :can-audit="canAudit"
        :busy="busy"
        @refresh="refreshToolRuns"
        @approve="decideToolApproval($event, true)"
        @reject="decideToolApproval($event, false)"
      />

      <section class="tools-card">
        <div class="tools-card__head">
          <strong>Bases de conhecimento</strong>
          <span>{{ bases.length }} bases</span>
        </div>
        <div class="tools-grid tools-grid--new">
          <input
            v-model="newBase.name"
            class="cfg-input"
            placeholder="nome da base"
            :disabled="!canManage"
          />
          <AppToggleSwitch v-model="newBase.enabled" label="Ativa" :disabled="!canManage" compact />
          <span class="tools-muted">Bases nascem sem binding de agente.</span>
          <AppPanelButton
            :disabled="!canManage || busy || !newBase.name.trim()"
            @click="createBase"
          >
            Criar base
          </AppPanelButton>
        </div>
        <div v-for="base in bases" :key="base.id" class="tools-row tools-row--base">
          <div>
            <strong>{{ base.name }}</strong>
            <small>{{ base.isEnabled ? 'publicável para busca' : 'desabilitada' }}</small>
          </div>
          <AppPanelButton variant="ghost" :disabled="!canManage || busy" @click="toggleBase(base)">
            {{ base.isEnabled ? 'Desabilitar' : 'Habilitar' }}
          </AppPanelButton>
        </div>
        <div v-if="selectedBase" class="documents">
          <div class="tools-card__head">
            <strong>Documentos de {{ selectedBase.name }}</strong>
            <span>{{ documents.length }} versões</span>
          </div>
          <div class="tools-grid tools-grid--new">
            <input
              v-model="newDocument.title"
              class="cfg-input"
              placeholder="título"
              :disabled="!canManage"
            />
            <input
              v-model="newDocument.sourceRef"
              class="cfg-input"
              placeholder="source_ref permitido"
              :disabled="!canManage"
            />
            <input
              v-model="newDocument.checksum"
              class="cfg-input"
              placeholder="checksum"
              :disabled="!canManage"
            />
            <label class="file-picker">
              <span>Importar texto (máx. 2 MB)</span>
              <input
                type="file"
                accept=".txt,.md,.csv,.json,text/plain,text/markdown,application/json"
                :disabled="!canManage || busy"
                @change="
                  importKnowledgeFile(($event.target as HTMLInputElement).files?.[0] || null)
                "
              />
            </label>
            <AppPanelButton
              :disabled="
                !canManage || busy || !newDocument.sourceRef.trim() || !newDocument.checksum.trim()
              "
              @click="createDocument"
            >
              Adicionar
            </AppPanelButton>
          </div>
          <div v-for="document in documents" :key="document.id" class="tools-row">
            <div>
              <strong>{{ document.title || document.sourceRef }}</strong>
              <small>
                {{ document.status }} · {{ document.chunkCount }} chunks · v{{ document.version
                }}{{ document.error ? ` · ${document.error}` : '' }}
              </small>
            </div>
            <AppPanelButton
              variant="ghost"
              :disabled="!canManage || busy"
              @click="startChunksEdit(document)"
            >
              Editar chunks
            </AppPanelButton>
            <AppPanelButton
              variant="ghost"
              :disabled="
                !canManage || busy || document.chunkCount < 1 || document.status === 'published'
              "
              @click="publishDocument(document)"
            >
              Publicar
            </AppPanelButton>
          </div>
          <div v-if="editingDocumentId" class="chunks-editor">
            <label class="cfg-field">
              <span class="cfg-field__label">Texto dos chunks</span>
              <textarea
                v-model="chunksDraft"
                class="cfg-input chunks-editor__input"
                rows="8"
                :disabled="!canManage || busy"
                placeholder="Um bloco por chunk. Separe blocos com uma linha contendo ---"
              ></textarea>
            </label>
            <div class="chunks-editor__foot">
              <span class="tools-muted">
                {{ parseChunksDraft().length }} blocos · validado e limitado pelo Go
              </span>
              <AppPanelButton
                :disabled="!canManage || busy || !parseChunksDraft().length"
                @click="saveChunks(documents.find((item) => item.id === editingDocumentId)!)"
              >
                Salvar chunks
              </AppPanelButton>
            </div>
          </div>
        </div>
      </section>

      <section class="tools-card">
        <div class="tools-card__head">
          <strong>Bindings de conhecimento do agente</strong>
          <span>{{ knowledgeBindings.length }} vinculados</span>
        </div>
        <div class="tools-grid tools-grid--new">
          <span class="tools-muted">
            Somente bases habilitadas e publicadas aparecem nas consultas da IA.
          </span>
          <AppPanelButton
            :disabled="!canManage || busy || !selectedAgentId || !selectedBaseId"
            @click="createKnowledgeBinding"
          >
            Vincular base selecionada
          </AppPanelButton>
        </div>
        <div v-for="binding in knowledgeBindings" :key="binding.id" class="tools-row">
          <div>
            <strong>{{ binding.baseName }}</strong>
            <small>topK {{ binding.topK }} · score mínimo {{ binding.minScore }}</small>
          </div>
          <span class="tools-status">{{ binding.isEnabled ? 'habilitado' : 'desabilitado' }}</span>
          <AppPanelButton
            variant="ghost"
            :disabled="!canManage || busy"
            @click="toggleKnowledgeBinding(binding)"
          >
            {{ binding.isEnabled ? 'Desabilitar' : 'Habilitar' }}
          </AppPanelButton>
          <AppPanelButton
            variant="danger"
            :disabled="!canManage || busy"
            @click="disableKnowledgeBinding(binding)"
          >
            Remover
          </AppPanelButton>
        </div>
        <p v-if="!knowledgeBindings.length" class="tools-muted">
          Nenhuma base vinculada ao agente.
        </p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.tools-config {
  display: grid;
  gap: 0.75rem;
}
.tools-card {
  display: grid;
  gap: 0.75rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: 16px;
  background: rgb(var(--surface-2) / 0.32);
}
.tools-card__head {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--text-main);
  font-size: 0.82rem;
}
.tools-card__head span,
.tools-muted,
.tools-row small {
  color: var(--text-muted);
  font-size: 0.74rem;
}
.tools-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
  align-items: center;
}
.tools-grid--new {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.cfg-input {
  min-height: 36px;
  padding: 0 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.8rem;
}
.tools-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto auto;
  gap: 0.5rem;
  align-items: center;
  padding-top: 0.6rem;
  border-top: 1px solid var(--line-soft);
}
.tools-row--base {
  grid-template-columns: minmax(0, 1fr) auto;
}
.tools-row > div {
  display: grid;
  gap: 0.15rem;
  min-width: 0;
}
.tools-row small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tools-status {
  color: rgb(var(--primary));
  font-size: 0.72rem;
  white-space: nowrap;
}
.documents {
  display: grid;
  gap: 0.6rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--line-soft);
}
.chunks-editor {
  display: grid;
  gap: 0.5rem;
  padding-top: 0.75rem;
  border-top: 1px dashed var(--line-soft);
}
.chunks-editor__input {
  width: 100%;
  min-height: 10rem;
  resize: vertical;
  padding: 0.7rem;
  line-height: 1.45;
}
.chunks-editor__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.file-picker {
  display: grid;
  gap: 0.2rem;
  color: var(--text-muted);
  font-size: 0.74rem;
}
.file-picker input {
  max-width: 100%;
  color: var(--text-muted);
  font-size: 0.72rem;
}
@media (max-width: 780px) {
  .tools-grid,
  .tools-grid--new {
    grid-template-columns: 1fr;
  }
  .tools-row,
  .tools-row--base {
    grid-template-columns: 1fr;
    align-items: start;
  }
}
</style>
