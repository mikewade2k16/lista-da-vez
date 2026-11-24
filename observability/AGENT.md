# Observabilidade — backlog e estado (fonte única)

Leia este arquivo antes de mexer em qualquer parte da observabilidade. Ao terminar uma
task: marque `[x]`, escreva a data, o commit e uma linha do que foi feito. Ao começar:
marque `[~]` com a data, para outro agente não pegar a mesma task.

Mapa do módulo:

| parte                                                    | onde                                                                                  | doc        |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------- | ---------- |
| contratos (endpoints, tabelas, regras)                   | `observability/SPEC.md`                                                               | —          |
| como rodar, flags, segurança do runner                   | `observability/README.md`                                                             | —          |
| instrumentação da API (coletor, tracer, presença)        | `back/internal/platform/observability/`                                               | `AGENT.md` |
| painel e alertas (endpoints `/v1/admin/observability/*`) | `back/internal/modules/observability/`                                                | `AGENT.md` |
| agente de containers e disco                             | `back/internal/obsagent/`, `back/cmd/obs-agent/`                                      | `AGENT.md` |
| runner, inventário, catálogo de segurança                | `observability/runner/`, `observability/scripts/`, `observability/config/`            | README     |
| página                                                   | `web/app/pages/manage/observabilidade.vue`, `web/app/components/admin/observability/` | —          |
| auditoria e Top 10                                       | `observability/reports/AUDITORIA.md`                                                  | —          |

Regras de trabalho (aprendidas em 2026-11-23):

- **Builds Go descartáveis:** `--cpus=2 --cpu-shares=256 -v omni_gocache:/root/.cache/go-build`,
  imagem `golang:1.26.11-bookworm`, com no máximo 2 agentes compilando ao mesmo tempo.
  Com 8 em paralelo, o Mac chegou a carga 350 e travou.
- **Banco local:** é cópia de produção. O runner não faz escrita nele.
- **Commit, push e deploy:** só com pedido explícito do usuário.
- **Credenciais de teste:** ficam em `observability/.env.local`, fora do Git
  (ADMIN, CONSULTANT, TENANT_A, TENANT_B; contas locais "OBS Teste A/B").
