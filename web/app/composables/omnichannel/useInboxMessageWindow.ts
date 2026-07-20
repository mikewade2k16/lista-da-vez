/**
 * useInboxMessageWindow
 *
 * Responsabilidade UNICA: garantir que o array de mensagens nunca exceda
 * MESSAGE_WINDOW_SIZE (50) items apos mutacoes automaticas.
 *
 * Regras:
 *   - Apos carga inicial: trim para 50 (ultimas)
 *   - Apos evento realtime (message.created): trim para 50 se usuario esta no fundo do chat
 *   - Apos loadOlderMessages (scroll up): NAO trim — usuario esta lendo historico
 *   - Apos switch de conversa: forceEnforceWindow (sempre 50)
 *   - reloadLatestMessages(): carrega 50 frescas da API (descarta tudo)
 *
 * Este composable e INDEPENDENTE e nao substitui nenhum codigo existente.
 * Ele e chamado como pos-processamento apos as mutacoes.
 */
import type { Ref } from "vue";
import type { Message } from "~/types";
import { isNearBottom } from "~/composables/omnichannel/useOmnichannelInboxShared";

export const MESSAGE_WINDOW_SIZE = 50;

interface MessageWindowOptions {
  messages: Ref<Message[]>;
  hasMoreMessages: Ref<boolean>;
  chatBodyRef: Ref<HTMLElement | null>;
  activeConversationId: Ref<string | null>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  normalizeMessage: (m: Message) => Message;
}

export function useInboxMessageWindow(options: MessageWindowOptions) {
  let loadingLock = false;

  /**
   * Enforce the 50-message window.
   * Keeps latest 50 messages, trims oldest.
   * Only trims when user is at the bottom of chat (or no chat element yet).
   * Skips if a loading operation is in progress.
   */
  function enforceWindow(): void {
    if (loadingLock) return;

    const msgs = options.messages.value;
    if (msgs.length <= MESSAGE_WINDOW_SIZE) return;

    if (!options.chatBodyRef.value || isNearBottom(options.chatBodyRef.value)) {
      options.messages.value = msgs.slice(msgs.length - MESSAGE_WINDOW_SIZE);
      options.hasMoreMessages.value = true;
    }
  }

  /**
   * Force trim to latest 50 regardless of scroll position.
   * Use on conversation switch or explicit reload.
   */
  function forceEnforceWindow(): void {
    const msgs = options.messages.value;
    if (msgs.length <= MESSAGE_WINDOW_SIZE) return;

    options.messages.value = msgs.slice(msgs.length - MESSAGE_WINDOW_SIZE);
    options.hasMoreMessages.value = true;
  }

  /**
   * Prevent enforceWindow from trimming during active load operations.
   */
  function setLoading(value: boolean): void {
    loadingLock = value;
  }

  /**
   * Reload the latest 50 messages from the API for the active conversation.
   * Discards all currently loaded messages and fetches fresh.
   * Used by the sync guard when staleness is detected.
   */
  async function reloadLatestMessages(): Promise<void> {
    const conversationId = options.activeConversationId.value;
    if (!conversationId) return;

    setLoading(true);
    try {
      const response = await options.apiFetch<{
        messages: Message[];
        hasMore: boolean;
      }>(`/conversations/${conversationId}/messages?limit=${MESSAGE_WINDOW_SIZE}`);

      options.messages.value = response.messages.map(options.normalizeMessage);
      options.hasMoreMessages.value = response.hasMore;
    } finally {
      setLoading(false);
    }
  }

  return {
    MESSAGE_WINDOW_SIZE,
    enforceWindow,
    forceEnforceWindow,
    setLoading,
    reloadLatestMessages,
  };
}
