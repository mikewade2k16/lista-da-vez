# Customer Intelligence — implementação local

- **Data:** 2026-07-23
- **Estado:** slice operacional parcial implementado no workspace local; verificação E2E, rollout,
  backfill, cutover e deploy não executados
- **Escopo:** Customer Data, Customer Intelligence, integração segura com Omnichannel e fontes
- **Cobertura auditada neste registro:** migrations `0239` a `0255`
- **Governança:** [GOVERNANCA.md](GOVERNANCA.md)
- **Marco de pausa:** [PAUSA_IMPLEMENTACAO_2026-07-23.md](PAUSA_IMPLEMENTACAO_2026-07-23.md)

> Este é um registro de estado, não uma declaração de que CI-00 a CI-10 estão concluídas. “Entregue”
> abaixo significa que existe código/migration local e teste focado; não significa browser
> verificado, provider real validado, rollout aprovado ou produção ativa. Tudo que aparece nas specs
> mas não está confirmado neste relatório continua planejado.

## 1. Resultado

Foi entregue um slice operacional que separa três responsabilidades:

1. **Omnichannel** continua dono da conversa, estado, fila, handoff, mensagem e envio ao canal.
2. **Customer Data** resolve identidade e relacionamento de forma determinística e isolada por
   `account_id + client_account_id`.
3. **Customer Intelligence** coleta observações minimizadas, constrói contexto e, no pipeline de
   conversa implementado, executa triagem/resposta; em jobs headless, gera perfil, recomendações e
   sugestões. Registra proveniência e devolve ou materializa somente resultados tipados.

A IA pode responder automaticamente quando a capability estiver ativa. Ela não chama
WhatsApp/Instagram diretamente:

```text
inbound -> Omnichannel -> Customer Data -> Customer Intelligence/LLM
        <- decisão estruturada <-
Omnichannel revalida lease + geração + FSM + política
-> mensagem PENDING + outbox -> adapter do canal
```

O desenho permite que o chat continue sem IA e que o runtime seja chamado sem painel aberto. A
união dos módulos para atendimento automático já possui caminho local e testes focados, mas ainda
não foi provada por E2E com provider/canal real nem ativada em qualquer ambiente.

## 2. Entregas locais

### 2.1 Banco e isolamento

| Migration | Entrega |
|---|---|
| `0239` | binding explícito canal/recurso → cliente, histórico, snapshots e repair jobs |
| `0240` | subjects, relacionamentos, identidades cifradas/HMAC, fontes, notas, interações e consentimentos |
| `0241` | definições/versões de segmentos, runs de avaliação, materializações e memberships; **não cria exports** |
| `0242` | persistência-base de fontes, runs, observações, claims, fatos, resumos, recomendações, contexto e portfólio |
| `0243` | persistência/configuração de processos, prompts em camadas, modelos, credenciais, agentes, runtime e auditoria |
| `0244` | outbox de efeitos aceitos de Customer Intelligence |
| `0245` | outbox independente para ingestão automática Omnichannel → Customer Data |
| `0246` | runtime tipado, shadow sem efeito, schemas fechados e idempotência por cliente |
| `0247` | correção do escopo de idempotência do Customer Data |
| `0248` | candidate claims extraídas de resultados aceitos, com revisão otimista |
| `0249` | política configurável de falha da IA no Omnichannel |
| `0250` | auditoria metadata-only de cada observação minimizada ingerida |
| `0251` | retenção versionada de observações, binding obrigatório por fonte/run e estado/índice consumido pelo worker Go |
| `0252` | schemas fechados dos onze processos além de triage/reply e pipeline `intelligence.headless`, sem ativação de tenant |
| `0253` | persistência cifrada/idempotente dos cinco resultados headless materializados |
| `0254` | publicação revisionada de retenção e legal hold de observação protegido/auditado no banco |
| `0255` | crypto-shred in-place de context snapshots expirados, com legal hold direto ou derivado de observação |