- **E-mail local:** vai para o Mailpit (http://localhost:8025).
- **E-mail em produção:** sai pelo Gmail, com SMTP no `.env.production` da VPS.

## Feito

- [x] **F0–F5 (2026-11-23):** inventário, auditoria, instrumentação, runner, painel e
      primeira execução. Commits 9912e3d..84928d9.
- [x] **F6 saúde do sistema (2026-11-23, 6d2619a):**
  - coletor de runtime, pool, Postgres e filas;
  - `obs-agent` (containers e disco);
  - alertas por e-mail com 17 regras;
  - presença de usuários;
  - abas Saúde, Sistema, Segurança, Alertas e Deploys;
  - catálogo de 81 ataques, com 22 automatizados;
  - histórico de deploys.
- [x] **F7 correções críticas e altas (2026-11-24, 3da9607):**
  - XSS dos comentários;
  - limite de login e rate limit por usuário;
  - `/uploads` sem listagem;
  - 404 para conta alheia;
  - desligamento gracioso;
  - 500→400 em 5 endpoints;
  - snapshot de 2,5 s para 0,3 s;
  - Go e npm sem vulnerabilidades altas ou críticas.
- [x] _*Variáveis OBS_* na API e Mailpit local (2026-11-24, 626efd9)._*
- [x] **Produção (2026-11-24):**
  - deploy feito;
  - SMTP do Gmail e `OBS_ALERT_EMAILS` configurados na VPS;
  - e-mail de teste entregue.
- [x] **Contas de teste locais e isolamento entre contas (2026-11-24):** 6 trocas de
      conta, nenhum vazamento.

## Backlog — do mais rápido ao mais complexo

Estimativa entre colchetes: P = até 1 h, M = meio dia, G = 1 a 2 dias, GG = vários dias.

### Etapa 1 — rápidas (P)

- [x] **1.1 Regra `container_down` sem falso alarme** (2026-11-24, ddd0c58; falta só deploy) (código pronto e testado, sem
  commit em 2026-11-24). Parada manual de container `always`/`unless-stopped` não
  alerta. Falta commit e deploy; em produção ainda há 6 alertas falsos abertos.
- [x] **1.2 Sincronizar o catálogo de segurança em produção.** (2026-11-24) 81 checagens e 32 achados gravados por túnel SSH; o sync lê o status da AUDITORIA (corrigido → fixed, sem sobrescrever edição manual). Hoje a área aparece
      como "desconhecida". Rodar `obs:security --sync` com `OBS_DATABASE_URL` da produção
      por túnel SSH, só upsert de catálogo e achados.
- [x] **1.3 Reconhecer e silenciar alertas.** (2026-11-24, sem commit) Migration 0332 (4 colunas em `obs.alerts`), endpoints `ack`/`silence`/`unsilence`, motor respeita ack e silêncio, botões na aba Alertas e selo na Saúde; falta aplicar a 0332 e deploy. Original: Botões na aba Alertas: "reconhecer"
      (para de reavisar) e "silenciar por X h", com quem fez e quando. Precisa de uma
      coluna nova em `obs.alerts`.
- [x] **1.4 Monitor externo de uptime.** (2026-11-24) `.github/workflows/uptime.yml`: cron de 5 min, `/healthz` e home com retry; falha abre/comenta issue `uptime`, volta fecha. Original: Workflow agendado no GitHub Actions (a cada
      5 min) que chama `/healthz` e a home, e abre uma issue ou manda e-mail se falhar.
      Cobre o caso de a VPS inteira cair junto com o próprio monitor.
- [x] **1.5 Vencimento de TLS e domínios.** (2026-11-24) Coleta + regra: obs-agent lê o certificado de `OBS_AGENT_TLS_HOSTS` a cada 6 h (`tls` na amostra agent); regra `tls_expiring` no módulo observability. Original: O agente lê a validade dos certificados
      dos domínios do Caddy; alerta com 14 dias de antecedência.
- [x] **1.6 Aba Integrações.** (2026-11-24, sem commit) `GET /integrations` (p95 por `percentile_cont`, por integração de `obs.external_calls`) e aba "Integrações" no painel. Original: Os dados já estão em `obs.external_calls`: mostrar
      tempo, erro e volume por host externo (Meta, Google, IA, WhatsApp, R2).
- [x] **1.7 Métrica de panic.** (2026-11-24) Coleta + regra: `platform/panics` conta panics recuperados (HTTP, background, jobs); amostra `api` ganha `panics {total, delta}`; regra `panic_detected` no módulo observability. Original: Contar os panics (API e workers) em
      `obs.system_samples` e criar a regra de alerta `panic_detected`.

### Etapa 2 — governança pelo painel (M)

- [x] **2.1 Configuração dos alertas no banco** (2026-11-24, sem commit): `obs.settings` +
      `GET /settings`, `PUT /settings/{key}`; o motor relê a cada 60 s (destinatários, reaviso,
      horário de silêncio, liga/desliga, severidade e limite de 19 regras); aba Alertas > Configurações. Original: tabela `obs.settings`. Editável na
      aba Alertas:
  - destinatários;
  - liga e desliga e limite de cada regra;
  - frequência de reaviso;
  - horário de silêncio.

  As variáveis de ambiente passam a ser só o valor inicial.

- [x] **2.2 Layout e conteúdo do e-mail** (2026-11-24, sem commit): chave `email` (prefixo, agrupar, seções, resolvidos, rodapé) aplicada ao montar o e-mail; `POST /settings/email/preview`. Original: Assunto, quais seções entram e se agrupa
      os alertas da rodada; pré-visualização no painel.
- [x] **2.3 Relatório semanal por e-mail** (2026-11-24, sem commit): chave `weeklyReport` + estado; envio no dia/hora, uma vez por semana; `GET /reports/weekly/preview`, `POST /reports/weekly/send`. Original: O que piorou, o que melhorou, alertas da
      semana, achados abertos e deploys.
- [x] **2.4 Achados como fila de trabalho.** Status, responsável e prazo editáveis no
      painel (aba Segurança). (2026-11-24: `PATCH /findings/{id}`, edição inline, filtros e vencidos.)

### Etapa 3 — banco (M a G)

- [ ] **3.1 Crescimento e manutenção.** Tamanho por tabela e índice ao longo do tempo,
      tuplas mortas (bloat), último vacuum e autovacuum e tabelas nunca analisadas.
      Coletor de hora em hora.
- [ ] **3.2 Priorizar os índices de FK.** Cruzar as 428 FKs sem índice com as consultas
      lentas e as mais frequentes, e sugerir só as que pesam.
- [ ] **3.3 EXPLAIN automático das consultas lentas.** Para cada fingerprint lento,
      guardar o plano (sem ANALYZE em produção) e uma recomendação de índice ou reescrita.
- [ ] **3.4 `pg_stat_statements`.** Precisa de `shared_preload_libraries` e de
      reiniciar o Postgres de produção, com janela combinada. Dá visão do banco inteiro
      (workers, n8n, scripts).
- [ ] **3.5 Ganho medido de cada otimização.** Comparar o mesmo fingerprint antes e
      depois de um deploy.
- [ ] **3.6 Backup verificado.** Restaurar o backup diário num container descartável e
      conferir contagens. Alertar se falhar.

### Etapa 4 — testes automáticos e agendamento (G a GG)

- [ ] **4.1 Agendamento pelo painel.** Por exemplo: saúde a cada 1 h, segurança toda
      noite e API e páginas uma vez por dia. Os horários ficam em `obs.settings`.
- [ ] **4.2 Runner no servidor.** Container `obs-runner`, com Node e Chrome headless,
      que consome `obs.run_requests`. Assim o botão "rodar agora" funciona sem o Mac.
- [ ] **4.3 Execução acompanhada em tempo real** no painel, com progresso por fase.
- [ ] **4.4 Banco de teste isolado.** Postgres descartável com seed, para os testes
      de escrita (526 rotas) e de carga.
- [ ] **4.5 Bloquear deploy em regressão.** Rodar o runner no CI ou no preflight do
      deploy e barrar em caso de regressão de latência ou de autorização.

### Etapa 5 — back (M a G)

- [ ] **5.1 pprof sob demanda.** Perfil de CPU e memória de 30 s pelo painel, só
      `platform_admin`, com top de funções.
- [ ] **5.2 Custo de IA.** Tokens e valor por conta, por recurso e por modelo.
- [ ] **5.3 Vazamento de goroutine.** Tendência e alerta de crescimento contínuo.
- [ ] **5.4 Carga real** nas rotas críticas, com limite de aceitação (precisa da 4.4).
- [ ] **5.5 Goroutines sem desligamento gracioso:** calendar, storage, cardápio,
      realtime, bi e site.

### Etapa 6 — front (M a G)

- [ ] **6.1 Medir no build de produção.** Rodar a fase de páginas contra produção (só
      leitura) ou contra um build local; confirmar `/tasks`.
- [ ] **6.2 Uso por funcionalidade.** Eventos de uso (tela, ação, conta) e ranking do
      que é usado e do que não é.
- [ ] **6.3 Jornadas automáticas.** Login, criar task, emitir nota e cardápio, com
      passo a passo e verificação.
- [ ] **6.4 Erros de JS com stack legível.** Source maps privados no deploy.
- [ ] **6.5 Tamanho do bundle por página**, com alerta quando crescer.
- [ ] **6.6 Configuração de layout do painel.** Quais cards e seções aparecem na
      aba Saúde.

### Etapa 7 — infra (M a G)

- [ ] **7.1 CPU, memória e load da VPS inteira**, não só dos containers.
- [ ] **7.2 Tendência de disco.** Estimar "enche em X dias" e alertar.
- [ ] **7.3 Logs centralizados e pesquisáveis** pelo painel (por correlation ID).
- [ ] **7.4 Alerta de deploy que piorou.** Comparar p95 e erros 30 min antes e
      depois do deploy.
- [ ] **7.5 Rollback com um clique** (pelo painel ou por comando).
- [ ] **7.6 DNS:** nameservers ainda na HostGator, com prazo até 2026-12-19.

### Etapa 8 — segurança (G a GG)

- [ ] **8.1 Cookie de sessão HttpOnly com CSRF** (M-08). É a maior correção pendente.
- [ ] **8.2 Detecção de ataque em produção:** muitos 401/403 por IP, varredura de
      rotas e login de origem incomum, com alerta.
- [ ] **8.3 Trilha de auditoria:** quem mudou permissão, apagou ou exportou;
      consultável no painel.
- [ ] **8.4 Anexos de feedback autenticados** (A-02 parcial).
- [ ] **8.5 500→400 nos outros 12 endpoints** listados na fase 7 (stores, planning,
      operationgoals, users, reports, entre outros).
- [ ] **8.6 Automatizar mais ataques do catálogo:** CSRF, IDOR em escrita (com a 4.4),
      SSRF, upload malicioso, webhooks sem assinatura e prompt injection.
- [ ] **8.7 Rotação e validade de segredos.** Inventário das chaves (APIs, tokens de
      integração, senha de app do Gmail) com data de rotação.
- [ ] **8.8 Bloqueio de login compartilhado e persistente.** Hoje fica em memória e
      zera quando a API reinicia.
- [ ] **8.9 WhatsApp como segundo canal de alerta.**
