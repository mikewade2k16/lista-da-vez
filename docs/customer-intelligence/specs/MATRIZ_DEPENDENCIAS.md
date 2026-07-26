# Matriz de dependências — CI-00 a CI-10

- **Status:** BLUEPRINT READY — implementação local parcial; gates de produção abertos
- **Autoridade:** [../GOVERNANCA.md](../GOVERNANCA.md)

> Esta matriz continua representando dependências e gates, não percentual de conclusão. O estado
> auditado de migrations `0239` a `0255` está em
> [../IMPLEMENTACAO_LOCAL_2026-07-23.md](../IMPLEMENTACAO_LOCAL_2026-07-23.md).

## 0. Posição local no grafo

| Trilha | Estado |
|---|---|
| CI-01→CI-04 | núcleo local de binding, identidade, banco de inteligência, proveniência, reveal e retenção de observações/context snapshots |
| CI-05 | adapters/fonte tipados parciais e sugestões revisáveis; aceitar não ativa fonte |
| CI-06 | Prompt Registry/Studio funcional no lifecycle básico; 2 processos conversacionais + 5 writers headless; 6 writers faltantes |
| CI-07 | proposta tipada → revalidação Omnichannel → `PENDING`/outbox implementada localmente |
| CI-08 | workspace e painéis presentes, sem E2E autenticado/browser concluído |
| CI-09 | resumo/recomendações/sugestões headless presentes; gerador seguro de portfólio ausente |
| CI-10 | canary determinístico existe; SLO, provider real, rollout, rollback, deploy e cutover não provados |

## 1. Caminho crítico

As setas abaixo são gates para implementação da fase, não autorização para antecipar a fase
inteira. Contratos, fixtures ou shell citados como paralelizáveis só avançam dentro de pacote
próprio e não satisfazem a dependência de runtime/DB.

```text
CI-00 contratos/decisões
  -> CI-01 binding canal-cliente
      -> CI-02 identidade/relacionamento
          -> CI-03 Customer Data

CI-02 + contratos estáveis CI-03
  -> CI-04 Intelligence Bank

CI-01 + CI-02 + CI-03 + núcleo CI-04
  -> CI-05 fontes/conectores

CI-04 + fontes mínimas CI-05
  -> CI-06 runtime + Prompt Registry

CI-01 + CI-03 + CI-06
  -> CI-07 integração Omnichannel

CI-03 + CI-04 + CI-06
  -> CI-08 frontend/Prompt Studio
     (CI08-SOURCES-03 exige handoff CI-05)

CI-04 + CI-05 + CI-06
  -> CI-09 recomendações/portfólio

CI-07 + CI-08 + CI-09
  -> CI-10 hardening/cutover
```

## 2. Fases e gates

| Fase | Depende de | Pode paralelizar | Gate de saída |
|---|---|---|---|
| CI-00 | validação deste pacote | nada antes do freeze | ADR, módulos, permissions, process keys e contratos READY |
| CI-01 | CI-00 | fixtures CI-02 | binding/backfill provados sem depender de IA |
| CI-02 | CI-00, contrato CI-01 | desenho CI-01 | matching/relacionamento congelados |
| CI-03 | CI-00, CI-01 em shadow, CI-02 aceita | contratos/fixtures CI-04 | writer único Customer Data |
| CI-04 | CI-00, CI-02, contratos estáveis CI-03 | CI-05 após núcleo pronto | evidência/fato/prompt persistence autoritativa |
| CI-05 | CI-00/01/02/03, núcleo CI-04 | adapters entre si | fontes tipadas, idempotentes e auditáveis |
| CI-06 | CI-04, fontes mínimas CI-05 | CI-08 somente com contratos/fixtures | prompts/processos/runtime headless provados |
| CI-07 | CI-01, CI-03, CI-06 | CI-08 | chat sem IA + IA pelo gateway/outbox |
| CI-08 | CI-03/04/06; sources exige handoff CI-05 | shell/fixtures podem começar após contratos, sem feature live | workspace e Prompt Studio browser-verificados |
| CI-09 | CI-04/05/06 | CI-08 views | recomendações governadas e cross-client restrito |
| CI-10 | CI-07/08/09 | QA/observabilidade | shadow, SLO, rollback e owner approval |