As tabelas novas repetem o escopo nas chaves e consultas. Um `subject_id` isolado nunca concede
acesso; leitura e escrita de relacionamento exigem owner, cliente e relacionamento autorizados.

### 2.2 Ingestão automática

- inbound suportado cria uma linha ID-only em `messaging.customer_data_outbox` na mesma transação da
  mensagem;
- o worker reidrata a evidência no PostgreSQL e resolve a identidade no Customer Data;
- o nome informado pelo WhatsApp/Instagram cria ou atualiza o `display_name` enquanto o
  relacionamento ainda for governado por regra; qualquer edição manual muda a autoridade para
  `manual` e passa a prevalecer;
- o nome atual também entra no JSON não confiável do turno para a IA saber como se dirigir à pessoa;
- a ingestão independe da IA, do sender e de painel aberto;
- grupos, mensagens `fromMe`, bindings não resolvidos e evidência sem escopo não são promovidos;
- falhas usam retry/dead-letter da lane própria e não bloqueiam o chat.

### 2.3 Runtime e envio — slice implementado

- `conversation.triage` e `conversation.reply` são executados pelo pipeline local
  `conversation.respond`;
- esses dois processos resolvem separadamente prompt/versionamento/binding, agente e modelo;
- os treze `process_key` possuem decoder/validador de saída fechado no Go, com rejeição de campos
  desconhecidos, JSON adicional, enum/UUID/data/confiança inválidos e payload acima de 128 KiB;
- cinco processos adicionais possuem execução e writer headless; os seis restantes possuem
  schema/validador registrado, mas nenhum entrypoint/orquestrador/writer de negócio;
- texto da conversa, perfil e dados de fonte entram somente como JSON não confiável no user prompt;
- prompt nunca pode alterar escopo, consentimento, schema, FSM, allowlist ou autoridade do sender;
- shadow executa e registra comparação, mas não envia, não transfere e não altera estado;
- `canary` usa alocação percentual persistida e bucket determinístico server-side por capability,
  tenant, relacionamento e canal; selecionados executam ativos e não selecionados ficam em shadow;
- falha técnica nunca vira `no_reply` aceito;
- políticas: `legacy_fallback`, `retry_then_handoff` e `immediate_handoff`;
- mensagem inbound, transição FSM, avanço de `ai_generation` e o intento
  `omnichannel.ai.inbound` são gravados no mesmo commit PostgreSQL; falha ao criar o intento
  reverte tudo e permite reentrega do provider;
- não existe mais goroutine/fallback LLM efêmero após o commit do webhook: o worker revalida
  conversa, agente, configuração e schema antes de criar `messaging.ai_dispatches`; indisponibilidade
  terminal degrada explicitamente para roteamento humano;
- somente o Omnichannel aceita a proposta, revalida lease, `ai_generation`, FSM e restrições e
  cria a mensagem `PENDING` + `messaging.outbox`;
- aceitação da decisão, mensagem, outbox do provider, outbox de integração e conclusão do dispatch
  compartilham o caminho transacional Go; `messaging.intelligence_outbox` nunca envia ao provider;
- no Instagram moderado, a resposta vira draft de moderação em vez de envio automático.

### 2.4 Refresh headless de relacionamento

`POST /v1/customer-intelligence/relationships/{relationshipId}/refresh` enfileira um job durável,
ordenado por relacionamento e idempotente por fingerprint opaco. O padrão executa:

1. `profile.summary`;
2. `recommendation.follow_up`;
3. `recommendation.offer`;
4. `recommendation.important_dates`;
5. `source.suggest`.

Cada processo resolve seu próprio prompt/binding/agente/modelo e pode falhar sem apagar o resultado
válido dos demais. Retry é limitado às chaves com falha retryable. O contexto é construído no
servidor; referências de observação/fato precisam existir exatamente nesse `ContextEnvelope` e são
reconfirmadas no PostgreSQL sob o mesmo owner, cliente, subject e relacionamento.

Somente execução ativa e bem-sucedida é materializada:

- resumo publica nova versão e supersede a anterior;
- follow-up, oferta e data importante criam recomendações `proposed` e supersedem propostas
  anteriores do mesmo tipo;
