# Contrato comum de execução para agentes

## 1. Autoridade e precedência

Em conflito, vale esta ordem:

1. instrução explícita do dono no turno atual;
2. `AGENT.md` da raiz e `AGENT.md` local do arquivo;
3. skills `principios-engenharia` e `omnichannel-hibrido`;
4. esta especificação comum;
5. spec da fase;
6. pacote atômico recebido.

O executor não “resolve” conflito por preferência. Ele registra caminho, linhas, decisões em
choque e para antes de escrever naquela área.

## 2. Invariantes arquiteturais

| Assunto | Fonte autoritativa | Regra não negociável |
|---|---|---|
| dados operacionais | Go + PostgreSQL | n8n não é banco e não mantém estado definitivo |
| canais/webhooks | adapters Go | n8n nunca recebe webhook público do canal como fonte final |
| dedupe/idempotência | PostgreSQL | nenhuma dedupe somente em memória/Redis/workflow |
| ciclo da conversa | `messaging.conversations.state` | uma máquina de estados; não criar segunda verdade |
| roteamento/filas | Go + PostgreSQL | LLM recomenda; policy/service valida e aplica |
| envio | `messaging.outbox` + adapter Go | n8n nunca chama Evolution, Meta ou Instagram para enviar |
| IA | n8n orquestra; Go governa | modelo/chave/prompt/config vêm do painel e banco, não hardcoded |
| multi-tenant | `account_id` do Principal | nunca aceitar `account_id` do body/query como autoridade |
| segredos | ciphertext/secretbox | nunca JSON exportável, workflow, log, fixture ou resposta HTTP |
| auditoria/custo | PostgreSQL | cada decisão/chamada relevante deixa registro explicável |

## 3. Ownership n8n exato

O módulo omnichannel pode editar apenas:

| Workflow | ID canônico | Owner |
|---|---|---|
| `automation/export/workflow-omnichannel-brain.json` | `omnibrain0000001` | Omnichannel |
| `automation/export/workflow-instagram-first-contact.json` | `instafirst000001` | Omnichannel |

São leitura somente e **não são legado do omnichannel**:

| Recurso | ID/owner | Proibição |
|---|---|---|
| `workflow-whatsapp.json` | `lzhb5JjN5kdcVuRR` / Automação | não editar, importar, exportar, ativar, desativar ou apagar |
| WAHA no Go/compose/VPS | Automação | não remover, renomear ou reutilizar no omnichannel |
| `workflow-calendar-chat.json` | `calendarchat0001` / Calendário | não sincronizar nem corrigir drift |
| `workflow-calendar-omni.json` | `calendaromni0001` / Calendário | não sincronizar nem corrigir drift |
| `workflow-calendar-transcribe.json` | `calendartrans001` / Calendário | não sincronizar nem corrigir drift |
| `workflow-omni-chat.json` | `omnichatmvp00001` / Operação | não alterar sem pacote do owner |

Scripts n8n compartilhados só podem ser alterados por pacote explícito de E0 e devem preservar
IDs, estados e conteúdo de todos os workflows não pertencentes ao omnichannel. Inventário/check
global limita-se a metadados, IDs e hashes dos alvos não selecionados; não exporta nem materializa
seu conteúdo. Um comando global que aponta drift em outro módulo **não autoriza** sincronizá-lo.
Toda escrita recebe owner e seletor exatos, e o registro deve provar que o owner solicitado é o
owner da entrada antes de qualquer Docker, importação ou escrita.

`credentials*.json`, `.mcp.json`, `.bak-*` e outros artefatos não canônicos ficam fora de qualquer
leitura glob do Omnichannel. Backups WAHA/WhatsApp pertencem à Automação e são intocáveis.

## 4. Preflight obrigatório de todo pacote

Antes da primeira alteração, o executor registra no handoff:

```text
git status --short
git diff -- <allowlist do pacote>
última migration no disco
AGENT.md lidos
testes de baseline do pacote
```

Se houver modificação prévia do usuário na allowlist, o executor deve preservá-la e trabalhar
ao redor. Se o pacote exigir sobrescrevê-la, para e reporta. Mudança fora da allowlist é
proibida, ainda que pareça “necessária para limpar”.

## 5. Banco e migrations

### 5.1 Regras de arquivo

