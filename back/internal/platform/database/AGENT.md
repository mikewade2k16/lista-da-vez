# AGENT

## Escopo

Estas instrucoes valem para o codigo tecnico de banco em `back/internal/platform/database/`.

## Papel desta pasta

- abrir pool PostgreSQL
- carregar e aplicar migrations
- servir como infraestrutura compartilhada para os modulos

## Regra principal

Esta pasta nao deve conter regra de negocio do produto.

Ela so cuida de:

- conexao
- transacao
- migrations
- utilitarios tecnicos de persistencia

Se a mudanca for de modelagem funcional, ela precisa refletir tambem em:

- `back/database/AGENT.md`
- `back/database/ERD.md`
- modulo de negocio correspondente

## Regra de eficiencia

- esta camada deve favorecer operacoes SQL pequenas, previsiveis e idempotentes
- evitar apoiar implementacoes que regravem secoes inteiras quando a mudanca e de um unico item
- quando o modulo precisar mutacao granular, a infraestrutura deve facilitar `upsert`, `delete` e `touch` focados no recurso alterado
- a infraestrutura tambem deve favorecer contratos HTTP que omitam campos opcionais/default, reduzindo serializacao, I/O e escrita desnecessaria
- quando houver upload de arquivo de usuario, a infraestrutura deve manter no banco apenas o metadado necessario
  - exemplo atual: `users.avatar_path`
  - o binario fica em storage/volume fora do PostgreSQL
- quando houver onboarding por convite, a infraestrutura deve favorecer:
  - token armazenado em hash
  - expiracao previsivel
  - revogacao simples de convites pendentes
  - leitura segura do estado de onboarding sem precisar devolver o token persistido
- quando houver papeis de escopo de loja:
  - a infraestrutura deve facilitar validacao de loja unica por usuario
  - o schema deve permitir diferenciar conta individual (`consultant`) de conta fixa da unidade (`store_terminal`)
- para leituras analiticas, a infraestrutura deve facilitar:
  - indices por `store_id` + tempo
  - filtros previsiveis por colunas mais consultadas
  - uso consciente de `jsonb` quando o dado estruturado fizer parte do filtro ou da agregacao
- para a operacao integrada multi-loja:
  - a infraestrutura deve permitir leitura previsivel do estado corrente por varias lojas sem duplicar logica de montagem
  - a coluna `kind` em `operation_paused_consultants` deve permanecer tratada como campo funcional do estado corrente

## Fonte de verdade humana do banco

Consultar primeiro:

- `back/database/AGENT.md`
- `back/database/ERD.md`

## Comandos uteis

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
```
