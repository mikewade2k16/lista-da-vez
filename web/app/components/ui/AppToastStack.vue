<script setup>
import { storeToRefs } from "pinia";
import { useUiStore } from "~/stores/ui";

const ui = useUiStore();
const { toasts } = storeToRefs(ui);

function handleToastRemove(toastId) {
  // Aguarda a animação de saída (300ms) + pequeno buffer
  setTimeout(() => {
    ui.dismissToast(toastId);
  }, 320);
}

function renderIcon(type) {
  if (type === "success") {
    return "checkmark";
  } else if (type === "error") {
    return "x-mark";
  } else {
    return "info";
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="app-toast-container" aria-live="polite" aria-atomic="true" data-testid="app-toast-container">
      <TransitionGroup name="toast-animation" tag="div" class="app-toast-stack" move-class="toast-animation-move">
        <article
          v-for="toast in toasts"
          :key="toast.id"
          class="app-toast"
          :class="`app-toast--${toast.type}`"
          :data-testid="`app-toast-${toast.type}`"
        >
          <div class="app-toast__indicator"></div>
          <div class="app-toast__icon">
            <svg v-if="renderIcon(toast.type) === 'checkmark'" class="toast-icon toast-icon--success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
            <svg v-else-if="renderIcon(toast.type) === 'x-mark'" class="toast-icon toast-icon--error" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
            <svg v-else class="toast-icon toast-icon--info" viewBox="0 0 24 24" fill="currentColor">
              <circle cx="12" cy="12" r="1"></circle>
              <path d="M12 7v5"></path>
              <circle cx="12" cy="2" r="10" fill="none" stroke="currentColor" stroke-width="2"></circle>
            </svg>
          </div>
          <div class="app-toast__content">
            <strong v-if="toast.title" class="app-toast__title">{{ toast.title }}</strong>
            <p class="app-toast__message">{{ toast.message }}</p>
          </div>
          <button
            class="app-toast__close"
            type="button"
            aria-label="Fechar notificação"
            :data-testid="`app-toast-close-${toast.id}`"
            @click="handleToastRemove(toast.id)"
          >
            <svg class="toast-close-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
          <div class="app-toast__progress"></div>
        </article>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.app-toast-container {
  position: fixed;
  top: 20px;
  right: 20px;
  pointer-events: none;
  z-index: 9999;
}

.app-toast-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.app-toast {
  pointer-events: auto;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.96);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(226, 232, 240, 0.1);
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3), 0 10px 10px -5px rgba(0, 0, 0, 0.2);
  min-width: 280px;
  max-width: 360px;
  position: relative;
  overflow: hidden;
}

.app-toast__indicator {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 3px;
  border-radius: 8px 0 0 8px;
}

.app-toast--success {
  border-color: rgba(34, 197, 94, 0.2);
}

