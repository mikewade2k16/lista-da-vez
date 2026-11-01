# Banco de dados

Leia antes de criar migration, alterar schema, mover tabela ou tocar em dados existentes.

## Migration idempotente
Toda migration usa `IF NOT EXISTS` / `CREATE OR REPLACE`. Numerar sequencialmente. Nunca renumerar migration já aplicada. Migrations que falham no meio precisam poder ser reaplicadas sem dropar dados.

## Schema-per-módulo + escopo de tenant em toda tabela
Separar schemas por domínio. Toda tabela tenant-scoped tem a coluna de escopo (`account_id`/`tenant_id`) NOT NULL com FK para a tabela de contas. Multi-tenancy com isolamento lógico e queries por schema mais previsíveis.

## SQL parametrizado e schema qualificado
- Sem SQL dinâmico concatenado — sempre parâmetros posicionais (`$1`, `$2`, ...). Zero superfície de SQL injection.
- Todo DDL qualifica o schema explicitamente (`create table queue.consultants`, `references public.users(id)`), não confiando no `search_path` (que pode mudar). Schema explícito garante execução igual em qualquer ambiente. Ver `lint.md`.
- Índices para qualquer coluna usada em `WHERE`, `JOIN` ou `ORDER BY` em hot path.

## Mover tabela para schema: criar view de compat
Ao mover tabela de `public.*` para `schema.*`, criar `CREATE OR REPLACE VIEW public.<tabela> AS SELECT * FROM schema.<tabela>` para não quebrar código legado que ainda aponta para `public.*`.

**Antes de transformar uma tabela-com-dados em view sobre outra tabela:** comparar os DADOS (não só as colunas) das duas. Se houver divergência (ex.: hash de senha diferente para o mesmo id), a view serve o lado errado e pode trancar usuários. Reconciliar os dados primeiro (a fonte VIVA vence) e só então trocar.

## CRÍTICO: nunca sobrescrever senha/dados de usuário sem permissão explícita
NUNCA alterar hash de senha, nem rodar migration/seed/bootstrap/view-swap que sobreescreva senhas, perfis ou outros dados de usuário existentes, sem permissão MUITO explícita do usuário para AQUELE comando específico. Mesmo "autorizado a rodar a sequência" NÃO inclui sobrescrever senha.

- **Por quê:** seed/bootstrap/troca de fonte que reescreve senha tranca o usuário para fora. Caso real: um view-swap passou a servir um hash de senha congelado (de um seed antigo) em vez do hash vivo, e o admin ficou sem login. Restaurado de backup.
- **Aplica quando:** qualquer comando que escreva em dados EXISTENTES (UPDATE/DELETE em users, seeds de senha, bootstrap, view-swap sobre tabela com dados). Operação destrutiva exige: (1) backup antes, (2) checagem de divergência, (3) confirmação explícita do que muda — e senha só com pedido direto.
