<template>
  <div
    class="omni-loading"
    role="status"
    aria-live="polite"
    aria-label="Carregando inbox de atendimento"
  >
    <div class="omni-loading__toolbar">
      <USkeleton class="h-4 w-28" />
      <USkeleton class="h-9 w-52" />
    </div>

    <div class="omni-loading__alert">
      <USkeleton class="h-5 w-72" />
      <USkeleton class="h-4 w-full max-w-[36rem]" />
    </div>

    <div class="omni-loading__grid">
      <section class="omni-loading__panel omni-loading__panel--sidebar">
        <div class="omni-loading__stack">
          <USkeleton class="h-10 w-full" />
          <USkeleton class="h-10 w-full" />
          <USkeleton class="h-24 w-full" />
        </div>

        <div class="omni-loading__cards">
          <div v-for="index in 4" :key="`sidebar-card-${index}`" class="omni-loading__card">
            <USkeleton class="h-10 w-10 rounded-full" />
            <div class="omni-loading__card-copy">
              <USkeleton class="h-4 w-32" />
              <USkeleton class="h-3 w-full" />
              <USkeleton class="h-3 w-24" />
            </div>
          </div>
        </div>
      </section>

      <section class="omni-loading__panel omni-loading__panel--chat">
        <div class="omni-loading__chat-header">
          <USkeleton class="h-11 w-56" />
          <USkeleton class="h-9 w-28" />
        </div>

        <div class="omni-loading__messages">
          <div
            v-for="index in 6"
            :key="`message-${index}`"
            class="omni-loading__message"
            :class="{ 'omni-loading__message--outbound': index % 2 === 0 }"
          >
            <USkeleton class="h-20 w-full max-w-[22rem]" />
          </div>
        </div>

        <div class="omni-loading__composer">
          <USkeleton class="h-12 flex-1" />
          <USkeleton class="h-12 w-12 rounded-full" />
          <USkeleton class="h-12 w-28" />
        </div>
      </section>

      <section class="omni-loading__panel omni-loading__panel--details">
        <div class="omni-loading__stack">
          <USkeleton class="h-10 w-full" />
          <USkeleton class="h-20 w-full" />
          <USkeleton class="h-28 w-full" />
          <USkeleton class="h-12 w-full" />
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.omni-loading {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  flex-direction: column;
  gap: 0.75rem;
  padding: 0.5rem;
}

.omni-loading__toolbar,
.omni-loading__alert,
.omni-loading__panel {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-lg);
  background: rgb(var(--surface));
}

.omni-loading__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
}

.omni-loading__alert {
  display: grid;
  gap: 0.5rem;
  padding: 1rem;
}

.omni-loading__grid {
  display: grid;
  flex: 1 1 auto;
  min-height: 0;
  gap: 0.75rem;
  grid-template-columns: minmax(18rem, 24rem) minmax(0, 1fr) minmax(18rem, 22rem);
}

.omni-loading__panel {
  min-height: 0;
  padding: 1rem;
}

.omni-loading__panel--sidebar,
.omni-loading__panel--details {
  display: grid;
  align-content: start;
  gap: 1rem;
}

.omni-loading__stack {
  display: grid;
  gap: 0.75rem;
}

.omni-loading__cards {
  display: grid;
  gap: 0.75rem;
}

.omni-loading__card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 0.75rem;
  align-items: center;
}

.omni-loading__card-copy {
  display: grid;
  gap: 0.45rem;
}

.omni-loading__panel--chat {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  gap: 1rem;
}

.omni-loading__chat-header,
.omni-loading__composer {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.omni-loading__messages {
  display: grid;
  align-content: start;
  gap: 0.75rem;
}

.omni-loading__message {
  display: flex;
  justify-content: flex-start;
}

.omni-loading__message--outbound {
  justify-content: flex-end;
}

@media (max-width: 1200px) {
  .omni-loading__grid {
    grid-template-columns: minmax(18rem, 22rem) minmax(0, 1fr);
  }

  .omni-loading__panel--details {
    display: none;
  }
}

@media (max-width: 900px) {
  .omni-loading__toolbar,
  .omni-loading__composer {
    flex-wrap: wrap;
  }

  .omni-loading__grid {
    grid-template-columns: 1fr;
  }

  .omni-loading__panel--sidebar {
    order: 2;
  }
}
</style>
