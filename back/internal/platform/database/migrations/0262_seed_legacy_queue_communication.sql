-- Preserva como dado editavel o comunicado que antes estava hardcoded em
-- OperationSidePanel.vue. Cada account com loja recebe sua propria copia.

insert into queue.communications (
    account_id,
    title,
    excerpt,
    body,
    ends_at,
    is_published,
    display_order,
    targets_all_stores,
    created_by,
    updated_by
)
select
    account.account_id,
    'Campanha Progressiva',
    'Vigência: até 26/07',
    'Desconto progressivo válido para joias e relógios, conforme a quantidade de itens comprados dentro do mesmo segmento.

Segmentos válidos:

* Prata com Prata
* Ouro com Ouro
* Relógio com Relógio

Condições de pagamento:

À vista:
1 item = 10% OFF
2 itens = 20% OFF
3 itens ou mais = 30% OFF

Cartão:
1 item = 5% OFF
2 itens = 10% OFF
3 itens ou mais = 20% OFF

❗Importante: os itens não podem ser somados entre segmentos diferentes para aumentar o desconto.

Exemplo: 2 peças em Prata + 1 peça em Ouro não contam como 3 itens para desconto progressivo.',
    timestamptz '2026-07-27 00:00:00-03',
    true,
    0,
    true,
    '00000000-0000-0000-0000-000000000000'::uuid,
    '00000000-0000-0000-0000-000000000000'::uuid
from (
    select distinct tenant_id as account_id
    from queue.stores
) account
where not exists (
    select 1
    from queue.communications existing
    where existing.account_id = account.account_id
      and existing.title = 'Campanha Progressiva'
      and existing.archived_at is null
);
