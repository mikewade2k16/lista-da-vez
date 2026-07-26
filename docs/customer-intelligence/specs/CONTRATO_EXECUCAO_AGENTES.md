# Contrato comum de execução — Customer Intelligence

- **Status:** DRAFT
- **Aplica-se a:** todos os pacotes CI-00 a CI-10

## 1. Autoridade e precedência

Em conflito:

1. instrução explícita do owner no turno atual;
2. `AGENT.md` raiz e AGENT local;
3. skills `principios-engenharia` e `omnichannel-hibrido`;
4. `docs/customer-intelligence/GOVERNANCA.md`;
5. este contrato;
6. matriz/catálogo;
7. spec da fase;
8. pacote atômico.

O executor registra a divergência e para antes de escrever na área conflitante.

## 2. Fronteiras obrigatórias

| Owner | Autoridade |
|---|---|
| Omnichannel | canal, participante local, conversa, `ai_generation`, dispatch/lease, FSM, handoff, mensagem, outbox e envio |
| Customer Data | subject, relacionamento, identidade canônica, notas, consentimentos e merge |
| Customer Intelligence | evidência derivada, facts, summaries, prompts, agentes, contexto, tools e recomendações |
| módulos-fonte | dado bruto ERP, Calendário, Site, BI e demais domínios |
| n8n | orquestração stateless e chamada a modelo/tools autorizadas |

Regras:

- IA produz proposta; Omnichannel valida, persiste e envia.
- n8n nunca envia ao canal nem grava PostgreSQL.
- sem SQL cross-module;
- sem segundo writer;
- sem chamada cross-module síncrona dentro da transação do webhook;
- aprendizado aceito entra em outbox na mesma transação de `state + ai_generation`;
- falha de Customer Intelligence nunca impede chat humano.

## 3. Customização e Prompt Registry

### 3.1 Regra de produto

Todo comportamento seguro e materialmente útil de customizar deve ser configurável pelo painel e
persistido no PostgreSQL. Não hardcode prompt, persona, tom, processo, threshold, modelo, tool,
fonte ou feature de negócio quando a spec os declarar administráveis.

### 3.2 Matriz de autoridade

| Assunto | Prompt | Policy estruturada | Invariante Go/PostgreSQL |
|---|---:|---:|---:|
| tom, persona, abordagem | sim | opcional | valida limites |
| objetivo e raciocínio do processo | sim | complementa | valida output |
| modelo, tokens, temperatura, timeout | não | sim | bounds |
| tools/fontes permitidas | apenas orienta uso | sim | allowlist |
| campos e schema de saída | instrui | sim | valida/rejeita |
| fila, FSM, handoff e fechamento | recomenda | sim | decide/aplica |
| permissão, tenant e consentimento | não | sim | obrigatório |
| envio ao canal | produz texto | não | mensagem + outbox |

### 3.3 Um prompt por processo

- todo prompt possui `process_key`;
- prompt publicado é imutável;
- edição cria draft;
- variável é tipada e allowlisted;
- publicação exige validação, casos de teste e diff;
- execução registra definição, versão, binding, camadas e rollout;
- fallback é versão publicada anterior ou comportamento humano seguro, nunca draft parcial;
- um mega-prompt compartilhado implicitamente por processos é rejeitado.
- composição entre processos usa pipeline estruturado/versionado; cada etapa mantém prompt,
  schema, run e `ProcessResult` próprios.

### 3.4 Camadas

```text
platform_guardrail
  + agency_policy
  + client_policy
  + process_prompt
  + agent_override permitido
  + runtime context
```

Compilação é determinística. Guardrail de plataforma não pode ser removido por override de tenant.
O painel mostra origem/herança e diff efetivo.

## 4. Preflight de qualquer pacote de implementação

Registrar no handoff:

```text
git status --short
git diff -- <allowlist>
última migration no disco
AGENTs lidos
baseline de testes do pacote
decisões READY atendidas
```

Mudança do usuário na allowlist é preservada. Se for impossível trabalhar ao redor, parar.

## 5. Unidade atômica e allowlist

- uma tarefa = um resultado verificável;
- só escrever na allowlist do catálogo e da spec;
- migration, backfill, API, UI, workflow, cutover e remoção são pacotes distintos;
- diff previsto acima de 8–12 arquivos ou mais de duas camadas deve ser dividido;
- integração e revisão pertencem ao orquestrador/revisor;
- remoção nunca acompanha criação.

## 6. Banco e migrations

- migrations append-only; nunca editar uma aplicada;
- número é reservado no início do pacote DB após medir o disco;
- PostgreSQL 16, SQL schema-qualified e parametrizado;
- tabela tenant-scoped usa `account_id` físico canônico, FKs compostas quando cabível e índices de
  hot path;
- `owner_account_id` não duplica `account_id` sem decisão explícita;
- enum persistido possui `CHECK`;
- DDL e backfill pesado são separados;
- backfill possui watermark, checksum, relatório de órfãos/ambiguidades e reexecução segura;
- migração legada usa lineage/source hash/transform version próprios e não sobrecarrega status
  funcional da entidade-alvo;
