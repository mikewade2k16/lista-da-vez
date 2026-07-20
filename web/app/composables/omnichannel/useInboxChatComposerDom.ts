import { ref } from "vue";

export function useInboxChatComposerDom() {
  const composerTextareaRef = ref<HTMLTextAreaElement | null>(null);
  const fileInputRef = ref<HTMLInputElement | null>(null);
  const contactPickerRef = ref<HTMLElement | null>(null);
  const emojiPanelRef = ref<HTMLElement | null>(null);
  const emojiSearchInputRef = ref<HTMLInputElement | null>(null);
  const emojiTriggerRef = ref<HTMLElement | null>(null);

  function setFileInputElement(element: Element | null) {
    fileInputRef.value = element instanceof HTMLInputElement ? element : null;
  }

  function setContactPickerElement(element: Element | null) {
    contactPickerRef.value = element instanceof HTMLElement ? element : null;
  }

  function setComposerInputElement(element: Element | null) {
    if (!(element instanceof HTMLElement)) {
      composerTextareaRef.value = null;
      return;
    }

    const textareaElement =
      (element instanceof HTMLTextAreaElement ? element : null) ??
      element.querySelector("textarea");
    composerTextareaRef.value = textareaElement instanceof HTMLTextAreaElement ? textareaElement : null;
  }

  function setEmojiPanelElement(element: Element | null) {
    emojiPanelRef.value = element instanceof HTMLElement ? element : null;
  }

  function setEmojiSearchInputElement(element: Element | null) {
    if (!(element instanceof HTMLElement)) {
      emojiSearchInputRef.value = null;
      return;
    }

    const inputElement = element.querySelector("input");
    emojiSearchInputRef.value = inputElement instanceof HTMLInputElement ? inputElement : null;
  }

  function setEmojiTriggerElement(element: Element | null) {
    emojiTriggerRef.value = element instanceof HTMLElement ? element : null;
  }

  function resolveComposerTextareaElement() {
    const cachedElement = composerTextareaRef.value;
    if (cachedElement instanceof HTMLTextAreaElement && cachedElement.isConnected) {
      return cachedElement;
    }

    if (!import.meta.client) {
      return null;
    }

    const fallbackElement = document.querySelector<HTMLTextAreaElement>(
      "#omni-inbox-center [data-chat-composer-input] textarea, #omni-inbox-center .chat-composer__input-wrap textarea"
    );

    if (fallbackElement instanceof HTMLTextAreaElement) {
      composerTextareaRef.value = fallbackElement;
      return fallbackElement;
    }

    return null;
  }

  function focusComposerTextarea(options?: { cursorAtEnd?: boolean }) {
    const textareaElement = resolveComposerTextareaElement();
    if (!textareaElement || textareaElement.disabled) {
      return false;
    }

    textareaElement.focus({
      preventScroll: true
    });

    if (options?.cursorAtEnd) {
      const cursorPosition = textareaElement.value.length;
      textareaElement.setSelectionRange(cursorPosition, cursorPosition);
    }

    return true;
  }

  function focusEmojiSearchInput() {
    emojiSearchInputRef.value?.focus();
  }

  function isInsideEmojiPanel(target: HTMLElement) {
    return Boolean(emojiPanelRef.value?.contains(target) || emojiTriggerRef.value?.contains(target));
  }

  function isInsideContactPicker(target: HTMLElement) {
    return Boolean(contactPickerRef.value?.contains(target));
  }

  return {
    composerTextareaRef,
    fileInputRef,
    contactPickerRef,
    emojiPanelRef,
    emojiSearchInputRef,
    emojiTriggerRef,
    setFileInputElement,
    setContactPickerElement,
    setComposerInputElement,
    setEmojiPanelElement,
    setEmojiSearchInputElement,
    setEmojiTriggerElement,
    resolveComposerTextareaElement,
    focusComposerTextarea,
    focusEmojiSearchInput,
    isInsideEmojiPanel,
    isInsideContactPicker
  };
}
