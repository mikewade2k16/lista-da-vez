import { computed, ref, type Ref } from "vue";
import type { Contact } from "~/types";

type AttachmentPickerMode = "document" | "media" | "camera" | "audio" | "sticker" | "gif";

type SendContactPayload = {
  name: string;
  phone: string;
  contactId?: string | null;
  avatarUrl?: string | null;
};

export function useInboxChatComposerControls(options: {
  getSavedContacts: () => Contact[];
  getDraft: () => string;
  getHasAttachment: () => boolean;
  getCanManageConversation: () => boolean;
  getActiveConversation: () => unknown | null;
  getIsRecording: () => boolean;
  fileInputRef: Ref<HTMLInputElement | null>;
  extractFirstLinkFromText: (value: string) => string | null;
  stopRecordingForPicker: () => void;
  closeEmojiPanel: (params?: { resetSearch?: boolean }) => void;
  onSendContact: (payload: SendContactPayload) => void;
}) {
  const pickerMode = ref<AttachmentPickerMode>("document");
  const linkPreviewEnabled = ref(true);
  const contactPickerOpen = ref(false);
  const contactPickerSearch = ref("");

  function updateContactPickerSearch(value: string) {
    contactPickerSearch.value = value;
  }

  const filteredSavedContacts = computed(() => {
    const list = Array.isArray(options.getSavedContacts()) ? options.getSavedContacts() : [];
    const term = contactPickerSearch.value.trim().toLowerCase();

    if (!term) {
      return list.slice(0, 40);
    }

    const normalizedDigits = term.replace(/\D/g, "");

    return list
      .filter((contactEntry) => {
        const name = contactEntry.name?.toLowerCase() ?? "";
        const phone = contactEntry.phone ?? "";

        if (name.includes(term)) {
          return true;
        }

        if (normalizedDigits && phone.includes(normalizedDigits)) {
          return true;
        }

        return phone.toLowerCase().includes(term);
      })
      .slice(0, 40);
  });

  const composerHasContent = computed(() => {
    return options.getDraft().trim().length > 0 || options.getHasAttachment();
  });

  const isComposerReadOnly = computed(() => !options.getCanManageConversation());

  const draftLinkUrl = computed(() => options.extractFirstLinkFromText(options.getDraft()));
  const showLinkPreviewToggle = computed(() => {
    return Boolean(draftLinkUrl.value && !options.getHasAttachment());
  });

  const pickerAccept = computed(() => {
    if (pickerMode.value === "sticker") {
      return ".webp,image/webp,image/png,image/jpeg";
    }

    if (pickerMode.value === "gif") {
      return "video/mp4,image/gif";
    }

    if (pickerMode.value === "media") {
      return "image/*,video/*";
    }

    if (pickerMode.value === "camera") {
      return "image/*";
    }

    if (pickerMode.value === "audio") {
      return "audio/*";
    }

    return ".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.csv,.zip,.rar,.7z,image/*,video/*,audio/*";
  });

  const pickerCapture = computed(() => {
    if (pickerMode.value === "camera") {
      return "environment";
    }

    return undefined;
  });

  function closeContactPicker() {
    contactPickerOpen.value = false;
    contactPickerSearch.value = "";
  }

  function openAttachmentPicker(mode: AttachmentPickerMode) {
    if (options.getIsRecording()) {
      options.stopRecordingForPicker();
    }

    pickerMode.value = mode;
    options.fileInputRef.value?.click();
  }

  function openContactPrompt() {
    if (!options.getActiveConversation() || !options.getCanManageConversation()) {
      return;
    }

    options.closeEmojiPanel();
    contactPickerOpen.value = !contactPickerOpen.value;
    contactPickerSearch.value = "";
  }

  function selectSavedContactForSend(contactEntry: Contact) {
    const phone = contactEntry.phone.trim();
    if (!phone) {
      return;
    }

    options.onSendContact({
      name: contactEntry.name?.trim() || phone,
      phone,
      contactId: contactEntry.id,
      avatarUrl: contactEntry.avatarUrl
    });
    closeContactPicker();
  }

  function toggleLinkPreview() {
    linkPreviewEnabled.value = !linkPreviewEnabled.value;
  }

  const attachmentMenuItems = computed(() => {
    return [
      [
        {
          label: "Documento (sem compressao)",
          icon: "i-lucide-file-text",
          onSelect: () => openAttachmentPicker("document")
        },
        {
          label: "Fotos e videos",
          icon: "i-lucide-image",
          onSelect: () => openAttachmentPicker("media")
        },
        {
          label: "Camera",
          icon: "i-lucide-camera",
          onSelect: () => openAttachmentPicker("camera")
        },
        {
          label: "Audio",
          icon: "i-lucide-headphones",
          onSelect: () => openAttachmentPicker("audio")
        },
        {
          label: "Figurinha",
          icon: "i-lucide-sticky-note",
          onSelect: () => openAttachmentPicker("sticker")
        }
      ],
      [
        {
          label: "Contato",
          icon: "i-lucide-user-round",
          onSelect: () => openContactPrompt()
        },
        {
          label: "Enquete",
          icon: "i-lucide-list-filter",
          disabled: true
        },
        {
          label: "Evento",
          icon: "i-lucide-calendar-days",
          disabled: true
        }
      ]
    ];
  });

  return {
    pickerMode,
    linkPreviewEnabled,
    contactPickerOpen,
    contactPickerSearch,
    updateContactPickerSearch,
    filteredSavedContacts,
    composerHasContent,
    isComposerReadOnly,
    showLinkPreviewToggle,
    pickerAccept,
    pickerCapture,
    closeContactPicker,
    openAttachmentPicker,
    openContactPrompt,
    selectSavedContactForSend,
    toggleLinkPreview,
    attachmentMenuItems
  };
}