## 3. Ondas de execução

| Onda | Pacotes | Regra |
|---|---|---|
| W0 | CI-00 DOC/ADR/fixtures | nenhum código de produto |
| W1 | CI-01 DB/BACKFILL/BE/QA | primeiro vínculo operacional independente da IA |
| W2 | CI-02 aceite + CI-03 DB/BE/API + CI-04 contratos/fixtures | CI-04 não cria persistence antes do contrato CI-03 |
| W3 | CI-04 DB/BE + CI-05 descriptors/adapters após ports + CI-08 shell/fixtures | sem feature UI ligada a API inexistente |
| W4 | CI-05 fontes mínimas + CI-06 Prompt Registry/runtime + pacotes CI-08 liberados por predecessor | contratos publicados, não hardcoded |
| W5 | CI-07 integração/shadow + CI-09 recomendações + UI restante | envio continua Omnichannel |
| W6 | CI-10 rollout/cutover/deprecação | remoção somente em pacote separado |

## 4. Gates transversais

| Gate | Bloqueia | Prova |
|---|---|---|
| G0 owner validation | todos | specs/decisões aceitas |
| G1 identity scope | CI-03+ | owner/client/subject/relationship sem ambiguidade |
| G2 writer authority | CI-04+ | um writer por entidade/cliente |
| G3 prompt contract | CI-06+ | process keys, layers, schemas, permissions e lifecycle |
| G4 privacy | CI-09/cross-client | observações/context snapshots têm retenção e legal hold técnico; finalidade jurídica, papéis, lifecycle amplo e coorte seguem abertos |
| G5 shadow | produção | canary determinístico local não basta: zero leak, efeito interno único, fallback, E2E e SLO |
| G6 removal | drops | FKs/consumidores/tráfego zero + rollback |

## 5. Dependências do Prompt Registry

| Capacidade | Precisa de |
|---|---|
| editar draft | CI-04 persistence + CI-06 API |
| testar/simular | fixtures CI-02/04 + runtime CI-06 |
| publicar | schema/tool/source registry + permissão publish |
| editar/publicar pipeline | process schemas + Prompt Registry + policy catalog |
| binding por cliente/agente | identidade de cliente CI-01/03 |
| canary | runs/audit CI-04/06 + métricas CI-10 |
| rollback | versão publicada imutável + binding histórico |
| Prompt Studio | APIs CI-06 + gates CI-08 |
| usar no chat | gateway CI-07 |
| usar em recomendações | CI-09 |

## 6. Áreas de conflito

| Área | Fases concorrentes | Owner de integração |
|---|---|---|
| `omnichannel/module.go` e app wiring | CI-01, CI-07 | CI-07 integrador |
| migrations | CI-01, CI-03, CI-04, CI-06 | orquestrador reserva números |
| `messaging.contacts`/CRM facade | CI-01, CI-03, CI-07 | CI-03 writer owner |
| permissions/module registry | CI-00, CI-03, CI-06, CI-08 | CI-00 contrato; integração sequencial |
| front Omnichannel drawer/header | CI-07, CI-08 | CI-08 |
| AI agents/runs/credentials | CI-06, CI-07, CI-10 | CI-06 até cutover |
| Prompt Registry | CI-04, CI-06, CI-08 | CI-06 domínio; CI-08 UI |
| sources | CI-05, CI-06, CI-09 | CI-05 registry |
| deprecação | todas | CI-10 exclusivamente |

Pacotes que tocam a mesma linha/arquivo não executam em paralelo sem partição explícita.

## 7. Dependências externas que não autorizam escrita

- `automation`/WAHA: preservar;
- Calendar: adapter/service público, workflow próprio intocável;
- BI: dataset registrado, mudanças atuais do usuário preservadas;
- Site: service público, schema privado;
- Social Publishing: fora do escopo;
- providers reais/n8n runtime: somente em rollout autorizado.
