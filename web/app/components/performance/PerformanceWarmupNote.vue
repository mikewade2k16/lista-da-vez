<script setup lang="ts">
// Bloco explicativo do warm-up de dev. Texto fixo pt-BR, sem dados dinamicos.
</script>

<template>
  <section class="performance-warmup">
    <header class="performance-warmup__head">
      <h3 class="performance-warmup__title">Por que medimos com warm-up</h3>
      <p class="performance-warmup__subtitle">
        Em desenvolvimento o tempo de compilacao do Vite contamina a medicao.
      </p>
    </header>

    <div class="performance-warmup__body">
      <p class="performance-warmup__text">
        No ambiente de dev, a primeira vez que uma rota e aberta o Vite precisa compilar o codigo
        daquela tela sob demanda. Esse custo de compilacao e unico (some no segundo acesso) e nao
        existe no build de producao, mas aparece como um pico enorme se for medido junto com a
        navegacao.
      </p>
      <p class="performance-warmup__text">
        Por isso a auditoria roda um
        <strong>warm-up</strong>
        : cada rota e visitada uma vez antes de medir, para pre-compilar/aquecer o cache. Assim os
        numeros refletem o custo real de navegacao do app (fetch de dados, montagem dos componentes,
        realtime), e nao o custo de compilar pela primeira vez.
      </p>

      <ul class="performance-warmup__list">
        <li class="performance-warmup__item">
          <span class="performance-warmup__marco">T1</span>
          <span>clique ate a troca de rota (fetch bloqueante de setup/middleware).</span>
        </li>
        <li class="performance-warmup__item">
          <span class="performance-warmup__marco">T2</span>
          <span>clique ate a primeira pintura (conteudo aparece na tela).</span>
        </li>
        <li class="performance-warmup__item">
          <span class="performance-warmup__marco">T3</span>
          <span>clique ate o carregamento final (network idle e sem skeleton).</span>
        </li>
      </ul>

      <p class="performance-warmup__note">
        Rotas marcadas como
        <strong>realtime (cap 15s)</strong>
        nunca ficam quietas porque seguem recebendo atualizacoes ao vivo (ex.: a operacao). Nesses
        casos o T3 e travado no teto de seguranca de 15 segundos: nao e que a tela demore 15s para
        abrir, e que ela nunca para de atualizar.
      </p>

      <p class="performance-warmup__note">
        Os modos comparam dois cenarios:
        <strong>in-app</strong>
        (navegacao SPA, ja com o app carregado) e
        <strong>cold</strong>
        (primeira visita ou F5, com o documento recarregando do zero). O cold e sempre mais lento
        porque inclui o boot do app.
      </p>

      <div class="performance-warmup__reaudit">
        <h4 class="performance-warmup__reaudit-title">Re-auditoria 26/06/2026 — action-first</h4>
        <p class="performance-warmup__text">
          Varredura nas 41 paginas do painel: 40 ja seguem o padrao action-first (a rota troca na
          hora e o conteudo carrega depois, com skeleton). A unica excecao era
          <strong>/usuarios</strong>
          , que segurava a troca de rota com um await de topo no componente (UsersAccessManager) ate
          /v1/users e /v1/auth/roles responderem — em correcao.
        </p>
        <p class="performance-warmup__note performance-warmup__note--alert">
          O gargalo de app que ainda sobra nao e a navegacao, e o
          <strong>login</strong>
          . Hoje o login encadeia 4+ chamadas em sequencia (login, contexto, contas,
          settings/operacao) antes de sair da tela, e o botao "Entrando..." volta ao normal antes de
          a navegacao terminar — por isso parece travado. A correcao (navegar assim que o contexto
          chega e adiar o resto para a pagina destino) esta planejada. Obs.: o login ainda nao e
          medido por esta auditoria (so rotas pos-login), por isso nao aparece na tabela acima.
        </p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.performance-warmup {
  display: grid;
  gap: 0.85rem;
  padding: 1.1rem 1.2rem;
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-card);
}

.performance-warmup__head {
  display: grid;
  gap: 0.2rem;
}

.performance-warmup__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-main);
}

.performance-warmup__subtitle {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.performance-warmup__body {
  display: grid;
  gap: 0.75rem;
}

.performance-warmup__text {
  margin: 0;
  font-size: 0.85rem;
  line-height: 1.55;
  color: var(--text-main);
}

.performance-warmup__list {
  display: grid;
  gap: 0.4rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.performance-warmup__item {
  display: grid;
  grid-template-columns: 2rem minmax(0, 1fr);
  align-items: baseline;
  gap: 0.6rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.performance-warmup__marco {
  display: grid;
  place-items: center;
  padding: 0.05rem 0.3rem;
  border-radius: var(--radius-soft);
  background: color-mix(in srgb, var(--accent-info) 16%, transparent);
  color: var(--accent-info);
  font-size: 0.72rem;
  font-weight: 700;
}

.performance-warmup__note {
  margin: 0;
  padding: 0.7rem 0.85rem;
  border-radius: var(--radius-soft);
  background: var(--bg-muted);
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-muted);
}

.performance-warmup__reaudit {
  display: grid;
  gap: 0.55rem;
  margin-top: 0.35rem;
  padding-top: 0.85rem;
  border-top: 1px solid var(--line-soft);
}

.performance-warmup__reaudit-title {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text-main);
}

.performance-warmup__note--alert {
  background: color-mix(in srgb, var(--accent-warning) 14%, transparent);
  color: var(--text-main);
}
</style>
