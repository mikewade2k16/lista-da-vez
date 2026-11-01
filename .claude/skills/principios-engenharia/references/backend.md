# Backend

Leia antes de criar ou alterar módulo, handler, service ou repository.

## Padrão de módulo
Cada módulo segue uma estrutura consistente: tipos, persistência, regras de negócio, handlers HTTP e documentação do módulo num arquivo separado. Consistência entre módulos facilita onboarding e troca de agente. Módulos plugáveis se registram via um registry central quando a feature flag correspondente está ligada.

## Separação de camadas (rígida)
`handler → service → repository`. Sem pular camada. O handler HTTP não acessa o banco diretamente; o repository não conhece HTTP. Cada função sabe só o que precisa saber. Mesmo dentro de um arquivo, funções separadas com nome claro — não um bloco de 200 linhas inline.

## Injeção de dependência
Dependências entram via construtor — sem globais, sem singletons implícitos. Interfaces pequenas e focadas; uma interface de 10 métodos é sinal de problema.

## IDs como string
Usar `string` para IDs internamente; não importar pacote de uuid externo no domínio. Casts e geração ficam centralizados num pacote de ids. Reduz dependência externa e facilita refatorar o esquema de IDs.

## Scan de campos NULL
Para colunas nullable, declarar ponteiro (`*string`) ou tipo nullable no Scan; nunca o tipo puro. Evita panic em scan de NULL.

## Tratamento de erro
Erros tratados no ponto onde ocorrem; nunca ignorados silenciosamente (exceto onde é explicitamente justificado). Cast explícito na borda do sistema.

## Permissões vivem no banco (RBAC dinâmico)
Não hardcoded nomes de permissão em `if` no código. Permissões vivem em tabelas (`permissions` + `role_permissions`); o service consulta via registry/repository. Permite customizar papel sem deploy. Um 403 de papel sem permissão é **comportamento correto**, não bug.

## Validação de input na borda
Validar input nos handlers (a borda do sistema). Nunca assumir que dados internos são limpos. Para regras de escopo de tenant/store/account, ver `seguranca-multitenant.md`.

## Logs estruturados
Logs estruturados com campos explícitos: operação, `account_id`, `user_id`, `error`. Nível INFO para fluxo normal, WARN para degradação recuperável, ERROR para falha que precisa de atenção. Nunca logar senha, token, hash de senha ou PII além do necessário — campos explícitos, não a struct inteira interpolada.