.app-toast--success .app-toast__indicator {
  background: linear-gradient(180deg, #22c55e 0%, #16a34a 100%);
}

.app-toast--success .app-toast__icon {
  color: #22c55e;
}

.app-toast--success .app-toast__progress {
  background: linear-gradient(90deg, #22c55e 0%, #16a34a 100%);
}

.app-toast--error {
  border-color: rgba(239, 68, 68, 0.2);
}

.app-toast--error .app-toast__indicator {
  background: linear-gradient(180deg, #ef4444 0%, #dc2626 100%);
}

.app-toast--error .app-toast__icon {
  color: #ef4444;
}

.app-toast--error .app-toast__progress {
  background: linear-gradient(90deg, #ef4444 0%, #dc2626 100%);
}

.app-toast--info {
  border-color: rgba(129, 140, 248, 0.2);
}

.app-toast--info .app-toast__indicator {
  background: linear-gradient(180deg, #8194f8 0%, #6366f1 100%);
}

.app-toast--info .app-toast__icon {
  color: #8194f8;
}

.app-toast--info .app-toast__progress {
  background: linear-gradient(90deg, #8194f8 0%, #6366f1 100%);
}

.app-toast__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
}

.toast-icon {
  width: 100%;
  height: 100%;
  animation: scaleIcon 0.4s cubic-bezier(0.68, -0.55, 0.265, 1.55) forwards 0.15s;
}

.toast-icon--success {
  stroke-dasharray: 52;
  stroke-dashoffset: 52;
  animation: scaleIcon 0.4s cubic-bezier(0.68, -0.55, 0.265, 1.55) forwards 0.15s, drawCheckmark 0.5s ease-out forwards 0.35s;
}

.toast-icon--error {
  stroke-dasharray: 60;
  stroke-dashoffset: 60;
  animation: scaleIcon 0.4s cubic-bezier(0.68, -0.55, 0.265, 1.55) forwards 0.15s, drawXMark 0.4s ease-out forwards 0.35s;
}

.toast-icon--info {
  opacity: 0;
  animation: scaleIcon 0.4s cubic-bezier(0.68, -0.55, 0.265, 1.55) forwards 0.15s, fadeIn 0.3s ease-out forwards 0.35s;
}

.toast-close-icon {
  width: 100%;
  height: 100%;
  stroke-dasharray: 60;
  stroke-dashoffset: 60;
  animation: drawXMark 0.5s cubic-bezier(0.43, 1.26, 0.84, 1) forwards 0.2s;
}

.app-toast__content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.app-toast__title {
  display: block;
  margin: 0;
  font-size: 0.8rem;
  font-weight: 600;
  color: #e2e8f0;
  word-break: break-word;
}

.app-toast__message {
  display: block;
  margin: 0;
  font-size: 0.75rem;
  color: #cbd5e1;
  line-height: 1.3;
  word-break: break-word;
  overflow-wrap: break-word;
}

.app-toast__close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.app-toast__close:hover {
  background: rgba(226, 232, 240, 0.1);
  color: #e2e8f0;
}

.app-toast__close:active {
  transform: scale(0.95);
}

.app-toast__progress {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 2px;
  animation: progressBar 4s linear forwards;
  opacity: 0.8;
}

.app-toast--error .app-toast__progress {
  animation: progressBar 5.5s linear forwards;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateX(100%) translateY(-20px);
  }
  to {
    opacity: 1;
    transform: translateX(0) translateY(0);
  }
}

@keyframes slideOut {
  from {
    opacity: 1;
    transform: translateX(0) translateY(0);
  }
  to {
    opacity: 0;
    transform: translateX(100%) translateY(-20px);
  }
}

@keyframes scaleIcon {
  from {
    transform: scale(0) rotate(-45deg);
    opacity: 0;
  }
  to {
    transform: scale(1) rotate(0deg);
    opacity: 1;
  }
}

@keyframes drawCheckmark {
  from {
    stroke-dashoffset: 52;
    stroke-linecap: round;
  }
  to {
    stroke-dashoffset: 0;
  }
}

@keyframes drawXMark {
  from {
    stroke-dashoffset: 60;
  }
  to {
    stroke-dashoffset: 0;
  }
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes progressBar {
  from {
    width: 100%;
    opacity: 0.8;
  }
  to {
    width: 0%;
    opacity: 0;
  }
}

.toast-animation-enter-active {
  animation: slideIn 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.toast-animation-leave-active {
  animation: slideOut 0.3s cubic-bezier(0.4, 0, 0.2, 1) forwards;
}

.toast-animation-move {
  transition: transform 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

@media (max-width: 768px) {
  .app-toast-container {
    left: 10px;
    right: 10px;
    top: 10px;
  }

  .app-toast {
    min-width: unset;
    max-width: unset;
    padding: 10px 12px;
    gap: 10px;
  }

  .app-toast__title {
    font-size: 0.7rem;
  }

  .app-toast__message {
    font-size: 0.65rem;
    line-height: 1.2;
  }

  .app-toast__icon {
    width: 20px;
    height: 20px;
  }

  .app-toast__close {
    width: 24px;
    height: 24px;
  }
}
</style>