- sugestões de fonte nascem `proposed`, com racional cifrado, confiança, lacunas e validade;
- shadow e membro não selecionado do canary registram run sem produzir efeito.

Não há writer de negócio para `conversation.handoff_summary`, `memory.extract`,
`portfolio.opportunity`, `media.image_analysis`, `media.document_analysis` e `quality.review`.

### 2.5 Claims, fatos e evidências

- extração aceita nasce `candidate + unverified + llm`;
- aceitar uma claim apenas registra curadoria: não cria nem sobrescreve fato;
- referências da outbox não duplicam valor ou PII; o valor é reidratado do runtime cifrado;
- evidências precisam pertencer ao mesmo owner, cliente, subject e relacionamento;
- observações de fontes são novamente filtradas pela allowlist atual antes de serem descriptografadas
  para contexto ou painel;
- observação `restricted` aparece mascarada na auditoria e não entra no contexto LLM genérico;
- payload irrestrito, segredo, ciphertext e chave de idempotência não são expostos.

### 2.6 Observação, reveal e retenção

- a lista do perfil expõe somente observações ativas, autorizadas, não restritas e campos
  allowlisted;
- o detalhe de auditoria mostra sensibilidades protegidas mascaradas por padrão;
- reveal exige `customer_intelligence.audit.view` e reason code seguro/obrigatório; o frontend
  oferece um catálogo fechado de motivos. A auditoria registra ator, motivo, origem, sensibilidade,
  finalidade, flag de reveal e quantidade de campos, sem valores;
- fontes, runs e observações congelam a versão publicada da policy de retenção;
- o scheduler diário enfileira workers por cliente; expiração remove snapshot/ciphertext, preserva
  a linha de proveniência e escreve auditoria metadata-only;
- no mesmo job, context snapshots vencidos sofrem crypto-shred in-place: `payload_ciphertext`,
  `cipher_key_version` e `payload_hash` são removidos, mas ID, escopo, process keys, finalidade,
  datas, contagens e estimativa de tokens mínimos permanecem para referências de runs/resultados;
- policy nova nasce draft e publicação exige `expectedRevision`, reason code e approval reference;
- a página de Fontes lista versões, cria draft e publica separadamente com confirmação, motivo
  catalogado e referência de aprovação; publicar não reponta fontes existentes automaticamente;
- legal hold ativo bloqueia tombstone/crypto-shred inclusive em concorrência com o worker e só pode
  mudar de `active` para `released` com ator/data auditados;
- legal hold direto de context snapshot ou hold de observação relacionado por cliente,
  subject/relacionamento bloqueia a expiração do contexto; business context retido protege
  conservadoramente os snapshots do cliente;
- o worker processa observações e context snapshots em lotes de até 250, no máximo 20 drains por
  tentativa, e reexecução ignora linhas já expiradas;
- ainda não existe API/painel para criar ou liberar legal hold.

### 2.7 Painel — superfícies locais

Workspace `/inteligencia-clientes` com:

- perfis e fatos;
- observações minimizadas e proveniência;
- candidate claims e revisão humana;
- refresh headless de resumo, follow-up, oferta, datas importantes e sugestões de fontes;
- leitura/revisão de resumos e recomendações materializados;
- sugestões de fontes com lacunas, racional, confiança, expiração e aceite/rejeição;
- segmentos;
- catálogo/configuração/saúde de fontes por descriptor fechado, incluindo finalidade, allowlist,
  frescor, retenção e configuração específica do adapter;
- governança de retention policies com histórico de versões, criação de draft e publicação
  explícita; a chave publicada pode ser informada na configuração da fonte, versão obsoleta é
  bloqueada e troca de escopo/permissão cancela leitura ou mutação pendente;
- Prompt Studio por processo com listagem, draft/edição, validação, teste estrutural,
  publicação/binding com agente publicado e rollback;
- runs e auditoria;
- modelos, credenciais write-only e agentes;
- leitura/administração de oportunidades de portfólio já persistidas, sem pipeline gerador;
- configurações/capabilities e writer states.

