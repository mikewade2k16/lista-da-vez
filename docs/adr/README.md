# ADRs

Esta pasta guarda Architecture Decision Records do projeto.

Cada ADR registra uma decisao relevante de arquitetura, produto ou operacao, com contexto, alternativas consideradas, consequencias e referencias. O objetivo e preservar o por que das escolhas, nao apenas o estado final do codigo.

Regras de uso:

- criar um arquivo por decisao relevante;
- usar numeracao sequencial com quatro digitos (`0001`, `0002`, ...);
- manter o status explicito (`Proposto`, `Aceito`, `Substituido`, `Obsoleto`);
- atualizar um ADR existente apenas para corrigir contexto ou refletir mudanca de status;
- quando a decisao mudar, criar um novo ADR apontando para o anterior.

Convencao de nomes:

- `docs/adr/0001-rename-omni.md`
- `docs/adr/0002-<tema>.md`

Template base:

Use [template.md](./template.md) como ponto de partida para novos ADRs.
