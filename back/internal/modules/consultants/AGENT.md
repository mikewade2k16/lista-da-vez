# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/consultants`.

## Responsabilidade do modulo

O modulo `consultants` cuida do roster administrativo de consultores por loja.

Hoje ele deve responder por:

- listar consultores ativos de uma loja
- criar consultor ja com acesso autenticado vinculado
- atualizar consultor
- arquivar consultor

Ele nao deve cuidar de:

- status operacional em tempo real
- fila
- pausas e atendimentos
- relatorios agregados

## Contrato atual

- `GET /v1/consultants?storeId=...`
- `POST /v1/consultants`
- `PATCH /v1/consultants/{id}`
- `POST /v1/consultants/{id}/archive`

## Regra nova de identidade

- cada consultor deve ser tambem um usuario real do sistema
- o vinculo e 1:1 por `consultants.user_id`
- o consultor nasce com:
  - email padrao gerado automaticamente por nome + loja
  - senha inicial padrao da politica de rollout
  - `must_change_password = true` para forcar senha pessoal no primeiro acesso
- a conta vinculada usa papel `consultant` no escopo da loja
- ao arquivar o consultor, a conta vinculada tambem deve ser inativada
- a criacao administrativa de consultor deve acontecer por este modulo, nao por `users`, para nao nascer conta `consultant` sem roster
- depois de vinculada, a conta de consultor deve ser considerada propriedade deste modulo
- `users` pode listar e resetar senha do consultor, mas nao deve editar escopo, convite, papel nem ciclo de vida dessa conta
- ao editar o proprio perfil em `auth`, o nome do consultor deve sincronizar de volta no roster

## Regras de escopo

- leitura: qualquer usuario com acesso a loja
- escrita: `owner` e `platform_admin`

## Backlog desta frente

- amadurecer reset operacional com auditoria mais detalhada
- avaliar se vale expor no roster algum indicativo visual de conta sem primeiro login concluido