No Omnichannel foram adicionados binding canal → cliente e política de falha da IA.

Limites importantes do painel atual:

- o “teste” de prompt valida estrutura/template/schema e cria avaliação estrutural, sem chamar o
  modelo nem executar corpus de eval;
- a API não publica descriptor de policy por processo nem rota de edição de pipeline; essas áreas
  permanecem vazias, e mudanças de policy mostradas localmente não podem ser tratadas como
  persistidas;
- tools/knowledge e rollout de prompt não possuem administração funcional completa no Studio;
- o painel governa versões de policy de retenção, mas não cria/libera legal hold;
- a tela de segmentos mostra materializações somente para leitura e informa que exportação está
  indisponível; a ação que chamava o endpoint inexistente foi removida;
- Configurações e Segmentos foram verificadas em sessão autenticada local. A correção tornou
  explícitos os imports de componentes aninhados, preservou o client scope entre instâncias do
  composable e eliminou a área vazia reportada. A varredura visual completa das demais rotas e
  estados continua pendente.

## 3. Fontes modulares

O registry aceita somente `source_key` conhecido e adapter registrado. A configuração do painel
define status, modo, finalidade, allowlist, freshness e configuração fechada.

| Fonte | Comportamento local |
|---|---|
| Omnichannel | inbound automático pelo outbox durável |
| Offline/manual | nota/interação e fato manual com evidência |
| Calendário | facade owner-scoped de contexto de negócio |
| ERP | consulta somente por vínculo determinístico previamente registrado no Customer Data |
| Site | consulta somente por vínculo determinístico previamente registrado no Customer Data |
| BI | somente registry/validação de configuração; retorna `deterministic_subject_link_unavailable` e não faz chamada externa enquanto não existir filtro determinístico por pessoa |

Não existe busca fuzzy por nome, SQL livre, URL livre ou escolha de credencial pela LLM. Fonte sem
contrato seguro retorna indisponível e não tenta ampliar acesso.

Aceitar uma sugestão de fonte é deliberadamente apenas feedback auditado. Não cria nem altera
`source_configs`, não ativa adapter, não inicia sync e não abre formulário de credencial. A
configuração real continua sendo uma ação separada no painel de Fontes, com permissão própria.

## 4. APIs principais adicionadas

```text
/v1/omnichannel/channel-client-bindings/**

/v1/customer-data/**

/v1/customer-intelligence/capabilities/**
/v1/customer-intelligence/sources/**
/v1/customer-intelligence/retention-policies
/v1/customer-intelligence/retention-policy-versions/{id}/publish
/v1/customer-intelligence/relationships/{relationshipId}/profile
/v1/customer-intelligence/relationships/{relationshipId}/facts
/v1/customer-intelligence/relationships/{relationshipId}/observations
/v1/customer-intelligence/observations/{id}
/v1/customer-intelligence/observations/{id}/reveal
/v1/customer-intelligence/relationships/{relationshipId}/claims
/v1/customer-intelligence/claims/{id}/review
/v1/customer-intelligence/relationships/{relationshipId}/refresh
/v1/customer-intelligence/relationships/{relationshipId}/recommendations
/v1/customer-intelligence/recommendations/{id}/review
/v1/customer-intelligence/relationships/{relationshipId}/source-suggestions
/v1/customer-intelligence/source-suggestions/{id}/review
/v1/customer-intelligence/processes
/v1/customer-intelligence/prompts/**
/v1/customer-intelligence/models
/v1/customer-intelligence/credentials/**
/v1/customer-intelligence/agents/**
/v1/customer-intelligence/runtime/**
/v1/customer-intelligence/runs
/v1/customer-intelligence/audit-events
/v1/customer-intelligence/portfolio/opportunities
```

As rotas implementadas passam por autenticação, module gate, RBAC e validação server-side de
client/relationship. O prefixo `/v1/customer-data/**` não inclui export de segmento, importação CSV
ou anexos nesta entrega. Não existe rota pública de resolução de contexto: o `ContextEnvelope`
bruto é construído e consumido apenas internamente.

