# AGENT.md — Customer Data

## Papel

`customerdata` é o domínio determinístico e headless de subject, relationship,
identidade multiorigem, consentimento, interação offline, matching/merge e
segmentação. Não executa LLM, não envia mensagem e não importa packages de
Omnichannel, ERP, Site ou Customer Intelligence.

## Autoridade e fronteiras

- PostgreSQL (`customer_data.*`) é a fonte única deste módulo.
- Toda operação repete `account_id + client_account_id` no service e repository.
- Em account standalone, `client_account_id = account_id`.
- Em account agência, o client precisa ser explícito, ativo, não-agência,
  acessível ao ator e da mesma `core.organization`.
- Resource fora do scope retorna `ErrNotFound`; não revelar existência cross-client.
- Dados-fonte ficam no módulo dono. Customer Data persiste apenas referência,
  fingerprint, projeção determinística ou payload ID-only.
- Identidade crua só entra no service por um `IdentityProtector`; nunca retorna,
  entra em log ou é armazenada sem cifra + HMAC.
- Consentimento é append-only. Merge é transacional, idempotente e reversível.
- `ResolveRelationship` pode atualizar `display_name` com o nome informado pelo provider somente
  enquanto `classification_source='rule'`. Qualquer edição administrativa muda a classificação
  para `manual` e nunca pode ser sobrescrita por novo inbound. A atualização automática gera
  audit metadata-only.
- Segmentos usam exclusivamente `segment.filter.v1`, catálogo fechado e SQL
  compilado pelo servidor com parâmetros.
- Membership não significa consentimento e não autoriza envio.

## Capabilities e writer

Ausência de linha em `customer_data.capability_states` equivale a `off`.
Ausência de linha em `customer_data.writer_states` equivale a `legacy`.
Mutação autoritativa só ocorre com capability `on` e writer da entidade em `new`.
`shadow` permite apenas validação/preview explicitamente implementados.

## Camadas

```text
HTTP -> Service/policy -> Repository -> customer_data.*
adapter no composition root -> fachada pública do Service
```

Repositories deste package não consultam schemas de outros módulos. Validação de
account/client pode consultar apenas `core.*`.

## Integração pública

O composition root pode obter `Module.Service()` e adaptar fontes por estas
fachadas owner-scoped:

- `ResolveSubject`
- `ResolveRelationship` (alias explícito para adapters de source link/identidade)
- `IngestOfflineInteraction`
- `GetDeterministicProfile`
- `GetSourceEvidence` (source links + interações offline sob capability)
- `GetSegmentContext`

Os requests sempre exigem `AccountID` e `ClientAccountID`. O adapter concreto
continua fora deste package.

## Segurança de API

- rotas: `/v1/customer-data/*`;
- `RequireAuthWithAccount` + module guard quando disponível;
- permissão fina no backend em toda rota;
- alteração de capability/writer state exige
  `customer_data.capabilities.manage`, revisão esperada, idempotência e motivo;
- decoder JSON rejeita campos desconhecidos e corpo excedente;
- IDs de account nunca vêm do body;
- mutações usam `expectedRevision` e/ou `idempotencyKey`;
- erro/log/audit não carregam PII, conteúdo de nota ou valor de predicate.

## Testes mínimos

```text
go test ./internal/modules/customerdata/...
```

Cobrir scope negativo, permission fail-closed, capability/writer off,
idempotência, consentimento append-only, merge/undo e AST adversarial.