- migrations são append-only; nunca editar `0200`–`0212` ou outra já aplicada;
- o número é reservado pelo orquestrador imediatamente antes do pacote; o executor confirma o
  maior prefixo no disco e não escolhe outro por conta própria;
- SQL plano, idempotente quando possível, schema-qualificado e sem `-- +goose Down`;
- PostgreSQL 16; PK principal em UUID; datas em `timestamptz`;
- toda entidade de negócio leva `account_id`; `store_id` quando a regra for por loja;
- FKs e índices devem refletir o hot path real; índice não é decoração;
- `CHECK` fecha enums persistidos; mudanças futuras são migration nova;
- backfill deve ser repetível, limitado por conta e separado de DDL quando puder ser pesado;
- nenhuma mídia/binário/base64 grande em PostgreSQL;
- nenhuma exclusão destrutiva sem retenção/auditoria/rollback definidos.

### 5.2 Migration proposta não é migration autorizada

As specs descrevem tabelas e nomes candidatos. Na execução, o pacote `DB` deve primeiro provar
que a estrutura não existe. Reutilizar tabela/coluna existente é obrigatório quando o contrato
semântico for o mesmo. Criar sinônimo (`contacts` e `customers`, por exemplo) é rejeição.

### 5.3 Teste mínimo de migration

1. banco vazio: `migrate up` completa;
2. banco na versão anterior: migration completa;
3. segunda execução não destrói nem duplica dado quando o runner permitir rechecagem;
4. constraints rejeitam tenant/status inválido;
5. query principal usa índice esperado em volume representativo;
6. `migrate status` inclui o novo arquivo.

## 6. Backend Go

### 6.1 Camadas

```text
HTTP handler -> service/policy -> store/interface -> PostgreSQL
                              -> outbox -> worker -> adapter de canal
                              -> n8n client -> resposta estruturada -> policy/service
```

- handler: autenticação, parse limitado, validação superficial, mapping HTTP;
- service: regra de negócio, autorização, transição e transação;
- store: SQL parametrizado e sempre filtrado por `account_id`;
- adapter: normalização específica do provider;
- worker: side effect repetível, retry classificado e idempotência;
- publisher/realtime: só depois de commit.

SQL no handler, regra de tenant no front, goroutine solta após webhook e chamada direta a canal
fora do adapter/outbox são falhas de arquitetura.

### 6.2 Contrato HTTP

- rotas autenticadas em `/v1/omnichannel/*` usam Principal e gate do módulo;
- webhooks públicos ficam em `/v1/webhooks/omnichannel/{provider}/{accountSlug}` e validam
  assinatura/token conforme provider;
- recurso de outra conta responde `404`, não `403`;
- request body possui limite antes de parse/multipart;
- erros têm `code` estável, mensagem segura e sem payload/segredo;
- listas usam paginação por cursor estável (`created_at,id` ou equivalente), não offset em tabela
  quente;
- idempotency key obrigatória em mutações externas/repetíveis.

### 6.3 Concorrência e transações

- persistir dedupe + mutação de domínio na mesma transação;
- publicar n8n/outbox/realtime somente após commit;
- usar `FOR UPDATE`/compare-and-swap onde IA e humano disputam a conversa;
- resultado atrasado da IA deve ser descartado se o estado/versão da conversa mudou;
- jobs usam `platform/jobs`/outbox; nenhum timer por conversa na memória do processo.

## 7. n8n

O workflow recebe um envelope versionado, consulta contexto/ferramentas por API interna Go e
devolve decisão estruturada. Ele pode agrupar, chamar modelo, transcrever, analisar imagem,
consultar tools permitidas e montar resposta. Ele não pode:

- enviar ao canal;
- escrever diretamente no PostgreSQL;
- conter API key, token, account ID, número ou URL privada fixa;
- decidir permissão, janela da Meta, fila válida ou transição final;
- reutilizar credencial de outro tenant;
- alterar workflow de outro módulo;
- depender de estado manual não exportável.

Cada workflow exportado deve ser determinístico, sem credenciais materializadas, com ID canônico,
nome estável, versão de contrato e erro explícito. Ativação/importação em n8n é ação de deploy e
não é autorizada por um pacote de código.

## 8. Frontend Nuxt/Vue