## 5. Como validar automaticamente

No PowerShell, a partir da raiz:

```powershell
.\scripts\verify-customer-intelligence.ps1
```

Para incluir migrations, use um PostgreSQL vazio e descartável:

```powershell
$env:TEST_DATABASE_URL = 'postgres://.../banco_descartavel?sslmode=disable'
.\scripts\verify-customer-intelligence.ps1 -WithMigrations
```

Para incluir também o typecheck completo e o build do Nuxt:

```powershell
.\scripts\verify-customer-intelligence.ps1 -WithWebBuild
```

O teste de migrations não deve apontar para banco com dados importantes.

Esses comandos cobrem testes unitários/integrados disponíveis, lint, typecheck/build opcionais e
migrations em banco descartável. Eles não substituem E2E autenticado no browser nem teste com WhatsApp,
Instagram, LLM/provider, n8n ou infraestrutura de produção reais.

Evidências desta rodada:

- `go test -count=1 ./...` e `go vet ./...` passaram;
- migrations `0001` a `0255` aplicaram em PostgreSQL 16 vazio e os testes integrados reais de
  Customer Data/Intelligence passaram;
- o teste PostgreSQL do intento de IA provou commit/idempotência e rollback integral;
- a suíte web completa passou em `61/61` arquivos e `295/295` testes; o lint completo terminou com
  zero erro e warnings globais preexistentes, e o recorte de Customer Intelligence ficou sem erro
  ou warning;
- `vue-tsc --noEmit` continua falhando por dívida global anterior, com zero linha relacionada aos
  arquivos de Customer Intelligence/rotas alterados nesta entrega;
- o build Nuxt normal compilou os bundles, mas o empacotamento encontrou um reparse point
  Linux/WSL antigo em `.output`; `Nitro prepare` com saída isolada passou, enquanto o full build
  isolado excedeu doze minutos e ficou inconclusivo. Isso não é aprovação de build.

Essas evidências não substituem os testes autenticados e integrados descritos acima.

## 6. Preparação para teste manual

1. Aplicar as migrations em ambiente descartável.
2. Definir chaves diferentes, fortes e persistentes:
   - `OMNI_SECRETS_KEY`;
   - `CUSTOMER_DATA_ENCRYPTION_KEY`;
   - `CUSTOMER_DATA_HMAC_KEY`.
3. Habilitar os módulos `customer_data` e `customer_intelligence` apenas na conta de teste.
4. No painel de configurações:
   - habilitar capabilities necessárias;
   - conferir writer state;
   - cadastrar modelo;
   - cadastrar credencial;
   - criar/publicar agente;
   - criar, salvar, validar, testar e publicar prompts separados para cada processo que será
     executado;
   - confirmar o agente publicado selecionado e, depois de uma segunda versão, testar rollback;
   - em `canary`, definir uma alocação percentual explícita.
5. Configurar o binding do recurso de canal para o cliente de teste.
6. Configurar a política de falha inicialmente como `retry_then_handoff`.
7. Habilitar uma fonte com allowlist mínima.
8. Não usar credenciais reais de produção; provider, modelo e adapters externos precisam de
   credenciais próprias do ambiente descartável.

Capabilities e módulos permanecem desligados por padrão. Não habilitar produção nesta etapa.

## 7. Cenários manuais obrigatórios

### A. Chat sem IA

1. deixar runtime da IA `off`;
2. enviar inbound pelo canal de teste;
3. confirmar mensagem no inbox e atendimento humano normal;
4. confirmar ingestão Customer Data;
5. confirmar ausência de envio automático.

### B. Shadow

1. publicar prompts/agente e colocar runtime em `shadow`;
2. enviar inbound;
3. confirmar runs com `executionMode=shadow`;
4. confirmar que o comportamento operacional veio do legado/humano;
5. confirmar ausência de efeito da decisão shadow.

### C. Resposta automática

