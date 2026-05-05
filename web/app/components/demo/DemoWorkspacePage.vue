<script setup>
defineProps({
  page: {
    type: Object,
    required: true
  }
});
</script>

<template>
  <section class="demo-page admin-panel">
    <header class="demo-page__header">
      <div class="demo-page__title-block">
        <span class="demo-page__eyebrow">{{ page.eyebrow }}</span>
        <h1>{{ page.title }}</h1>
        <p>{{ page.description }}</p>
      </div>

      <div class="demo-page__status">
        <span class="material-icons-round" aria-hidden="true">bolt</span>
        <div>
          <strong>{{ page.status }}</strong>
          <span>{{ page.statusMeta }}</span>
        </div>
      </div>
    </header>

    <div class="demo-page__metrics">
      <article
        v-for="metric in page.metrics"
        :key="metric.label"
        class="demo-page__metric"
        :class="`demo-page__metric--${metric.tone}`"
      >
        <span>{{ metric.label }}</span>
        <strong>{{ metric.value }}</strong>
      </article>
    </div>

    <div class="demo-page__body">
      <section class="demo-page__panel">
        <div class="demo-page__panel-head">
          <h2>Movimentacoes recentes</h2>
          <span>{{ page.rows.length }} itens</span>
        </div>

        <div class="demo-page__rows">
          <article
            v-for="row in page.rows"
            :key="row.title"
            class="demo-page__row"
          >
            <div class="demo-page__row-main">
              <strong>{{ row.title }}</strong>
              <span>{{ row.meta }}</span>
            </div>
            <span class="demo-page__row-status" :class="`demo-page__row-status--${row.tone}`">
              {{ row.status }}
            </span>
          </article>
        </div>
      </section>

      <aside class="demo-page__side">
        <div class="demo-page__panel-head">
          <h2>{{ page.asideTitle }}</h2>
        </div>

        <div class="demo-page__tags">
          <span
            v-for="item in page.asideItems"
            :key="item"
            class="demo-page__tag"
          >
            {{ item }}
          </span>
        </div>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.demo-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-height: 0;
  padding: 0.2rem 0.15rem 1rem;
}

.demo-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.25rem 0.1rem 0.1rem;
}

.demo-page__title-block {
  display: grid;
  gap: 0.35rem;
  min-width: 0;
}

.demo-page__eyebrow {
  width: fit-content;
  padding: 0.25rem 0.55rem;
  border-radius: 999px;
  background: rgba(45, 212, 191, 0.12);
  color: #99f6e4;
  font-size: 0.66rem;
  font-weight: 850;
  letter-spacing: 0.08em;
  line-height: 1;
  text-transform: uppercase;
}

.demo-page h1,
.demo-page h2,
.demo-page p {
  margin: 0;
}

.demo-page h1 {
  color: #f8fafc;
  font-size: clamp(1.35rem, 2vw, 1.85rem);
  line-height: 1.1;
}

.demo-page p {
  max-width: 54rem;
  color: rgba(203, 213, 225, 0.76);
  font-size: 0.92rem;
  line-height: 1.55;
}

.demo-page__status {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  flex-shrink: 0;
  min-width: 12.5rem;
  padding: 0.78rem 0.86rem;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.76);
}

.demo-page__status > .material-icons-round {
  display: inline-grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: 10px;
  background: rgba(129, 140, 248, 0.14);
  color: #c7d2fe;
  font-size: 1.05rem;
}

.demo-page__status div {
  min-width: 0;
  display: grid;
  gap: 0.1rem;
}

.demo-page__status strong {
  color: #f8fafc;
  font-size: 0.86rem;
  line-height: 1.2;
}

.demo-page__status span {
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.72rem;
}

.demo-page__metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.7rem;
}

.demo-page__metric {
  display: grid;
  gap: 0.45rem;
  min-height: 6.2rem;
  padding: 0.9rem;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 12px;
  background: rgba(13, 18, 29, 0.86);
}

.demo-page__metric span {
  color: rgba(203, 213, 225, 0.74);
  font-size: 0.76rem;
  font-weight: 700;
}

.demo-page__metric strong {
  align-self: end;
  color: #f8fafc;
  font-size: 1.55rem;
  line-height: 1;
}

.demo-page__metric--info {
  border-color: rgba(56, 189, 248, 0.22);
}

.demo-page__metric--warning {
  border-color: rgba(251, 191, 36, 0.28);
}

.demo-page__metric--success {
  border-color: rgba(34, 197, 94, 0.24);
}

.demo-page__body {
  display: grid;
  grid-template-columns: minmax(0, 1.65fr) minmax(16rem, 0.7fr);
  gap: 0.8rem;
  min-height: 0;
}

.demo-page__panel,
.demo-page__side {
  min-width: 0;
  display: grid;
  align-content: start;
  gap: 0.85rem;
  padding: 0.95rem;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 14px;
  background: rgba(13, 18, 29, 0.82);
}

.demo-page__panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.demo-page__panel-head h2 {
  color: #f8fafc;
  font-size: 0.96rem;
  line-height: 1.2;
}

.demo-page__panel-head span {
  flex-shrink: 0;
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.72rem;
  font-weight: 700;
}

.demo-page__rows {
  display: grid;
  gap: 0.55rem;
}

.demo-page__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.8rem;
  min-height: 4.2rem;
  padding: 0.72rem 0.78rem;
  border: 1px solid rgba(148, 163, 184, 0.1);
  border-radius: 11px;
  background: rgba(8, 13, 24, 0.72);
}

.demo-page__row-main {
  min-width: 0;
  display: grid;
  gap: 0.2rem;
}

.demo-page__row-main strong {
  overflow: hidden;
  color: #e2e8f0;
  font-size: 0.86rem;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.demo-page__row-main span {
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.75rem;
  line-height: 1.3;
}

.demo-page__row-status,
.demo-page__tag {
  display: inline-flex;
  align-items: center;
  min-height: 1.7rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 800;
  line-height: 1;
  white-space: nowrap;
}

.demo-page__row-status {
  padding: 0 0.62rem;
}

.demo-page__row-status--info {
  background: rgba(56, 189, 248, 0.12);
  color: #bae6fd;
}

.demo-page__row-status--warning {
  background: rgba(251, 191, 36, 0.13);
  color: #fde68a;
}

.demo-page__row-status--success {
  background: rgba(34, 197, 94, 0.13);
  color: #bbf7d0;
}

.demo-page__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.demo-page__tag {
  padding: 0 0.68rem;
  border: 1px solid rgba(129, 140, 248, 0.18);
  background: rgba(129, 140, 248, 0.1);
  color: #c7d2fe;
}

@media (max-width: 980px) {
  .demo-page__header,
  .demo-page__body {
    grid-template-columns: minmax(0, 1fr);
  }

  .demo-page__header {
    display: grid;
  }

  .demo-page__status {
    min-width: 0;
  }
}

@media (max-width: 720px) {
  .demo-page__metrics {
    grid-template-columns: minmax(0, 1fr);
  }

  .demo-page__row {
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
  }

  .demo-page__row-main strong {
    white-space: normal;
  }
}
</style>
