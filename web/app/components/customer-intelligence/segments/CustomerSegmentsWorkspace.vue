<script setup lang="ts">
import { ref } from 'vue'
import SegmentBuilder from '~/components/customer-intelligence/segments/SegmentBuilder.vue'
import SegmentEvaluationPanel from '~/components/customer-intelligence/segments/SegmentEvaluationPanel.vue'
import SegmentList from '~/components/customer-intelligence/segments/SegmentList.vue'
import SegmentMaterializationsPanel from '~/components/customer-intelligence/segments/SegmentMaterializationsPanel.vue'
import SegmentVersionsPanel from '~/components/customer-intelligence/segments/SegmentVersionsPanel.vue'
import { useCustomerSegments } from '~/composables/customer-intelligence/useCustomerSegments'

const segments = useCustomerSegments()
const statusFilter = ref('')
const createName = ref('')
const createKey = ref('')

const selectedId = computed(() => segments.selected.value?.segment.id ?? '')
const busy = computed(
  () => segments.status.value === 'loading' || segments.actionStatus.value === 'loading',
)

async function createNew(): Promise<void> {
  const created = await segments.createNew(createName.value, createKey.value)
  if (!created) return
  createName.value = ''
  createKey.value = ''
}
</script>

<template>
  <div class="segments-workspace">
    <CustomerIntelligenceStatus
      v-if="segments.error.value && !segments.segments.value.length"
      title="Segmentos indisponiveis"
      :error="segments.error.value"
      @retry="segments.refresh(statusFilter)"
    />
    <template v-else>
      <section class="segments-workspace__sidebar">
        <form
          v-if="segments.access.canManageSegments.value"
          class="segment-create"
          @submit.prevent="createNew"
        >
          <h2>Novo segmento</h2>
          <input
            v-model="createName"
            type="text"
            maxlength="120"
            placeholder="Nome"
            aria-label="Nome do segmento"
          />
          <input
            v-model="createKey"
            type="text"
            maxlength="80"
            pattern="[a-z0-9-]+"
            placeholder="chave-estavel"
            aria-label="Chave do segmento"
          />
          <button type="submit" :disabled="busy || !createName || !createKey">Criar draft</button>
        </form>
        <SegmentList
          v-model:status="statusFilter"
          :items="segments.segments.value"
          :selected-id="selectedId"
          :loading="segments.status.value === 'loading'"
          @select="segments.selectSegment"
          @refresh="segments.refresh(statusFilter)"
        />
      </section>

      <main class="segments-workspace__main">
        <CustomerIntelligenceStatus
          v-if="!segments.selected.value"
          title="Selecione um segmento"
          empty
          empty-text="O builder, as versoes e as avaliacoes aparecem apos selecionar um item."
        />
        <template v-else>
          <header class="segment-heading">
            <div>
              <small>{{ segments.selected.value.segment.segmentKey }}</small>
              <h2>{{ segments.selected.value.segment.name }}</h2>
            </div>
            <span>segmentacao {{ segments.selected.value.capabilities.segmentationMode }}</span>
          </header>

          <SegmentBuilder
            v-if="segments.localAst.value && segments.catalog.value"
            :ast="segments.localAst.value"
            :catalog="segments.catalog.value"
            :editable="
              segments.access.canManageSegments.value &&
              segments.selected.value.draft?.status !== 'published'
            "
            :dirty="segments.dirty.value"
            :busy="busy"
            @update="segments.setLocalAst"
            @save="segments.saveDraft"
            @discard="segments.discardDraft"
          />
          <CustomerIntelligenceStatus
            v-else
            title="Sem draft editavel"
            empty
            empty-text="Crie uma nova versao draft pela API autoritativa antes de editar."
          />

          <div class="segments-workspace__split">
            <SegmentVersionsPanel
              :versions="segments.selected.value.versions"
              :draft="segments.selected.value.draft"
              :dirty="segments.dirty.value"
              :busy="busy"
              :can-manage="segments.access.canManageSegments.value"
              :can-publish="segments.access.canPublishSegments.value"
              @validate="segments.versionAction('validate')"
              @publish="segments.versionAction('publish')"
            />
            <SegmentEvaluationPanel
              :run="segments.evaluation.value"
              :can-evaluate="segments.access.canEvaluateSegments.value"
              :dirty="segments.dirty.value"
              :busy="busy"
              :has-draft="Boolean(segments.selected.value.draft)"
              @preview="segments.versionAction('preview')"
            />
          </div>

          <SegmentMaterializationsPanel :materializations="segments.materializations.value" />
        </template>
      </main>
    </template>
  </div>
</template>

<style scoped>
.segments-workspace {
  display: grid;
  grid-template-columns: minmax(15rem, 0.32fr) minmax(0, 1fr);
  gap: 1rem;
}

.segments-workspace__sidebar,
.segments-workspace__main {
  display: grid;
  align-content: start;
  gap: 1rem;
  min-width: 0;
}

.segment-create {
  display: grid;
  gap: 0.55rem;
  padding: 0.8rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.8rem;
}

.segment-create h2 {
  margin: 0;
  font-size: 0.9rem;
}

.segment-create input {
  min-height: 2.3rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.55rem;
  background: rgb(var(--surface));
  color: inherit;
}

.segment-heading,
.segments-workspace__split {
  display: grid;
  gap: 1rem;
}

.segment-heading {
  grid-template-columns: 1fr auto;
  align-items: center;
}

.segment-heading h2,
.segment-heading small {
  margin: 0;
}

.segment-heading small,
.segment-heading span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.segments-workspace__split {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

@media (max-width: 960px) {
  .segments-workspace,
  .segments-workspace__split {
    grid-template-columns: 1fr;
  }
}
</style>