1. colocar runtime em `on`;
2. enviar inbound com conversa elegível;
3. confirmar triagem e resposta em runs;
4. confirmar decisão aceita;
5. confirmar criação da mensagem `PENDING` e outbox pelo Omnichannel;
6. confirmar entrega pelo adapter e ACK.

### D. Takeover concorrente

1. iniciar uma execução de IA;
2. fazer takeover humano antes do retorno;
3. confirmar mudança de `ai_generation`;
4. confirmar descarte do resultado atrasado e ausência de mensagem/outbox.

### E. Falha do provider

1. usar credencial inválida ou provider indisponível;
2. confirmar retry limitado para erro transitório;
3. confirmar handoff seguro ao esgotar tentativas;
4. repetir com `immediate_handoff`;
5. confirmar que nenhum erro técnico foi persistido como decisão `no_reply` aceita.

### F. Claim candidata

1. produzir triagem com claim válida e evidência do mesmo relacionamento;
2. confirmar claim `candidate/unverified/llm`;
3. aceitar com revision e reason;
4. confirmar status `accepted`;
5. confirmar que nenhum fato foi criado ou sobrescrito automaticamente;
6. repetir com revisão desatualizada e esperar conflito.

### G. Fonte e observação

1. configurar ERP/Calendário/Site ou fonte manual com duas chaves na allowlist e vínculo
   determinístico quando exigido;
2. sincronizar;
3. abrir o perfil e conferir apenas essas chaves;
4. remover uma chave da allowlist e recarregar;
5. confirmar que a chave removida deixa de ser exibida e de entrar no contexto;
6. abrir o evento de auditoria e conferir proveniência metadata-only.
7. configurar BI separadamente e confirmar
   `deterministic_subject_link_unavailable`, sem chamada externa nem observação individual.

### H. Reveal auditado

1. abrir na Auditoria uma observação `personal`, `sensitive` ou `restricted`;
2. confirmar campos mascarados por padrão;
3. revelar com um motivo permitido;
4. confirmar que somente campos allowlisted aparecem naquela resposta;
5. conferir `source.observation_accessed` com ator, motivo e contagem, sem valores no metadata;
6. tentar acessar/revelar com outro cliente e esperar `not_found`/`forbidden`.

### I. Refresh headless

1. publicar agente e prompt de cada um dos cinco processos;
2. habilitar profile/runtime no cliente de teste;
3. clicar em **Gerar / atualizar** no perfil;
4. confirmar job idempotente e runs separados;
5. confirmar nova versão do resumo, três recomendações e sugestões de fontes propostas;
6. verificar evidence/fact refs e tentar uma referência fora do contexto, esperando rejeição;
7. repetir em `shadow` e confirmar zero materialização;
8. repetir em `canary` e conferir `canary_selected` ou `canary_not_selected`.

### J. Sugestão de fonte

1. abrir uma sugestão `proposed`;
2. aceitar/rejeitar com motivo permitido;
3. confirmar auditoria e transição única;
4. ao aceitar, confirmar que nenhuma fonte foi criada/habilitada, nenhuma credencial foi pedida e
   nenhum sync foi iniciado.

### K. Retenção e legal hold

1. pela tela de Fontes, criar draft de policy e publicar com revisão esperada, motivo catalogado,
   referência de aprovação e confirmação;
2. vincular uma fonte de teste e ingerir observação com expiração curta permitida;
3. executar o worker e confirmar payload limpo, linha/proveniência preservada e auditoria;
4. criar um context snapshot expirável e confirmar crypto-shred in-place, auditoria e preservação
   dos IDs/metadados mínimos;
5. em teste de repository/migration, criar legal hold direto e outro herdado de observação antes da
   expiração e confirmar que o worker não limpa os payloads;
6. liberar o hold pela transição auditada e repetir a retenção;
7. repetir o job e confirmar idempotência e batches limitados.

O passo de legal hold é técnico porque ainda não existe API/painel autenticado para seu lifecycle.

### L. Isolamento negativo

