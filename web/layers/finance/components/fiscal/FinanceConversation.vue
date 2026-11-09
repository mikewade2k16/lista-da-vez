<script setup lang="ts">
import { nextTick, ref, watch, onActivated } from 'vue'
import AccountingEvidence from '../accounting/AccountingEvidence.vue'
import CalendarChatMessage from '~/components/calendar/CalendarChatMessage.vue'
import FiscalAccessPanel from './FiscalAccessPanel.vue'
import FiscalProfileCard from './FiscalProfileCard.vue'
import FinanceDocumentIntake from './FinanceDocumentIntake.vue'
import FinanceConversationVoice from './FinanceConversationVoice.vue'
import { useFiscalProfile } from '../../composables/useFiscalProfile'
import { useFinanceConversation } from '../../composables/useFinanceConversation'
defineEmits<{ discuss: [text: string] }>()
const chat = useFinanceConversation(),
  fiscal = useFiscalProfile()
const { messages, draft, error, busy, action, scope, selectedDraftId, activeDraftId } = chat
const { error: fiscalError, message: fiscalMessage } = fiscal
const stream = ref<HTMLElement | null>(null),
  voiceBusy = ref(false),
  credentialsDirty = ref(false),
  profileEdits = ref(false),
  documentEdits = ref(false)
async function send(text = draft.value) {
  if (
    voiceBusy.value ||
    credentialsDirty.value ||
    fiscal.profileDirty.value ||
    profileEdits.value ||
    documentEdits.value
  )
    return
  await chat.send(text)
}
watch([() => messages.value.length, action], async () => {
  await nextTick()
  if (stream.value) {
    const last = stream.value.querySelector<HTMLElement>('.calendar-chat__message:last-of-type')
    stream.value.scrollTop =
      action.value && last ? last.offsetTop - stream.value.offsetTop : stream.value.scrollHeight
  }
})
watch(scope, () => {
  credentialsDirty.value = false
  profileEdits.value = false
  documentEdits.value = false
})
onActivated(() => {
  if (
    !chat.busy.value &&
    !chat.draft.value.trim() &&
    !credentialsDirty.value &&
    !fiscal.profileDirty.value &&
    !profileEdits.value &&
    !documentEdits.value
  )
    void chat.load()
})
</script>
<template>
  <div class="finance-conversation">
    <div ref="stream" class="calendar-chat__stream" aria-live="polite">
      <p v-if="!messages.length" class="calendar-chat__empty">Como posso ajudar?</p>
      <template v-for="m in messages" :key="m.id">
        <CalendarChatMessage
          :message="{
            id: m.id,
            role: m.role,
            text: m.text,
            proposals: [],
            calendarItems: [],
            resources: [],
          }"
          :busy="busy"
          :clients="[]"
          scope-mode="all"
          scope-client-id=""
        />
        <AccountingEvidence
          v-if="m.evidence?.length"
          :evidence="m.evidence"
          :data="{ coverage: 'informational' }"
        />
      </template>
      <p v-if="error" role="alert">{{ error }}</p>
      <p v-if="fiscalError" role="alert">{{ fiscalError }}</p>
      <p v-if="fiscalMessage" role="status">{{ fiscalMessage }}</p>
      <fieldset v-if="action" class="finance-conversation__editor" :disabled="busy">
        <FiscalAccessPanel
          v-if="action === 'access'"
          :key="scope"
          :editor="fiscal"
          expanded
          @dirty="credentialsDirty = $event"
        />
        <FiscalProfileCard
          v-if="action === 'profile'"
          :key="`profile:${scope}`"
          @dirty="profileEdits = $event"
        />
        <FinanceDocumentIntake
          v-if="action === 'document' || action === 'prepare'"
          :key="`documents:${scope}:${messages[messages.length - 1]?.id || ''}`"
          :document-id="activeDraftId"
          :review-expanded="action === 'prepare'"
          expanded
          @selected="selectedDraftId = $event"
          @dirty="documentEdits = $event"
          @discuss="draft = $event"
        />
      </fieldset>
      <p v-if="busy" role="status">Aguarde...</p>
    </div>
    <FinanceConversationVoice
      :key="scope"
      :value="draft"
      :disabled="
        busy || credentialsDirty || fiscal.profileDirty.value || profileEdits || documentEdits
      "
      :sending="busy"
      :send-disabled="!draft.trim()"
      @text="draft = $event"
      @busy="voiceBusy = $event"
      @send="send()"
      @attach="send('Quero anexar um comprovante')"
    />
  </div>
</template>
<style scoped>
.finance-conversation {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.finance-conversation__editor {
  flex-shrink: 0;
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
</style>