- import cria somente drafts; path sem mapping ou split sem aceite bloqueia validate/publish;
- writer state por cliente/entidade impede dual-write;
- drop exige consumidores/FKs zero, retenção, rollback e pacote REMOVE aprovado;
- atualizar `back/database/ERD.md` e AGENTs na mesma entrega.

Teste mínimo:

1. banco vazio;
2. upgrade da versão anterior;
3. rechecagem idempotente quando suportada;
4. constraints de tenant/status;
5. índice no hot path;
6. `migrate status`.

## 7. Backend Go

- `handler -> service -> repository`;
- dependências por construtor e interfaces pequenas do consumidor;
- handler valida borda; service escopo/regra; repository repete filtro;
- recurso fora do escopo retorna 404;
- eventos críticos usam outbox durável;
- realtime só depois do commit;
- workers possuem idempotência, retry classificado e dead-letter;
- resultado IA é input não confiável;
- prompt version e binding resolvidos no servidor;
- nenhum fallback em memória parece persistência;
- sem segredo, prompt bruto ou PII desnecessária em logs.

## 8. Frontend

- página administrativa usa `AdminPageHeader`;
- `component -> composable/store -> API Go`;
- front nunca chama provider ou n8n;
- sem `any`, `console.log`, token hardcoded ou fonte local autoritativa;
- draft reidrata da API, exceto enquanto dirty;
- prompt editor mostra versão, herança, diff, estado, teste e rollout;
- chave digitada nunca retorna/reidrata/persiste no front;
- gate de workspace, módulo e permissão; URL direta testada;
- loading, empty, error, stale, retry, mobile, tema e acessibilidade;
- ação publicar/rollback exige confirmação e resposta autoritativa.

## 9. n8n e modelos

- somente workflows do owner explicitamente autorizado;
- sem node Evolution/Meta/Instagram sender;
- sem credencial, PII fixa, pinData, staticData ou execution persistence;
- entrada e saída schema-versioned;
- prompt/config chegam do Go/PostgreSQL;
- workflow não escolhe prompt publicado, tenant, tool ou fonte;
- timeout/schema inválido baixa confiança conforme policy fazem fail-open;
- import/ativação são deploy e exigem autorização separada.

## 10. Fontes e tools

- registry explícito em código;
- source/tool IDs não são texto livre;
- credencial é referência segura;
- SSRF, timeout, paginação, limite de resposta e rate limit;
- fonte declara escopo: subject, business context, aggregate ou action;
- write tool chama o service owner e deixa auditoria;
- dado externo é não confiável contra prompt injection;
- desabilitar fonte separa ingestão, uso histórico, invalidação e retenção.

## 11. Segurança e privacidade

- rotas autenticadas derivam escopo do Principal;
- webhook resolve conta server-side e autentica provider;
- gateway interno usa token/claims, timestamp e proteção de replay;
- operador do cliente A não consulta relação B;
- cross-client individual começa desabilitado;
- PII minimizada, criptografada/fingerprinted e com retenção;
- prompt/context snapshot possui TTL, proteção e finalidade;
- segmento usa versão imutável e AST/campos/operadores allowlisted; SQL/expressão livre é proibido;
- preview/materialização/export respeitam tenant, finalidade, consentimento e permissão próprios;
- exclusão invalida derivados e impede reingestão;
- portfólio aplica coorte mínima e supressão.

## 12. Validação proporcional

| Alteração | Prova mínima |
|---|---|
| Go | gofmt + testes dos packages + lint focado |
| SQL | lint + migrate/status descartável + constraints/índice |
| Vue/TS | lint/typecheck/teste focado + browser |
| prompt/runtime | fixtures, schema, eval, custo, versão, fallback e prompt injection |
| adapter | sucesso, timeout, paginação, retry, idempotência e escopo |
| n8n | parse, owner/ID, sem segredo/pin/sender, contrato |
| integração | feliz, duplicata, atraso, erro, humano concorrente e cross-tenant |

Falha preexistente fora do escopo é registrada, não “corrigida por oportunidade”.

## 13. Stop conditions

Parar sem ampliar escopo quando:

- decisão material continua DRAFT;
- target possui mudança do usuário impossível de preservar;
- migration já existe semanticamente;
- owner de workflow/fonte é outro;
- pacote requer sender fora do Omnichannel;
- prompt precisa furar guardrail/schema/allowlist;
- backfill encontra ambiguidade não prevista;
- rollback depende de memória legada obsoleta;
- teste revela vazamento cross-tenant.

## 14. Handoff obrigatório

1. resultado objetivo;
2. arquivos alterados e motivo;
3. migration/backfill e compatibilidade;
4. contratos/API/eventos/prompts alterados;
5. comandos e resultados;
6. critérios provados e não provados;
7. rollout/rollback;
8. riscos/bloqueios;
9. confirmação de nenhum recurso fora do ownership;
10. `git diff --check` e `git status --short`.