1. repetir leituras com outro `clientAccountId`;
2. tentar usar observation/claim/relationship de outro cliente;
3. esperar `not_found` ou `forbidden`, nunca dados;
4. confirmar que o repository não faz leitura antes do client scope quando a autorização falha.

### M. Idempotência

1. repetir o mesmo webhook/inbound;
2. confirmar uma mensagem, um evento de ingestão e um efeito operacional;
3. repetir source sync com a mesma chave dentro do cliente e confirmar o mesmo run;
4. usar a mesma chave em outro cliente e confirmar isolamento.

## 8. Rollout e rollback

Rollout recomendado:

```text
off -> shadow -> canary restrito -> on por cliente/canal
```

O seletor canary local usa bucket determinístico e configuração persistida. Isso permite testar
coorte estável, mas não prova rollout seguro: não avançar para `on` antes de E2E autenticado,
métricas/SLO, credenciais de ambiente, teste com provider real controlado e ensaio de rollback.

Stop conditions:

- vazamento cross-client;
- envio duplicado;
- shadow causando efeito;
- resultado atrasado vencendo takeover;
- erro técnico aceito como decisão;
- observação fora da allowlist;
- degradação não controlada do provider.

Rollback operacional usa capabilities/bindings para voltar a `off` ou ao legado. As migrations são
aditivas; remoção de tabelas, backfill destrutivo e exclusão do legado não fazem parte desta entrega.

## 9. Lacunas e itens ainda planejados

O produto completo descrito nas specs não foi entregue nesta rodada:

| Área | Estado real / pendência |
|---|---|
| Processos de IA | 13 contratos fechados; triage/reply e cinco processos headless possuem fluxo funcional; handoff summary, memory extract, portfólio, mídia de imagem/documento e quality review não têm writer/orquestração de negócio |
| Resumo/recomendações | refresh headless materializa resumo, follow-up, oferta, datas e sugestões; não existe scheduler amplo nem ação comercial automática |
| Prompt Studio | draft/edição, validação, teste estrutural, publicação/binding e rollback existem; policy descriptor/persistência por processo, editor de pipeline, simulação com LLM, corpus/evals, diff completo, tools/knowledge e rollout de prompt permanecem incompletos |
| Canary | seletor determinístico local entregue; rollout, métricas/SLO, provider real e rollback não foram provados |
| BI | catálogo/configuração apenas; nenhuma evidência individual é consultada |
| Segmentos | definição/versão e fila de runs existem; worker contínuo de avaliação/materialização não foi entregue |
| Export de segmentos | capability/permissão e tipos preparatórios existem, mas a UI acionável foi retirada; não há tabela, migration, endpoint Go, worker, objeto privado, download intent ou arquivo; `0241` não entrega export |
| Offline | CRUD manual existe; importação CSV, anexos, storage/scan e backfill amplo não |
| Tools/knowledge | persistência-base existe; execução de tools e recuperação de knowledge no runtime não |
| Portfólio | persistência e gates mínimos existem; falta gerador agregado seguro, ranking validado e fluxo de promoção; uso cross-client individual permanece desabilitado |
| Retenção/reveal | observações e context snapshots têm expiração auditada, reveal/crypto-shred e legal hold direto/derivado; falta API/painel de legal hold e cobertura equivalente para summaries, recomendações, runs e demais derivados |
| LGPD | minimização, escopo, HMAC/criptografia e retenção de observações/contexto existem; papéis, base legal, DSAR/export do titular, anonimização/exclusão ampla e política de backups ainda exigem decisão e implementação |
| Legado/n8n | paridade, importação de prompts/workflows, writer cutover e retirada do legado não executados |
| Operação | credenciais/configuração reais de ambiente, E2E autenticado, provider/canal real, observabilidade/SLO, ensaio de rollback, rollout, deploy e cutover pendentes |

Versões antigas de agente também não possuem GET/PATCH dedicado; o painel administra/publica apenas
o draft criado na sessão atual. Esses limites não impedem testes focados do slice local, desde que
capabilities permaneçam desligadas por padrão e o resultado não seja tratado como pronto para
produção.