- front nunca chama provider ou n8n; chama somente API Go;
- manter separação `componentes de apresentação -> composables -> cliente API`;
- sem `any`, `console.log`, import morto, mutação de prop ou segredo no bundle;
- usar tokens/design system existentes, sem hex arbitrário;
- desktop e largura reduzida devem permanecer utilizáveis;
- toda leitura tem estados loading/empty/error/retry;
- toda mutação mostra pending/success/error e reconcilia com resposta autoritativa;
- ação perigosa exige confirmação e explica efeito;
- acessibilidade: label, foco, teclado e `aria-*` quando necessário;
- feature/capability vem da API; não inferir por nome do provider no template;
- arquivo novo acima de ~450–500 linhas deve ser dividido por responsabilidade.

## 9. Segurança e privacidade

- payload de cliente não aparece em log/trace/erro; usar allowlist e masking;
- URL de mídia externa passa por proteção SSRF, limite de tamanho, timeout e MIME validado;
- path de mídia é relativo e validado contra prefixo `{account_id}`;
- chave de API só é retornada mascarada e nunca é reexibida em texto puro;
- chamadas internas Go↔n8n usam assinatura, timestamp, nonce/idempotency e timeout;
- prompt/model output são dados não confiáveis; schema validation acontece no Go;
- tools usam allowlist, timeout, limite, permission check e auditoria;
- comentários/DM automáticos obedecem opt-out, rate limit e política do canal;
- qualquer ação cross-tenant é teste obrigatório, não caso “futuro”.

### 9.1 Permissões canônicas

Reutilizar as permissões já declaradas no módulo; não criar uma key nova por botão:

| Ação | Gate mínimo |
|---|---|
| listar/abrir conversa e mídia | `omnichannel.conversations.view` + visibilidade de fila |
| responder/reagir/usar draft IA | `omnichannel.conversations.reply` |
| assumir, liberar, transferir | `omnichannel.conversations.assign` + membership/policy |
| encerrar/reabrir | `omnichannel.conversations.close` |
| editar contato/nota/tag/consentimento | `omnichannel.contacts.manage` |
| merge/export/anonimização | `contacts.manage` **e** `audit.view`, com confirmação/auditoria |
| configurar provider/segredo/template | `omnichannel.instances.manage` |
| setores, filas, SLA, routing e rollout operacional | `omnichannel.settings.manage` |
| agente, modelo, chave, tools e knowledge binding | `omnichannel.agents.manage` |
| runs, custo, health, DLQ e trilha | `omnichannel.audit.view` |
| moderar comentário | `conversations.reply`; transferir exige também `assign` |

`platform_admin` precisa de branch explícito onde o frontend atual não o representa por `has()`.
Permissão não substitui filtro de `account_id`, membership, state, capability ou janela do canal.

## 10. Validação proporcional

O pacote declara seus comandos exatos. A base mínima é:

```text
Go alterado: gofmt + go test dos packages tocados
SQL alterado: migrate/status em banco descartável ou evidência equivalente
Vue/TS alterado: npm --prefix web run lint + teste/build focado disponível
n8n alterado: parse JSON + validação de ID/owner + diff apenas dos workflows próprios
integração: smoke do caminho feliz + duplicata + erro + cross-tenant
```

Não corrigir falha preexistente fora do escopo. Registrar comando, trecho do erro e por que não é
causado pelo diff.

## 11. Proibições globais

- sem commit, push, PR, deploy, import/ativação de workflow ou alteração na VPS sem pedido;
- sem apagar/renomear legado antes de inventário de consumidores, backup e rollback;
- sem alterar módulo vizinho para “facilitar” o pacote;
- sem feature flag somente no front;
- sem default silencioso que pareça sucesso;
- sem duplicar regra em Go e n8n;
- sem mock em caminho de produção;
- sem TODO genérico como entrega;
- sem afirmar conclusão sem evidência dos critérios de aceite.

## 12. Handoff obrigatório do executor

O retorno final deve conter:

1. resultado objetivo;
2. arquivos alterados e motivo de cada um;
3. migration criada e compatibilidade/backfill;
4. contratos/API/eventos alterados;
5. comandos executados e resultado;
6. critérios de aceite provados e não provados;
7. riscos, decisões e bloqueios reais;
8. confirmação explícita: “nenhum workflow/recurso fora do ownership foi alterado”;
9. `git diff --check` e `git status --short` finais.
