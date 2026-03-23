# Backlog do Produto Omni

Atualizado em: 2026-03-23

## Entregas concluidas (MVP atual)

- Lista da vez com entrada na fila pela barra de Co.
- Atender primeiro da fila e atender fora da vez.
- Encerrar atendimento com retorno automatico ao fim da fila.
- Controle de limite simultaneo de atendimento (default 10, configuravel).
- Pausa e retomada de consultor.
- Modal de encerramento com desfecho (`reserva`, `compra`, `nao-compra`).
- Modal com produto visto e produto fechado.
- Modal com nome e telefone obrigatorios (configuravel).
- Modal com email opcional.
- Modal com origem do cliente em multipla escolha.
- Modal com motivo da visita em multipla escolha + detalhe opcional.
- Modal com motivo de atendimento fora da vez (apenas quando aplicavel).
- Registro de id de atendimento para log (`serviceId`).
- Coleta e persistencia de tempo de atendimento (`durationMs`).
- Coleta e persistencia de espera de fila (`queueWaitMs`).
- Coleta e persistencia de tempo por status (`available`, `queue`, `service`, `paused`).
- Painel `Consultor` com meta, progresso e simulador.
- Painel `Ranking` com visao mensal e diaria.
- Painel `Dados` com leituras operacionais brutas.
- Painel `Inteligencia` com diagnostico automatico e acoes recomendadas.
- Painel `Configuracoes` para campos do modal, textos, regras e opcoes.
- Modo teste com preenchimento automatico e catalogo mock de produtos.

## Backlog oficial (aberto)

### P1 (parcial em 2026-03-13)

- [x] Templates de operacao (presets por tipo de loja/estrategia).
- [x] Gestao administrativa de consultores (CRUD de meta/comissao/perfil via UI).
- [ ] Persistencia backend (substituir `localStorage` como origem principal).
- [x] Permissoes por perfil (admin/gerente/consultor).
- [ ] Autenticacao por login (adiada; usando dropdown de perfil no header para testes).

### P2

- [ ] Integracao real de produtos via API (substituir catalogo mock).
- [x] Relatorios com filtros avancados e exportacao CSV/PDF.
- [x] Campanhas e regras comerciais (modulo dedicado).

### P3

- [x] Multi-loja com visao consolidada.

## Observacoes de implementacao do P1

- Templates aplicaveis via menu de configuracoes.
- CRUD de consultores disponivel no painel administrativo.
- Troca de perfil por dropdown no header para teste rapido.
- Persistencia permanece local (`localStorage`) nesta fase.
- P2 em andamento sem API de produtos (relatorios e campanhas entregues em frontend/local).
- Multi-loja ativo com operacao independente por loja e painel consolidado.

## Novas regras de negocio e arquitetura alvo

### Operacao em tempo real

- A plataforma deve operar com sincronizacao em tempo real entre dispositivos conectados na mesma loja.
- O app pode permanecer logado nos computadores das lojas e tambem nos celulares das atendentes.
- Mudancas de fila, pausa, retomada, inicio e encerramento de atendimento devem propagar imediatamente para os clientes conectados.
- A mesma operacao nao pode depender de refresh manual ou polling como mecanismo principal.
- O backend deve tratar concorrencia para evitar estado divergente entre dispositivos.

### Escopo inicial de rollout

- O primeiro rollout sera um piloto em 1 loja.
- A arquitetura deve nascer preparada para 4 lojas no curto prazo.
- O alvo inicial de capacidade e cerca de 30 acessos simultaneos.
- O sistema deve priorizar baixo consumo de recursos e tempo de resposta consistente no piloto.

### Diretriz de stack

- Migrar o frontend para Nuxt 3 como base oficial da aplicacao.
- Implementar persistencia backend real para substituir `localStorage` como origem principal.
- Adotar PostgreSQL como banco principal para estado operacional, historico e configuracoes.
- Implementar backend em Go para API, regras transacionais e camada de tempo real.
- WebSocket passa a ser requisito de arquitetura para sincronizacao de fila, atendimento e dashboards.

### Modularizacao

- A funcionalidade de `lista da vez` deve ser isolada como modulo de dominio reutilizavel.
- O modulo deve poder ser plugado futuramente em outro projeto que tambem use Nuxt no frontend e Go no backend.
- Regras centrais da fila nao devem ficar acopladas aos paineis administrativos ou a detalhes visuais da aplicacao atual.

### Itens a entrar no backlog tecnico

- [ ] Definir arquitetura realtime com WebSocket e reconciliacao de estado por loja.
- [ ] Definir modelo de autenticacao persistente por usuario/dispositivo.
- [ ] Projetar schema PostgreSQL para lojas, usuarios, consultores, fila, atendimentos, eventos e historico.
- [ ] Separar regras da `lista da vez` em modulo independente de dominio.
- [ ] Iniciar migracao oficial para Nuxt 3 mantendo paridade funcional do MVP.
- [ ] Implementar backend Go com API e eventos em tempo real.
- [ ] Instrumentar logs, metricas e observabilidade para o piloto da primeira loja.
- [ ] Executar teste controlado de carga para pelo menos 30 conexoes simultaneas.
