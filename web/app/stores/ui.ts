import { ref } from "vue";
import { defineStore } from "pinia";

let toastSequence = 0;
let dialogSequence = 0;
let dialogResolver = null;
const toastTimers = new Map();

function normalizeDialogOptions(options, kind) {
  if (typeof options === "string") {
    return {
      kind,
      title: kind === "error" ? "Erro" : "Aviso",
      message: options,
      confirmLabel: kind === "confirm" ? "Confirmar" : "Fechar",
      cancelLabel: "Cancelar",
      inputLabel: "",
      inputPlaceholder: "",
      initialValue: "",
      required: false
    };
  }

  return {
    kind,
    title: options?.title || (kind === "confirm" ? "Confirmar" : kind === "prompt" ? "Informacao" : "Aviso"),
    message: options?.message || "",
    confirmLabel: options?.confirmLabel || (kind === "confirm" ? "Confirmar" : "Fechar"),
    cancelLabel: options?.cancelLabel || "Cancelar",
    inputLabel: options?.inputLabel || "",
    inputPlaceholder: options?.inputPlaceholder || "",
    initialValue: String(options?.initialValue || ""),
    required: Boolean(options?.required)
  };
}

export const useUiStore = defineStore("ui", () => {
  const toasts = ref([]);
  const dialog = ref(null);

  function clearToastTimer(toastId) {
    const currentTimer = toastTimers.get(toastId);

    if (currentTimer && import.meta.client) {
      window.clearTimeout(currentTimer);
    }

    toastTimers.delete(toastId);
  }

  function dismissToast(toastId) {
    clearToastTimer(toastId);
    toasts.value = toasts.value.filter((toast) => toast.id !== toastId);
  }

  function notify({ type = "info", title = "", message = "", duration = 4000 }) {
    const toastId = `toast-${++toastSequence}`;
    toasts.value = [
      ...toasts.value,
      {
        id: toastId,
        type,
        title,
        message
      }
    ];

    if (import.meta.client && duration > 0) {
      const timerId = window.setTimeout(() => {
        dismissToast(toastId);
      }, duration);
      toastTimers.set(toastId, timerId);
    }

    return toastId;
  }

  function openDialog(options, kind) {
    if (import.meta.server) {
      return Promise.resolve({ confirmed: false, value: "" });
    }

    if (dialog.value && dialogResolver) {
      dialogResolver({ confirmed: false, value: "" });
      dialogResolver = null;
    }

    dialog.value = {
      id: `dialog-${++dialogSequence}`,
      ...normalizeDialogOptions(options, kind)
    };

    return new Promise((resolve) => {
      dialogResolver = resolve;
    });
  }

  function resolveDialog(payload) {
    const resolver = dialogResolver;
    dialogResolver = null;
    dialog.value = null;

    if (resolver) {
      resolver(payload);
    }
  }

  return {
    toasts,
    dialog,
    notify,
    dismissToast,
    success(message, title = "Sucesso") {
      return notify({ type: "success", title, message });
    },
    error(message, title = "Erro") {
      return notify({ type: "error", title, message, duration: 5500 });
    },
    info(message, title = "Informacao") {
      return notify({ type: "info", title, message });
    },
    alert(options) {
      return openDialog(options, "alert");
    },
    confirm(options) {
      return openDialog(options, "confirm");
    },
    prompt(options) {
      return openDialog(options, "prompt");
    },
    submitDialog(value = "") {
      resolveDialog({ confirmed: true, value });
    },
    cancelDialog() {
      resolveDialog({ confirmed: false, value: "" });
    }
  };
});
